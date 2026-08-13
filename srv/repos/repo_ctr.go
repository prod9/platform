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

// RepoCtr serves repo registration: the registered∩live listing, the onboarding
// wizard's candidate list and manifest pre-read, and the one registration write
// (spec §Repos are registered, visibility is live).
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

// list is registered∩live: the stored registrations filtered by what the session
// user can currently reach on GitHub, read with the user's own token — losing GitHub
// permission loses platform visibility in the same moment.
func list(resp http.ResponseWriter, req *http.Request) {
	user, ok := auth.RequireUser(resp, req)
	if !ok {
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

	userToken, err := auth.GitHubToken(ctx, user.ID)
	if err != nil {
		render.Error(resp, req, 500, err)
		return
	}
	record, err := install.Bound(ctx)
	if err != nil {
		render.Error(resp, req, 500, err)
		return
	}
	client, err := github.NewClient(ctx)
	if err != nil {
		render.Error(resp, req, 500, err)
		return
	}
	reachable, err := client.UserInstallationRepos(ctx, userToken, record.InstallationID)
	if err != nil {
		render.Error(resp, req, 500, err)
		return
	}

	listing := []repoResponse{}
	for _, row := range registered {
		if repo, ok := findRepo(reachable, row.Owner, row.Repo); ok {
			listing = append(listing, repoResponse{repo.Owner, repo.Name, repo.FullName})
		}
	}
	render.JSON(resp, req, listing)
}

// candidates lists what the onboarding wizard may pick: the App-reachable repos not
// yet registered, live from GitHub.
func candidates(resp http.ResponseWriter, req *http.Request) {
	if _, ok := auth.RequireUser(resp, req); !ok {
		return
	}
	ctx := req.Context()

	token, client, err := install.Token(ctx)
	if err != nil {
		render.Error(resp, req, 500, err)
		return
	}
	all, err := client.Repos(ctx, token)
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
			listing = append(listing, repoResponse{repo.Owner, repo.Name, repo.FullName})
		}
	}
	render.JSON(resp, req, listing)
}

// register records a registration. Reachability by the installation is the boundary
// check — the same 404 the manual build trigger draws for a repo the App cannot see.
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

	token, client, err := install.Token(ctx)
	if err != nil {
		render.Error(resp, req, 500, err)
		return
	}
	if _, err := client.RepoCloneURL(ctx, token, action.Owner, action.Repo); errors.Is(err, github.ErrRepoUnreachable) {
		render.Error(resp, req, 404, err)
		return
	} else if err != nil {
		render.Error(resp, req, 500, err)
		return
	}

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
	Maintainer string           `json:"maintainer"`
	Repository string           `json:"repository"`
	Modules    []moduleResponse `json:"modules"`
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
	if _, ok := auth.RequireUser(resp, req); !ok {
		return
	}
	ctx := req.Context()
	owner, repo := chi.URLParam(req, "owner"), chi.URLParam(req, "repo")

	token, client, err := install.Token(ctx)
	if err != nil {
		render.Error(resp, req, 500, err)
		return
	}
	raw, err := client.RepoManifest(ctx, token, owner, repo)
	if errors.Is(err, github.ErrNoManifest) {
		render.Error(resp, req, 404, err)
		return
	} else if err != nil {
		render.Error(resp, req, 500, err)
		return
	}
	parsed, err := conf.Parse(raw)
	if err != nil {
		render.Error(resp, req, 422, err)
		return
	}

	out := manifestResponse{
		Maintainer: parsed.Maintainer,
		Repository: parsed.Repository,
		Modules:    []moduleResponse{},
	}
	for name, module := range parsed.Modules {
		out.Modules = append(out.Modules, moduleResponse{name, module.Framework, module.WorkDir})
	}
	sort.Slice(out.Modules, func(i, j int) bool { return out.Modules[i].Name < out.Modules[j].Name })
	render.JSON(resp, req, out)
}

// findRepo matches a registration row against the live list; GitHub logins and repo
// names compare case-insensitively.
func findRepo(live []github.Repo, owner, name string) (github.Repo, bool) {
	for _, repo := range live {
		if strings.EqualFold(repo.Owner, owner) && strings.EqualFold(repo.Name, name) {
			return repo, true
		}
	}
	return github.Repo{}, false
}

func isRegistered(registered []*Repo, repo github.Repo) bool {
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
