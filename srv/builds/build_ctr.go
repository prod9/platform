package builds

import (
	"context"
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
// resolves ref→sha before recording — resolution is part of validating the request —
// and the repo lookup doubles as authorization: unreachable by the installation is 404.
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

	build := &Build{}
	if err := create.Execute(ctx, build); err != nil {
		render.Error(resp, req, 500, err)
		return
	}
	renderCreated(resp, req, respond(build, Latest(nil)))
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
	if _, ok := auth.RequireUser(resp, req); !ok {
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
		SELECT `+buildColumns+` FROM builds
		WHERE owner = $1 AND repo = $2
		ORDER BY id DESC LIMIT $3`,
		chi.URLParam(req, "owner"), chi.URLParam(req, "repo"), limit)
	if err != nil {
		render.Error(resp, req, 500, err)
		return
	}

	renderList(resp, req, builds)
}

// buildResponse is the record as stored plus the fold of its events. The fold is computed
// per read — nothing about how a build went is stored on the row.
type buildResponse struct {
	ID        int64     `json:"id"`
	Trigger   Trigger   `json:"trigger"`
	RetryOf   int64     `json:"retry_of"`
	UserID    int64     `json:"user_id"`
	Owner     string    `json:"owner"`
	Repo      string    `json:"repo"`
	CloneURL  string    `json:"clone_url"`
	Ref       string    `json:"ref"`
	SHA       string    `json:"sha"`
	CreatedAt time.Time `json:"created_at"`

	Status     Status    `json:"status"`
	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at"`
	Image      string    `json:"image"`
	Hash       string    `json:"hash"`
	Error      string    `json:"error"`
}

func list(resp http.ResponseWriter, req *http.Request) {
	if _, ok := auth.RequireUser(resp, req); !ok {
		return
	}

	ctx := req.Context()
	builds := []*Build{}
	err := data.Select(ctx, &builds, `
		SELECT `+buildColumns+` FROM builds ORDER BY id DESC LIMIT $1`, listLimit)
	if err != nil {
		render.Error(resp, req, 500, err)
		return
	}

	renderList(resp, req, builds)
}

// renderList folds each listed build's events and renders the page — the shared back
// half of the global and per-repo lists.
func renderList(resp http.ResponseWriter, req *http.Request, builds []*Build) {
	streams, err := streamsFor(req.Context(), builds)
	if err != nil {
		render.Error(resp, req, 500, err)
		return
	}

	out := make([]buildResponse, len(builds))
	for i, build := range builds {
		out[i] = respond(build, Latest(streams[build.ID]))
	}
	render.JSON(resp, req, out)
}

// streamsFor reads the listed builds' events in one query and groups them by build,
// so a list of n builds costs two queries rather than n+1.
func streamsFor(ctx context.Context, builds []*Build) (map[int64][]*BuildEvent, error) {
	ids := make([]int64, len(builds))
	for i, build := range builds {
		ids[i] = build.ID
	}

	events := []*BuildEvent{}
	err := data.Select(ctx, &events, `
		SELECT * FROM build_events
		WHERE build_id = ANY($1)
		ORDER BY build_id, id`, ids)
	if err != nil {
		return nil, err
	}

	streams := map[int64][]*BuildEvent{}
	for _, event := range events {
		streams[event.BuildID] = append(streams[event.BuildID], event)
	}
	return streams, nil
}

// detailResponse is the detail view: the record plus every attempt folded. Steps stay
// behind the /steps sub-resource so the heavy output payload never rides this read. It
// is a view composed over the domain folds, per spec §Data-domain structs stay flat.
type detailResponse struct {
	buildResponse
	Attempts []attemptResponse `json:"attempts"`
}

type attemptResponse struct {
	Status     Status    `json:"status"`
	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at"`
	Image      string    `json:"image"`
	Hash       string    `json:"hash"`
	Error      string    `json:"error"`
}

func get(resp http.ResponseWriter, req *http.Request) {
	build, events, ok := loadBuild(resp, req)
	if !ok {
		return
	}

	attempts := fold(events)
	out := detailResponse{
		buildResponse: respond(build, Latest(events)),
		Attempts:      make([]attemptResponse, len(attempts)),
	}
	for i, attempt := range attempts {
		out.Attempts[i] = attemptResponse{
			Status:     attempt.Status,
			StartedAt:  attempt.StartedAt,
			FinishedAt: attempt.FinishedAt,
			Image:      attempt.Image,
			Hash:       attempt.Hash,
			Error:      attempt.Error,
		}
	}
	render.JSON(resp, req, out)
}

type stepResponse struct {
	Attempt    int       `json:"attempt"`
	Unit       string    `json:"unit"`
	Step       string    `json:"step"`
	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at"`
	Error      string    `json:"error"`
	Stdout     string    `json:"stdout"`
	Stderr     string    `json:"stderr"`
}

func listSteps(resp http.ResponseWriter, req *http.Request) {
	_, events, ok := loadBuild(resp, req)
	if !ok {
		return
	}

	steps := Steps(events)
	out := make([]stepResponse, len(steps))
	for i, step := range steps {
		out[i] = stepResponse{
			Attempt:    step.Attempt,
			Unit:       step.Unit,
			Step:       step.Step,
			StartedAt:  step.StartedAt,
			FinishedAt: step.FinishedAt,
			Error:      step.Error,
			Stdout:     step.Stdout,
			Stderr:     step.Stderr,
		}
	}
	render.JSON(resp, req, out)
}

// loadBuild gates, resolves {id}, and reads the row plus its whole stream — the shared
// front half of the detail and steps reads. ok=false means a response was written.
func loadBuild(resp http.ResponseWriter, req *http.Request) (*Build, []*BuildEvent, bool) {
	if _, authed := auth.RequireUser(resp, req); !authed {
		return nil, nil, false
	}
	ctx := req.Context()

	id, err := strconv.ParseInt(chi.URLParam(req, "id"), 10, 64)
	if err != nil {
		render.Error(resp, req, 404, httperrors.ErrNotFound)
		return nil, nil, false
	}

	build := &Build{}
	err = data.Get(ctx, build, `SELECT `+buildColumns+` FROM builds WHERE id = $1`, id)
	if data.IsNoRows(err) {
		render.Error(resp, req, 404, httperrors.ErrNotFound)
		return nil, nil, false
	} else if err != nil {
		render.Error(resp, req, 500, err)
		return nil, nil, false
	}

	events := []*BuildEvent{}
	err = data.Select(ctx, &events, `
		SELECT * FROM build_events WHERE build_id = $1 ORDER BY id`, id)
	if err != nil {
		render.Error(resp, req, 500, err)
		return nil, nil, false
	}
	return build, events, true
}

func respond(build *Build, latest BuildAttempt) buildResponse {
	return buildResponse{
		ID:        build.ID,
		Trigger:   build.Trigger,
		RetryOf:   build.RetryOf,
		UserID:    build.UserID,
		Owner:     build.Owner,
		Repo:      build.Repo,
		CloneURL:  build.CloneURL,
		Ref:       build.Ref,
		SHA:       build.SHA,
		CreatedAt: build.CreatedAt,

		Status:     latest.Status,
		StartedAt:  latest.StartedAt,
		FinishedAt: latest.FinishedAt,
		Image:      latest.Image,
		Hash:       latest.Hash,
		Error:      latest.Error,
	}
}
