package builds

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"fx.prodigy9.co/config"
	"fx.prodigy9.co/data"
	"fx.prodigy9.co/fxlog"
	"fx.prodigy9.co/httpserver/controllers"
	"fx.prodigy9.co/httpserver/httperrors"
	"fx.prodigy9.co/httpserver/render"
	"github.com/go-chi/chi/v5"
	"platform.prodigy9.co/srv/auth"
	"platform.prodigy9.co/srv/github"
	"platform.prodigy9.co/srv/install"
	"platform.prodigy9.co/srv/repos"
)

// listLimit is what the build list shows: enough history to see the last few pushes of
// every active ref without paging machinery that has no consumer yet.
const listLimit = 50

// BuildCtr serves the fragment's slice of the UI API. Wire structs are hand-written per
// handler; there is deliberately no shared api/ contract package (spec §No api/ contract
// layer).
type BuildCtr struct{}

var _ controllers.Interface = BuildCtr{}

func (BuildCtr) Mount(cfg *config.Source, router chi.Router) error {
	router.Get("/api/builds", list)
	router.Get("/api/builds/{id}", get)
	router.Get("/api/builds/{id}/steps", listSteps)
	router.Post("/api/builds", trigger)
	router.Get("/api/repos/{owner}/{repo}/builds", listForRepo)
	return nil
}

// trigger records a webui-triggered build: the same domain fact as the webhook,
// authorized by session instead of HMAC (spec §Triggering a build). The controller
// applies the session's write gate, then resolves ref→sha before recording; the App's
// independent repo lookup ensures the worker can reach the requested source.
func trigger(resp http.ResponseWriter, req *http.Request) {
	user, ok := auth.RequireUser(resp, req)
	if !ok {
		return
	}
	ctx := req.Context()

	create := &Create{Trigger: TriggerWebUI, UserID: user.ID}
	if err := controllers.ReadAction(req, create); err != nil {
		render.Error(resp, req, 400, err)
		return
	}
	if !auth.RequireRepoWrite(resp, req, create.Owner, create.Repo) {
		return
	}
	if _, err := repos.Registered(ctx, create.Owner, create.Repo); errors.Is(err, repos.ErrNotRegistered) {
		render.Error(resp, req, 404, err)
		return
	} else if err != nil {
		render.Error(resp, req, 500, err)
		return
	}

	token, client, err := install.Token(ctx)
	if err != nil {
		render.Error(resp, req, 500, err)
		return
	}
	create.CloneURL, err = client.RepoCloneURL(ctx, token, create.Owner, create.Repo)
	if errors.Is(err, github.ErrRepoUnreachable) {
		render.Error(resp, req, 404, err)
		return
	} else if err != nil {
		render.Error(resp, req, 500, err)
		return
	}
	create.SHA, err = client.ResolveRef(ctx, token, create.Owner, create.Repo,
		strings.TrimPrefix(create.Ref, "refs/"))
	if errors.Is(err, github.ErrRefUnresolvable) {
		render.Error(resp, req, 404, err)
		return
	} else if err != nil {
		render.Error(resp, req, 500, err)
		return
	}
	observed, err := client.RepoManifestAt(ctx, token, create.Owner, create.Repo, create.SHA)
	if errors.Is(err, github.ErrRepoUnreachable) {
		render.Error(resp, req, 404, err)
		return
	} else if errors.Is(err, github.ErrNoManifest) {
		render.Error(resp, req, 409, err)
		return
	} else if err != nil {
		render.Error(resp, req, 500, err)
		return
	}
	manifest, err := repos.ParseManifest(observed.Raw, create.Repo)
	if err != nil {
		render.Error(resp, req, 400, err)
		return
	}
	create.ManifestRaw, create.Manifest = string(observed.Raw), *manifest
	if create.RetryOf != 0 {
		create.Trigger = TriggerRetry
	}

	build := &Build{}
	var result Result
	err = data.Run(ctx, func(scope data.Scope) error {
		if err := create.Execute(scope.Context(), build); err != nil {
			return err
		}
		results, err := ResultsFor(scope.Context(), []*Build{build})
		if err != nil {
			return err
		}
		result = results[build.ID]
		return nil
	})
	if errors.Is(err, ErrInvalidModules) || errors.Is(err, ErrInvalidRetry) {
		render.Error(resp, req, 400, err)
		return
	} else if errors.Is(err, repos.ErrNotRegistered) {
		render.Error(resp, req, 404, err)
		return
	} else if err != nil {
		render.Error(resp, req, 500, err)
		return
	}
	renderCreated(resp, req, respond(build, result))
}

