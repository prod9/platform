package repos

import (
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strings"

	"fx.prodigy9.co/config"
	"fx.prodigy9.co/fxlog"
	"fx.prodigy9.co/httpserver/controllers"
	"fx.prodigy9.co/httpserver/render"
	"github.com/go-chi/chi/v5"
	"platform.prodigy9.co/conf"
	"platform.prodigy9.co/srv/auth"
	"platform.prodigy9.co/srv/github"
	"platform.prodigy9.co/srv/install"
)

// RepoCtr serves repo registration: the registered∩authorized listing, the onboarding
// wizard's candidate list and manifest pre-read, and the one registration write
// (spec §Repos are registered, authorization is GitHub-derived).
type RepoCtr struct{}

var _ controllers.Interface = RepoCtr{}

func (RepoCtr) Mount(cfg *config.Source, router chi.Router) error {
	router.Get("/api/repos", list)
	router.Get("/api/repos/candidates", candidates)
	router.Post("/api/repos", register)
	router.Get("/api/repos/{owner}/{repo}/manifest", manifest)
	return nil
}

type repoResponse struct {
	Owner    string `json:"owner"`
	Repo     string `json:"repo"`
	FullName string `json:"full_name"`
}

// list is the stored registrations filtered by the session's authorization snapshot.
func list(resp http.ResponseWriter, req *http.Request) {
	if _, ok := auth.RequireUser(resp, req); !ok {
		return
	}
	ctx := req.Context()

	registered, err := ListRegistered(ctx)
	if err != nil {
		render.Error(resp, req, 500, err)
		return
	}
	if len(registered) == 0 {
		render.JSON(resp, req, []repoResponse{})
		return
	}

	reachable, err := auth.ReadableRepos(req)
	if err != nil {
		render.Error(resp, req, 500, err)
		return
	}

	listing := []repoResponse{}
	for _, row := range registered {
		if repo, ok := findRepo(reachable, row.Owner, row.Repo); ok {
			listing = append(listing, repoResponse{repo.Owner, repo.Name, repo.Owner + "/" + repo.Name})
		}
	}
	render.JSON(resp, req, listing)
}

// candidates lists what the onboarding wizard may pick: repos reachable by both the
// session user and the App, minus registrations.
func candidates(resp http.ResponseWriter, req *http.Request) {
	if _, ok := auth.RequireUser(resp, req); !ok {
		return
	}
	ctx := req.Context()

	all, err := auth.ReadableRepos(req)
	if err != nil {
		render.Error(resp, req, 500, err)
		return
	}
	registered, err := ListRegistered(ctx)
	if err != nil {
		render.Error(resp, req, 500, err)
		return
	}

	listing := []repoResponse{}
	for _, repo := range all {
		if !isRegistered(registered, repo) {
			listing = append(listing, repoResponse{repo.Owner, repo.Name, repo.Owner + "/" + repo.Name})
		}
	}
	render.JSON(resp, req, listing)
}

// register records a registration after the session's write gate; installation
// reachability independently ensures the App can build what is registered.
func register(resp http.ResponseWriter, req *http.Request) {
	user, ok := auth.RequireUser(resp, req)
	if !ok {
		return
	}
	ctx := req.Context()

	action := &RegisterRepo{UserID: user.ID}
	if err := controllers.ReadAction(req, action); err != nil {
		render.Error(resp, req, 400, err)
		return
	}
	if !auth.RequireRepoWrite(resp, req, action.Owner, action.Repo) {
		return
	}

	token, client, err := install.Token(ctx)
	if err != nil {
		render.Error(resp, req, 500, err)
		return
	}
	observed, err := client.RepoManifestAt(
		ctx, token, action.Owner, action.Repo, action.ManifestSHA)
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
	parsed, err := parseManifest(observed.Raw, action.Repo)
	if err != nil {
		render.Error(resp, req, 422, err)
		return
	}
	action.ManifestRaw = string(observed.Raw)
	action.Manifest = *parsed

	row := &Repo{}
	if err := action.Execute(ctx, row); errors.Is(err, ErrAlreadyRegistered) {
		render.Error(resp, req, 409, err)
		return
	} else if err != nil {
		render.Error(resp, req, 500, err)
		return
	}
	renderCreated(resp, req, repoResponse{row.Owner, row.Repo, row.Owner + "/" + row.Repo})
}

type manifestResponse struct {
	SHA           string             `json:"sha"`
	Maintainer    string             `json:"maintainer"`
	Repository    string             `json:"repository"`
	PublishPolicy conf.PublishPolicy `json:"publish_policy"`
	Modules       []moduleResponse   `json:"modules"`
}

type moduleResponse struct {
	Name      string `json:"name"`
	Framework string `json:"framework"`
	WorkDir   string `json:"workdir"`
}

// manifest is the wizard's review step: the repo's platform.toml read live from
// GitHub at the default branch's head and parsed through the real parser — the user
// confirms what the server pre-read, not what the client guessed. A manifest that
// does not parse is the repo's data problem, answered 422.
func manifest(resp http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	owner, repo := chi.URLParam(req, "owner"), chi.URLParam(req, "repo")
	if !auth.RequireRepoRead(resp, req, owner, repo) {
		return
	}

	token, client, err := install.Token(ctx)
	if err != nil {
		render.Error(resp, req, 500, err)
		return
	}
	observed, err := client.RepoManifest(ctx, token, owner, repo)
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
	parsed, err := parseManifest(observed.Raw, repo)
	if err != nil {
		render.Error(resp, req, 422, err)
		return
	}

	out := manifestResponse{
		SHA:           observed.SHA,
		Maintainer:    parsed.Maintainer,
		Repository:    parsed.Repository,
		PublishPolicy: parsed.Server.Publish,
		Modules:       []moduleResponse{},
	}
	for name, module := range parsed.Modules {
		out.Modules = append(out.Modules, moduleResponse{name, module.Framework, module.WorkDir})
	}
	sort.Slice(out.Modules, func(i, j int) bool { return out.Modules[i].Name < out.Modules[j].Name })
	render.JSON(resp, req, out)
}

func parseManifest(raw []byte, repo string) (*conf.Model, error) {
	model, err := conf.Parse(raw)
	if err != nil {
		return nil, err
	}

	policy, err := model.ResolvePublishPolicy(repo)
	if err != nil {
		return nil, err
	}
	model.Server.Publish = policy
	return model, nil
}

// findRepo matches a registration row against the session snapshot; GitHub logins and
// repo names compare case-insensitively.
func findRepo(repositories []auth.Repository, owner, name string) (auth.Repository, bool) {
	for _, repo := range repositories {
		if strings.EqualFold(repo.Owner, owner) && strings.EqualFold(repo.Name, name) {
			return repo, true
		}
	}
	return auth.Repository{}, false
}

func isRegistered(registered []*Repo, repo auth.Repository) bool {
	for _, row := range registered {
		if strings.EqualFold(row.Owner, repo.Owner) && strings.EqualFold(row.Repo, repo.Name) {
			return true
		}
	}
	return false
}

// renderCreated is render.JSON at 201 — fx's render fixes status 200, so the
// created-returning handler writes the status itself (same fx gap builds works
// around). The 201 is committed before encoding; an encode failure is logged.
func renderCreated(resp http.ResponseWriter, req *http.Request, obj any) {
	resp.Header().Set("Content-Type", "application/json")
	resp.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(resp).Encode(obj); err != nil {
		fxlog.Log("encoding 201 response failed after commit",
			fxlog.String("path", req.URL.Path),
			fxlog.String("error", err.Error()))
	}
}
