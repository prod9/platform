package github

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"fx.prodigy9.co/config"
)

var (
	httpClient = &http.Client{Timeout: 10 * time.Second}

	// ErrRepoUnreachable reports that the installation cannot see the repo — absent,
	// or outside the granted repository set.
	ErrRepoUnreachable = errors.New("github: repo unreachable by this installation")

	// ErrRefUnresolvable reports a ref the repo does not have — the manual trigger's
	// likely typo, answered as the caller's error rather than the server's.
	ErrRefUnresolvable = errors.New("github: ref does not resolve to a commit")

	// ErrNoManifest reports a repo with no platform.toml at its default branch's
	// head — or one that just fell out of reach; GitHub answers both 404 and the
	// wizard treats both as "nothing to review".
	ErrNoManifest = errors.New("github: repo has no platform.toml")

	// ErrUnauthorized reports that GitHub rejected the supplied user credential.
	ErrUnauthorized = errors.New("github: unauthorized")

	// errNotMember folds the membership endpoint's 404 — "not a member" — into
	// IsOrgOwner's false.
	errNotMember = errors.New("github: not an org member")

	// errNotInstalledOnOrg folds the org-installation lookup's 404 — GitHub's
	// standard "not installed" answer — into InstalledOnOrg's false.
	errNotInstalledOnOrg = errors.New("github: app not installed on org")
)

// Client is the App API client every fragment consumes; none talks to GitHub's API
// directly except auth's own user-OAuth exchange (spec §"srv owns the App"). App-scoped
// calls authenticate with a fresh App JWT; installation-scoped calls take the caller's
// installation token. Wire facts: docs/vendor/github-app-api.md.
type Client struct {
	app    *App
	apiURL string
}

type RepoPermission string

const (
	RepoRead  RepoPermission = "read"
	RepoWrite RepoPermission = "write"
)

// Repo is one repository the App installation reaches.
type Repo struct {
	ID         int64
	Name       string
	FullName   string
	Owner      string
	Permission RepoPermission
}

// Manifest is platform.toml as observed at one immutable commit.
type Manifest struct {
	SHA string
	Raw []byte
}

type repository struct {
	CloneURL      string `json:"clone_url"`
	DefaultBranch string `json:"default_branch"`
}

// Org is the account an installation is installed on: the rename-stable numeric id
// and the current login.
type Org struct {
	ID    int64
	Login string
}

func NewClient(ctx context.Context) (*Client, error) {
	app, err := LoadApp(ctx)
	if err != nil {
		return nil, err
	}

	cfg := config.FromContext(ctx)
	return &Client{app: app, apiURL: config.Get(cfg, APIURLConfig)}, nil
}

// InstallationToken mints a short-lived (~1h) installation token — mint per
// operation, never store.
func (c *Client) InstallationToken(ctx context.Context, installationID int64) (string, error) {
	minted, err := fetchJSON[struct {
		Token string `json:"token"`
	}](ctx, c, request{
		method: "POST",
		path:   fmt.Sprintf("/app/installations/%d/access_tokens", installationID),
		auth:   c.asApp(),
		op:     "installation token",
	})
	return minted.Token, err
}

// InstallationOrg resolves an installation to the org it is installed on.
func (c *Client) InstallationOrg(ctx context.Context, installationID int64) (*Org, error) {
	installation, err := fetchJSON[struct {
		Account struct {
			ID    int64  `json:"id"`
			Login string `json:"login"`
		} `json:"account"`
	}](ctx, c, request{
		method: "GET",
		path:   fmt.Sprintf("/app/installations/%d", installationID),
		auth:   c.asApp(),
		op:     "installation lookup",
	})
	if err != nil {
		return nil, err
	}
	return &Org{installation.Account.ID, installation.Account.Login}, nil
}