// renderCreated is render.JSON at 201: fx's render fixes status 200 (a gap its own TODO
// notes), so the one created-returning handler writes the status itself. The 201 is
// committed before encoding, so an encode failure cannot become a status — it is logged
// instead of silently swallowed.
func renderCreated(resp http.ResponseWriter, req *http.Request, obj any) {
	resp.Header().Set("Content-Type", "application/json")
	resp.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(resp).Encode(obj); err != nil {
		fxlog.Log("encoding 201 response failed after commit",
			fxlog.String("path", req.URL.Path),
			fxlog.String("error", err.Error()))
	}
}

// listForRepo serves one repo's build feed, newest first; ?limit=N caps the page —
// builds nest under a repo in the UI, and the landing page fans out ?limit=3 per
// visible repo (spec §Operations).
func listForRepo(resp http.ResponseWriter, req *http.Request) {
	owner, repo := chi.URLParam(req, "owner"), chi.URLParam(req, "repo")
	if !auth.RequireRepoRead(resp, req, owner, repo) {
		return
	}
	ctx := req.Context()

	limit := listLimit
	if requested := req.URL.Query().Get("limit"); requested != "" {
		parsed, err := strconv.Atoi(requested)
		if err != nil || parsed < 1 {
			render.Error(resp, req, 400, httperrors.ErrBadRequest)
			return
		}
		limit = min(parsed, listLimit)
	}

	builds := []*Build{}
	err := data.Select(ctx, &builds, `
		SELECT `+buildColumns+` FROM `+buildFrom+`
		WHERE lower(r.owner) = lower($1) AND lower(r.repo) = lower($2)
		ORDER BY b.id DESC LIMIT $3`,
		owner, repo, limit)
	if err != nil {
		render.Error(resp, req, 500, err)
		return
	}

	renderList(resp, req, builds)
}

// buildResponse is the record as stored plus the fold of its events. The fold is computed
// per read — nothing about how a build went is stored on the row.
type buildResponse struct {
	ID         int64     `json:"id"`
	Trigger    Trigger   `json:"trigger"`
	RetryOf    int64     `json:"retry_of"`
	UserID     int64     `json:"user_id"`
	RepoID     int64     `json:"repo_id"`
	ManifestID int64     `json:"manifest_id"`
	Owner      string    `json:"owner"`
	Repo       string    `json:"repo"`
	CloneURL   string    `json:"clone_url"`
	Ref        string    `json:"ref"`
	SHA        string    `json:"sha"`
	CreatedAt  time.Time `json:"created_at"`

	Status     Status           `json:"status"`
	StartedAt  time.Time        `json:"started_at"`
	FinishedAt time.Time        `json:"finished_at"`
	Error      string           `json:"error"`
	Modules    []moduleResponse `json:"modules"`
}

type moduleResponse struct {
	BuildModuleID    int64     `json:"build_module_id"`
	ManifestModuleID int64     `json:"manifest_module_id"`
	Name             string    `json:"name"`
	Status           Status    `json:"status"`
	StartedAt        time.Time `json:"started_at"`
	FinishedAt       time.Time `json:"finished_at"`
	Error            string    `json:"error"`
	Image            string    `json:"image"`
	Hash             string    `json:"hash"`
	ClaimedAt        time.Time `json:"claimed_at"`
	ClaimedBy        string    `json:"claimed_by"`
	EngineHost       string    `json:"engine_host"`
	EngineAssignedAt time.Time `json:"engine_assigned_at"`
	LastEventKind    EventKind `json:"last_event_kind"`
	LastEventAt      time.Time `json:"last_event_at"`
}

func list(resp http.ResponseWriter, req *http.Request) {
	if _, ok := auth.RequireUser(resp, req); !ok {
		return
	}

	repositories, err := auth.ReadableRepos(req)
	if err != nil {
		render.Error(resp, req, 500, err)
		return
	}
	keys := make([]string, len(repositories))
	for i, repository := range repositories {
		keys[i] = strings.ToLower(repository.Owner) + "/" + strings.ToLower(repository.Name)
	}
	if len(keys) == 0 {
		render.JSON(resp, req, []buildResponse{})
		return
	}

	ctx := req.Context()
	builds := []*Build{}
	err = data.Select(ctx, &builds, `
		SELECT `+buildColumns+` FROM `+buildFrom+`
		WHERE lower(r.owner) || '/' || lower(r.repo) = ANY($1)
		ORDER BY b.id DESC LIMIT $2`, keys, listLimit)
	if err != nil {
		render.Error(resp, req, 500, err)
		return
	}

	renderList(resp, req, builds)
}

