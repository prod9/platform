package builds

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"fx.prodigy9.co/config"
	"fx.prodigy9.co/data"
	"fx.prodigy9.co/httpserver/controllers"
	"fx.prodigy9.co/httpserver/render"
	"github.com/go-chi/chi/v5"
	"platform.prodigy9.co/srv/auth"
	"platform.prodigy9.co/srv/github"
	"platform.prodigy9.co/srv/install"
)

// errTriggerIncomplete rejects a manual trigger missing any part of the domain fact —
// which repo, at which ref.
var errTriggerIncomplete = errors.New("builds: owner, repo, and ref are all required")

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
	router.Post("/api/builds", trigger)
	router.Get("/api/repos", listRepos)
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

	var body struct {
		Owner string `json:"owner"`
		Repo  string `json:"repo"`
		Ref   string `json:"ref"`
	}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		render.Error(resp, req, 400, err)
		return
	}
	if body.Owner == "" || body.Repo == "" || body.Ref == "" {
		render.Error(resp, req, 400, errTriggerIncomplete)
		return
	}

	token, client, err := installationClient(ctx)
	if err != nil {
		render.Error(resp, req, 500, err)
		return
	}
	cloneURL, err := client.RepoCloneURL(ctx, token, body.Owner, body.Repo)
	if errors.Is(err, github.ErrRepoUnreachable) {
		render.Error(resp, req, 404, err)
		return
	} else if err != nil {
		render.Error(resp, req, 500, err)
		return
	}
	sha, err := client.ResolveRef(ctx, token, body.Owner, body.Repo,
		strings.TrimPrefix(body.Ref, "refs/"))
	if err != nil {
		render.Error(resp, req, 500, err)
		return
	}

	create := &Create{
		Trigger:  TriggerWebUI,
		UserID:   user.ID,
		Owner:    body.Owner,
		Repo:     body.Repo,
		CloneURL: cloneURL,
		Ref:      body.Ref,
		SHA:      sha,
	}
	build := &Build{}
	if err := create.Execute(ctx, build); err != nil {
		render.Error(resp, req, 500, err)
		return
	}

	resp.Header().Set("Content-Type", "application/json")
	resp.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(resp).Encode(respond(build, Latest(nil))); err != nil {
		render.Error(resp, req, 500, err)
	}
}

// listRepos serves the webui's repo picker, listed live from GitHub — a stored repo
// table would be RBAC state the zero-RBAC model forbids (spec §Operations).
func listRepos(resp http.ResponseWriter, req *http.Request) {
	if _, ok := auth.RequireUser(resp, req); !ok {
		return
	}
	ctx := req.Context()

	token, client, err := installationClient(ctx)
	if err != nil {
		render.Error(resp, req, 500, err)
		return
	}
	repos, err := client.Repos(ctx, token)
	if err != nil {
		render.Error(resp, req, 500, err)
		return
	}

	listing := make([]repoResponse, 0, len(repos))
	for _, repo := range repos {
		listing = append(listing, repoResponse{repo.Name, repo.FullName, repo.Owner})
	}
	render.JSON(resp, req, listing)
}

type repoResponse struct {
	Name     string `json:"name"`
	FullName string `json:"full_name"`
	Owner    string `json:"owner"`
}

// installationClient pairs a fresh installation token with the App client that minted
// it — every GitHub call these handlers make is installation-scoped.
func installationClient(ctx context.Context) (string, *github.Client, error) {
	token, err := install.Token(ctx)
	if err != nil {
		return "", nil, err
	}

	client, err := github.NewClient(ctx)
	if err != nil {
		return "", nil, err
	}
	return token, client, nil
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

	streams, err := streamsFor(ctx, listLimit)
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

// streamsFor reads the events of the newest builds in one query and groups them by build,
// so a list of n builds costs two queries rather than n+1.
func streamsFor(ctx context.Context, limit int) (map[int64][]*BuildEvent, error) {
	events := []*BuildEvent{}
	err := data.Select(ctx, &events, `
		SELECT * FROM build_events
		WHERE build_id IN (SELECT id FROM builds ORDER BY id DESC LIMIT $1)
		ORDER BY build_id, id`, limit)
	if err != nil {
		return nil, err
	}

	streams := map[int64][]*BuildEvent{}
	for _, event := range events {
		streams[event.BuildID] = append(streams[event.BuildID], event)
	}
	return streams, nil
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