// InstalledOnOrg reports whether the App is installed on org — the direct
// org-installation lookup, authenticated as the App itself; GitHub's 404 means
// not installed.
func (c *Client) InstalledOnOrg(ctx context.Context, org string) (bool, error) {
	_, err := fetchJSON[struct{}](ctx, c, request{
		method: "GET",
		path:   fmt.Sprintf("/orgs/%s/installation", org),
		auth:   c.asApp(),
		op:     "org installation lookup",
		status: map[int]error{http.StatusNotFound: errNotInstalledOnOrg},
	})
	if errors.Is(err, errNotInstalledOnOrg) {
		return false, nil
	}
	return err == nil, err
}

// AppPermissions reads the App's own permission map — slug → "read"|"write" —
// authenticated as the App itself (GET /app, JWT).
func (c *Client) AppPermissions(ctx context.Context) (map[string]string, error) {
	app, err := fetchJSON[struct {
		Permissions map[string]string `json:"permissions"`
	}](ctx, c, request{
		method: "GET",
		path:   "/app",
		auth:   c.asApp(),
		op:     "app lookup",
	})
	return app.Permissions, err
}

// IsOrgOwner reports whether user holds an active owner (admin) membership in org.
// A 404 means "not a member" — false, not an error.
func (c *Client) IsOrgOwner(ctx context.Context, token, org, user string) (bool, error) {
	membership, err := fetchJSON[struct {
		Role  string `json:"role"`
		State string `json:"state"`
	}](ctx, c, request{
		method: "GET",
		path:   fmt.Sprintf("/orgs/%s/memberships/%s", org, user),
		auth:   asToken(token),
		op:     "membership lookup",
		status: map[int]error{http.StatusNotFound: errNotMember},
	})
	if errors.Is(err, errNotMember) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return membership.Role == "admin" && membership.State == "active", nil
}

// Repos lists every repository the installation reaches, walking all pages.
func (c *Client) Repos(ctx context.Context, token string) ([]Repo, error) {
	type repoPage struct {
		Repositories []struct {
			ID       int64  `json:"id"`
			Name     string `json:"name"`
			FullName string `json:"full_name"`
			Owner    struct {
				Login string `json:"login"`
			} `json:"owner"`
		} `json:"repositories"`
	}

	var repos []Repo
	err := fetchPaged(ctx, c, request{
		method: "GET",
		path:   "/installation/repositories?per_page=100",
		auth:   asToken(token),
		op:     "repo list",
	}, func(page repoPage) {
		for _, repo := range page.Repositories {
			repos = append(repos, Repo{ID: repo.ID, Name: repo.Name,
				FullName: repo.FullName, Owner: repo.Owner.Login})
		}
	})
	if err != nil {
		return nil, err
	}
	return repos, nil
}

// UserInstallationRepos lists the repositories the given user can reach through one
// installation, authenticated with the user's own OAuth token — the live half of the
// registered∩live visibility rule (spec §Repos are registered, visibility is live).
func (c *Client) UserInstallationRepos(ctx context.Context, userToken string, installationID int64) ([]Repo, error) {
	type repoPage struct {
		Repositories []struct {
			ID       int64  `json:"id"`
			Name     string `json:"name"`
			FullName string `json:"full_name"`
			Owner    struct {
				Login string `json:"login"`
			} `json:"owner"`
			Permissions struct {
				Push     bool `json:"push"`
				Admin    bool `json:"admin"`
				Maintain bool `json:"maintain"`
			} `json:"permissions"`
		} `json:"repositories"`
	}

	var repos []Repo
	err := fetchPaged(ctx, c, request{
		method: "GET",
		path:   fmt.Sprintf("/user/installations/%d/repositories?per_page=100", installationID),
		auth:   asToken(userToken),
		op:     "user installation repo list",
		status: map[int]error{http.StatusUnauthorized: ErrUnauthorized},
	}, func(page repoPage) {
		for _, repo := range page.Repositories {
			permission := RepoRead
			if repo.Permissions.Push || repo.Permissions.Admin || repo.Permissions.Maintain {
				permission = RepoWrite
			}
			repos = append(repos, Repo{ID: repo.ID, Name: repo.Name,
				FullName: repo.FullName, Owner: repo.Owner.Login, Permission: permission})
		}
	})
	if err != nil {
		return nil, err
	}
	return repos, nil
}