// renderList folds each listed build's events and renders the page — the shared back
// half of the global and per-repo lists.
func renderList(resp http.ResponseWriter, req *http.Request, builds []*Build) {
	results, err := ResultsFor(req.Context(), builds)
	if err != nil {
		render.Error(resp, req, 500, err)
		return
	}

	out := make([]buildResponse, len(builds))
	for i, build := range builds {
		out[i] = respond(build, results[build.ID])
	}
	render.JSON(resp, req, out)
}

func get(resp http.ResponseWriter, req *http.Request) {
	build, ok := loadBuild(resp, req)
	if !ok {
		return
	}

	results, err := ResultsFor(req.Context(), []*Build{build})
	if err != nil {
		render.Error(resp, req, 500, err)
		return
	}
	render.JSON(resp, req, respond(build, results[build.ID]))
}

type stepResponse struct {
	BuildModuleID int64     `json:"build_module_id"`
	Step          string    `json:"step"`
	StartedAt     time.Time `json:"started_at"`
	FinishedAt    time.Time `json:"finished_at"`
	Error         string    `json:"error"`
	Stdout        string    `json:"stdout"`
	Stderr        string    `json:"stderr"`
}

func listSteps(resp http.ResponseWriter, req *http.Request) {
	build, ok := loadBuild(resp, req)
	if !ok {
		return
	}

	steps, err := ReadSteps(req.Context(), build.ID)
	if err != nil {
		render.Error(resp, req, 500, err)
		return
	}
	out := make([]stepResponse, len(steps))
	for i, step := range steps {
		out[i] = stepResponse{
			BuildModuleID: step.BuildModuleID,
			Step:          step.Step,
			StartedAt:     step.StartedAt,
			FinishedAt:    step.FinishedAt,
			Error:         step.Error,
			Stdout:        step.Stdout,
			Stderr:        step.Stderr,
		}
	}
	render.JSON(resp, req, out)
}

// loadBuild gates, resolves {id}, and reads the row — the shared
// front half of the detail and steps reads. ok=false means a response was written.
func loadBuild(resp http.ResponseWriter, req *http.Request) (*Build, bool) {
	if _, authed := auth.RequireUser(resp, req); !authed {
		return nil, false
	}
	ctx := req.Context()

	id, err := strconv.ParseInt(chi.URLParam(req, "id"), 10, 64)
	if err != nil {
		render.Error(resp, req, 404, httperrors.ErrNotFound)
		return nil, false
	}

	build := &Build{}
	err = data.Get(ctx, build, `SELECT `+buildColumns+` FROM `+buildFrom+` WHERE b.id = $1`, id)
	if data.IsNoRows(err) {
		render.Error(resp, req, 404, httperrors.ErrNotFound)
		return nil, false
	} else if err != nil {
		render.Error(resp, req, 500, err)
		return nil, false
	}
	if !auth.RequireRepoRead(resp, req, build.Owner, build.Repo) {
		return nil, false
	}
	return build, true
}

func respond(build *Build, result Result) buildResponse {
	modules := make([]moduleResponse, len(result.Modules))
	for i, module := range result.Modules {
		modules[i] = moduleResponse{
			BuildModuleID: module.ID, ManifestModuleID: module.ManifestModuleID,
			Name: module.Name, Status: module.Status,
			StartedAt: module.StartedAt, FinishedAt: module.FinishedAt,
			Error: module.Error, Image: module.Image, Hash: module.Hash,
			ClaimedAt: module.ClaimedAt, ClaimedBy: module.ClaimedBy,
			EngineHost: module.EngineHost, EngineAssignedAt: module.EngineAssignedAt,
			LastEventKind: module.LastEventKind, LastEventAt: module.LastEventAt,
		}
	}

	return buildResponse{
		ID:         build.ID,
		Trigger:    build.Trigger,
		RetryOf:    build.RetryOf,
		UserID:     build.UserID,
		RepoID:     build.RepoID,
		ManifestID: build.ManifestID,
		Owner:      build.Owner,
		Repo:       build.Repo,
		CloneURL:   build.CloneURL,
		Ref:        build.Ref,
		SHA:        build.SHA,
		CreatedAt:  build.CreatedAt,

		Status:     result.Status,
		StartedAt:  result.StartedAt,
		FinishedAt: result.FinishedAt,
		Error:      result.Error,
		Modules:    modules,
	}
}
