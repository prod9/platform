package srv

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"fx.prodigy9.co/config"
	"fx.prodigy9.co/httpserver/controllers"
	"fx.prodigy9.co/httpserver/httperrors"
	"fx.prodigy9.co/httpserver/render"
	"github.com/go-chi/chi/v5"
)

// ErrWebhookExists reports a repo that already carries the hook config GitHub was
// asked to create; GitHub answers a duplicate with a 422.
var ErrWebhookExists = errors.New("srv: flux webhook already configured")

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

	if _, err := currentUser(req); errors.Is(err, ErrNoSession) {
		render.Error(resp, req, 401, httperrors.ErrUnauthorized)
		return
	} else if err != nil {
		render.Error(resp, req, 500, err)
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
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<10))
		return fmt.Errorf("srv: creating flux webhook failed: %d %s: %s",
			resp.StatusCode, resp.Status, respBody)
	}
	return nil
}