// RepoManifest resolves the repo's default-branch head and reads platform.toml at that
// immutable commit. The SHA is what lets onboarding confirm exactly what it reviewed.
func (c *Client) RepoManifest(ctx context.Context, token, owner, repo string) (*Manifest, error) {
	if err := CheckRepoPath(owner, repo); err != nil {
		return nil, err
	}
	repository, err := c.repository(ctx, token, owner, repo)
	if err != nil {
		return nil, err
	}
	sha, err := c.ResolveRef(ctx, token, owner, repo, repository.DefaultBranch)
	if err != nil {
		return nil, err
	}
	return c.repoManifestAt(ctx, token, owner, repo, sha)
}

// RepoManifestAt reads platform.toml at sha. Callers provide a resolved commit rather
// than a moving ref, so the bytes cannot change between review and confirmation.
func (c *Client) RepoManifestAt(ctx context.Context, token, owner, repo, sha string) (*Manifest, error) {
	if err := CheckRepoPath(owner, repo); err != nil {
		return nil, err
	}
	if _, err := c.repository(ctx, token, owner, repo); err != nil {
		return nil, err
	}
	return c.repoManifestAt(ctx, token, owner, repo, sha)
}

func (c *Client) repoManifestAt(ctx context.Context, token, owner, repo, sha string) (*Manifest, error) {
	raw, err := fetchRaw(ctx, c, request{
		method: "GET",
		path: fmt.Sprintf("/repos/%s/%s/contents/platform.toml?ref=%s",
			owner, repo, url.QueryEscape(sha)),
		auth:   asToken(token),
		accept: "application/vnd.github.raw+json",
		op:     "manifest read",
		status: map[int]error{http.StatusNotFound: ErrNoManifest},
	})
	if err != nil {
		return nil, err
	}
	return &Manifest{SHA: sha, Raw: raw}, nil
}

// RepoCloneURL fetches the repo's clone URL. It doubles as the reachability check for
// the manual build trigger: ErrRepoUnreachable when the installation cannot see the
// repo, so the caller can answer 404 instead of recording an unbuildable intent.
func (c *Client) RepoCloneURL(ctx context.Context, token, owner, repo string) (string, error) {
	if err := CheckRepoPath(owner, repo); err != nil {
		return "", err
	}

	repository, err := c.repository(ctx, token, owner, repo)
	return repository.CloneURL, err
}

func (c *Client) repository(ctx context.Context, token, owner, repo string) (*repository, error) {
	repository, err := fetchJSON[repository](ctx, c, request{
		method: "GET",
		path:   fmt.Sprintf("/repos/%s/%s", owner, repo),
		auth:   asToken(token),
		op:     "repo lookup",
		status: map[int]error{http.StatusNotFound: ErrRepoUnreachable},
	})
	return &repository, err
}

// ResolveRef resolves a ref (sha, heads/BRANCH, or tags/TAG) to its commit sha via
// the vnd.github.sha media type, whose response body is the bare sha. GitHub's 404
// (repo absent) and 422 (unresolvable name) are both the caller's ref being wrong,
// not a server fault.
func (c *Client) ResolveRef(ctx context.Context, token, owner, repo, ref string) (string, error) {
	if err := CheckRepoPath(owner, repo); err != nil {
		return "", err
	}

	sha, err := fetchRaw(ctx, c, request{
		method: "GET",
		path:   fmt.Sprintf("/repos/%s/%s/commits/%s", owner, repo, ref),
		auth:   asToken(token),
		accept: "application/vnd.github.sha",
		op:     "ref resolution",
		status: map[int]error{
			http.StatusNotFound:            ErrRefUnresolvable,
			http.StatusUnprocessableEntity: ErrRefUnresolvable,
		},
	})
	return string(sha), err
}
