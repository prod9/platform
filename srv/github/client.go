package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
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
)

// maxRepoPages bounds the repo-list walk so a misbehaving server that keeps returning
// full pages cannot hold a request forever; 100 repos/page makes the cap generous.
const maxRepoPages = 50

// Client is the App API client every fragment consumes; none talks to GitHub's API
// directly except auth's own user-OAuth exchange (spec §"srv owns the App"). App-scoped
// calls authenticate with a fresh App JWT; installation-scoped calls take the caller's
// installation token. Wire facts: docs/vendor/github-app-api.md.
type Client struct {
	app    *App
	apiURL string
}

// Repo is one repository the App installation reaches.
type Repo struct {
	Name     string
	FullName string
	Owner    string
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
	path := fmt.Sprintf("/app/installations/%d/access_tokens", installationID)
	bearer, err := c.app.jwt(time.Now())
	if err != nil {
		return "", err
	}

	body, err := c.fetch(ctx, "POST", path, bearer, "installation token")
	if err != nil {
		return "", err
	}

	var minted struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(body, &minted); err != nil {
		return "", fmt.Errorf("github: decoding installation token: %w", err)
	}
	return minted.Token, nil
}

// InstallationOrg resolves an installation to the org it is installed on.
func (c *Client) InstallationOrg(ctx context.Context, installationID int64) (*Org, error) {
	path := fmt.Sprintf("/app/installations/%d", installationID)
	bearer, err := c.app.jwt(time.Now())
	if err != nil {
		return nil, err
	}

	body, err := c.fetch(ctx, "GET", path, bearer, "installation lookup")
	if err != nil {
		return nil, err
	}

	var installation struct {
		Account struct {
			ID    int64  `json:"id"`
			Login string `json:"login"`
		} `json:"account"`
	}
	if err := json.Unmarshal(body, &installation); err != nil {
		return nil, fmt.Errorf("github: decoding installation: %w", err)
	}
	return &Org{installation.Account.ID, installation.Account.Login}, nil
}

// AppPermissions reads the App's own permission map — slug → "read"|"write" —
// authenticated as the App itself (GET /app, JWT).
func (c *Client) AppPermissions(ctx context.Context) (map[string]string, error) {
	bearer, err := c.app.jwt(time.Now())
	if err != nil {
		return nil, err
	}

	body, err := c.fetch(ctx, "GET", "/app", bearer, "app lookup")
	if err != nil {
		return nil, err
	}

	var app struct {
		Permissions map[string]string `json:"permissions"`
	}
	if err := json.Unmarshal(body, &app); err != nil {
		return nil, fmt.Errorf("github: decoding app: %w", err)
	}
	return app.Permissions, nil
}

// IsOrgOwner reports whether user holds an active owner (admin) membership in org.
// A 404 means "not a member" — false, not an error.
func (c *Client) IsOrgOwner(ctx context.Context, token, org, user string) (bool, error) {
	path := fmt.Sprintf("/orgs/%s/memberships/%s", org, user)
	resp, err := c.send(ctx, "GET", path, token, "")
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}
	if resp.StatusCode != http.StatusOK {
		return false, RespError("membership lookup", resp)
	}

	var membership struct {
		Role  string `json:"role"`
		State string `json:"state"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&membership); err != nil {
		return false, fmt.Errorf("github: decoding membership: %w", err)
	}
	return membership.Role == "admin" && membership.State == "active", nil
}

// Repos lists every repository the installation reaches, walking all pages.
func (c *Client) Repos(ctx context.Context, token string) ([]Repo, error) {
	const perPage = 100

	var repos []Repo
	for page := 1; page <= maxRepoPages; page++ {
		path := fmt.Sprintf("/installation/repositories?per_page=%d&page=%d", perPage, page)
		body, err := c.fetch(ctx, "GET", path, token, "repo list")
		if err != nil {
			return nil, err
		}

		var listing struct {
			Repositories []struct {
				Name     string `json:"name"`
				FullName string `json:"full_name"`
				Owner    struct {
					Login string `json:"login"`
				} `json:"owner"`
			} `json:"repositories"`
		}
		if err := json.Unmarshal(body, &listing); err != nil {
			return nil, fmt.Errorf("github: decoding repo list: %w", err)
		}

		for _, repo := range listing.Repositories {
			repos = append(repos, Repo{repo.Name, repo.FullName, repo.Owner.Login})
		}
		if len(listing.Repositories) < perPage {
			return repos, nil
		}
	}
	return nil, fmt.Errorf("github: repo list did not end within %d pages", maxRepoPages)
}

// RepoCloneURL fetches the repo's clone URL. It doubles as the reachability check for
// the manual build trigger: ErrRepoUnreachable when the installation cannot see the
// repo, so the caller can answer 404 instead of recording an unbuildable intent.
func (c *Client) RepoCloneURL(ctx context.Context, token, owner, repo string) (string, error) {
	if err := CheckRepoPath(owner, repo); err != nil {
		return "", err
	}

	path := fmt.Sprintf("/repos/%s/%s", owner, repo)
	resp, err := c.send(ctx, "GET", path, token, "")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return "", ErrRepoUnreachable
	}
	if resp.StatusCode != http.StatusOK {
		return "", RespError("repo lookup", resp)
	}

	var repository struct {
		CloneURL string `json:"clone_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&repository); err != nil {
		return "", fmt.Errorf("github: decoding repo: %w", err)
	}
	return repository.CloneURL, nil
}

// ResolveRef resolves a ref (sha, heads/BRANCH, or tags/TAG) to its commit sha via
// the vnd.github.sha media type, whose response body is the bare sha.
func (c *Client) ResolveRef(ctx context.Context, token, owner, repo, ref string) (string, error) {
	if err := CheckRepoPath(owner, repo); err != nil {
		return "", err
	}

	path := fmt.Sprintf("/repos/%s/%s/commits/%s", owner, repo, ref)
	resp, err := c.send(ctx, "GET", path, token, "application/vnd.github.sha")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// 404 = ref (or repo) absent; 422 = a name the endpoint cannot resolve. Both are
	// the caller's ref being wrong, not a server fault.
	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusUnprocessableEntity {
		return "", ErrRefUnresolvable
	}
	if resp.StatusCode != http.StatusOK {
		return "", RespError("ref resolution", resp)
	}
	sha, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("github: reading resolved sha: %w", err)
	}
	return string(sha), nil
}

// send performs the request and returns the raw response — status handling is the
// caller's. fetch is the common wrapper for calls where any non-2xx is an error.
func (c *Client) send(ctx context.Context, method, path, bearer, accept string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.apiURL+path, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+bearer)
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	return httpClient.Do(req)
}

func (c *Client) fetch(ctx context.Context, method, path, bearer, op string) ([]byte, error) {
	resp, err := c.send(ctx, method, path, bearer, "")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, RespError(op, resp)
	}
	return io.ReadAll(resp.Body)
}
