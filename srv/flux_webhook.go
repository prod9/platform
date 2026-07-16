package srv

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"fx.prodigy9.co/config"
	"fx.prodigy9.co/httpserver/controllers"
	"fx.prodigy9.co/httpserver/render"
	"github.com/go-chi/chi/v5"
)

// ErrWebhookExists reports a repo that already carries the hook config GitHub was
// asked to create; GitHub answers a duplicate with a 422.
var ErrWebhookExists = errors.New("srv: flux webhook already configured")

// errNoRepoPush reports a repo the session user cannot push to — GitHub answers a repo
// the user cannot even see with a 404, so both fail the same way.
var errNoRepoPush = errors.New("srv: no push access to repo")

// FluxWebhook closes the flux-webhook ADR's manual GitHub-side step: it points a
// repo's webhook at the cluster's Flux Receiver so a GHCR publish pokes the
// reconcile. Deploy-adjacent config, so the endpoint is session-gated.
type FluxWebhook struct{}

var _ controllers.Interface = FluxWebhook{}

func (FluxWebhook) Mount(cfg *config.Source, router chi.Router) error {
	router.Post("/api/repos/{owner}/{repo}/flux-webhook", configureFluxWebhook)
	return nil
}

func configureFluxWebhook(resp http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	user, ok := requireUser(resp, req)
	if !ok {
		return
	}

	app, err := loadGitHubApp(ctx)
	if errors.Is(err, ErrNoGitHubApp) {
		render.Error(resp, req, 503, err)
		return
	} else if err != nil {
		render.Error(resp, req, 500, err)
		return
	}

	action := &ConfigureFluxWebhook{
		Owner: chi.URLParam(req, "owner"),
		Repo:  chi.URLParam(req, "repo"),
		app:   app,
	}
	if err := controllers.ReadAction(req, action); err != nil {
		render.Error(resp, req, 400, err)
		return
	}

	// the hook is created with an installation token, which carries no notion of the
	// session user — so their write access is checked explicitly first (spec §Two
	// token types).
	userToken, err := loadUserGitHubToken(ctx, user.ID)
	if errors.Is(err, errNoUserGitHubToken) {
		render.Error(resp, req, 403, err)
		return
	} else if err != nil {
		render.Error(resp, req, 500, err)
		return
	}

	apiURL := config.Get(config.FromContext(ctx), GitHubAPIURLConfig)
	switch err := checkRepoPush(ctx, http.DefaultClient, apiURL, userToken, action.Owner, action.Repo); {
	case errors.Is(err, errNoRepoPush):
		render.Error(resp, req, 403, err)
		return
	case err != nil:
		render.Error(resp, req, 502, err)
		return
	}

	switch err := action.Execute(ctx, nil); {
	case errors.Is(err, ErrAppNotInstalled):
		render.Error(resp, req, 404, err)
	case errors.Is(err, ErrWebhookExists):
		render.Error(resp, req, 409, err)
	case err != nil:
		render.Error(resp, req, 502, err)
	default:
		render.JSON(resp, req, struct {
			Status string `json:"status"`
		}{"configured"})
	}
}

// ConfigureFluxWebhook creates the repo's registry_package webhook pointing at the
// cluster's Flux Receiver URL with its HMAC secret, authenticated as the App's
// installation on the repo. Owner/Repo come from the URL, never the body.
type ConfigureFluxWebhook struct {
	Owner       string `json:"-"`
	Repo        string `json:"-"`
	ReceiverURL string `json:"receiver_url"`
	Secret      string `json:"secret"`

	app *GitHubApp
}

var _ controllers.Validator = (*ConfigureFluxWebhook)(nil)

func (c *ConfigureFluxWebhook) Validate() error {
	if err := checkRepoPath(c.Owner, c.Repo); err != nil {
		return err
	}

	receiver, err := url.Parse(c.ReceiverURL)
	if err != nil {
		return fmt.Errorf("srv: invalid receiver_url: %w", err)
	}
	if receiver.Scheme != "https" || receiver.Host == "" {
		return fmt.Errorf("srv: receiver_url must be an https URL, got %q", c.ReceiverURL)
	}
	if c.Secret == "" {
		return errors.New("srv: secret must not be empty")
	}
	return nil
}

// hookRequest is POST /repos/{owner}/{repo}/hooks' wire shape.
type hookRequest struct {
	Name   string     `json:"name"`
	Active bool       `json:"active"`
	Events []string   `json:"events"`
	Config hookConfig `json:"config"`
}

type hookConfig struct {
	URL         string `json:"url"`
	ContentType string `json:"content_type"`
	Secret      string `json:"secret"`
}

func (c *ConfigureFluxWebhook) Execute(ctx context.Context, out any) error {
	cfg := config.FromContext(ctx)
	apiURL := strings.TrimSuffix(config.Get(cfg, GitHubAPIURLConfig), "/")

	token, err := mintInstallationToken(ctx, http.DefaultClient, apiURL, c.app, c.Owner, c.Repo)
	if err != nil {
		return err
	}

	body, err := json.Marshal(hookRequest{
		Name:   "web",
		Active: true,
		Events: []string{"registry_package"},
		Config: hookConfig{URL: c.ReceiverURL, ContentType: "json", Secret: c.Secret},
	})
	if err != nil {
		return err
	}
	hooksURL := apiURL + "/repos/" + c.Owner + "/" + c.Repo + "/hooks"
	req, err := http.NewRequestWithContext(ctx, "POST", hooksURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusUnprocessableEntity:
		return fmt.Errorf("%w on %s/%s", ErrWebhookExists, c.Owner, c.Repo)
	case resp.StatusCode != http.StatusCreated:
		return githubRespError("creating flux webhook", resp)
	}
	return nil
}

// repoAccess is the subset of GET /repos/{owner}/{repo} the access check reads; the
// permissions block reflects the requesting token's user.
type repoAccess struct {
	Permissions struct {
		Push bool `json:"push"`
	} `json:"permissions"`
}

// checkRepoPush verifies the token's user can push to owner/repo. No push permission,
// or a repo the token cannot see at all (404), is errNoRepoPush.
func checkRepoPush(ctx context.Context, client *http.Client, apiURL, token, owner, repo string) error {
	repoURL := strings.TrimSuffix(apiURL, "/") + "/repos/" + owner + "/" + repo
	req, err := http.NewRequestWithContext(ctx, "GET", repoURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusNotFound:
		return fmt.Errorf("%w: %s/%s", errNoRepoPush, owner, repo)
	case resp.StatusCode != http.StatusOK:
		return githubRespError("repo access check", resp)
	}

	access := repoAccess{}
	if err := json.NewDecoder(resp.Body).Decode(&access); err != nil {
		return err
	}
	if !access.Permissions.Push {
		return fmt.Errorf("%w: %s/%s", errNoRepoPush, owner, repo)
	}
	return nil
}
