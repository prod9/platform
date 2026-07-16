package srv

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"strings"

	"fx.prodigy9.co/config"
	"fx.prodigy9.co/httpserver/controllers"
	"fx.prodigy9.co/httpserver/render"
	"github.com/go-chi/chi/v5"
)

var (
	ServerURLConfig    = config.Str("SERVER_URL")
	GitHubURLConfig    = config.StrDef("GITHUB_URL", "https://github.com")
	GitHubAPIURLConfig = config.StrDef("GITHUB_API_URL", "https://api.github.com")
)

// Setup serves the GitHub App bootstrap pages under /setup/ (spec §Auth mechanism):
// srv generates an App Manifest, the operator submits it to GitHub, and GitHub
// redirects back with a one-time code that srv exchanges for the App credentials.
// Operator-facing pages served by srv directly — deliberately not part of the webui.
type Setup struct{}

var _ controllers.Interface = Setup{}

func (Setup) Mount(cfg *config.Source, router chi.Router) error {
	router.Get("/setup/github", githubSetupPage)
	router.Get("/setup/github/callback", githubSetupCallback)
	return nil
}

// loadGitHubApp seams LoadGitHubApp so router tests run without postgres.
var loadGitHubApp = LoadGitHubApp

func githubSetupPage(resp http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	cfg := config.FromContext(ctx)

	serverURL, ok := config.GetOK(cfg, ServerURLConfig)
	if !ok {
		render.Error(resp, req, 500, errors.New("srv: SERVER_URL must be set to run GitHub App setup"))
		return
	}
	serverURL = strings.TrimSuffix(serverURL, "/")

	app, err := loadGitHubApp(ctx)
	switch {
	case err == nil:
		renderHTML(resp, req, "configured", app.Slug)
	case errors.Is(err, ErrNoGitHubApp):
		manifest, err := json.Marshal(githubAppManifest(serverURL))
		if err != nil {
			render.Error(resp, req, 500, err)
			return
		}
		renderHTML(resp, req, "form", struct {
			GitHubURL string
			Manifest  string
		}{config.Get(cfg, GitHubURLConfig), string(manifest)})
	default:
		render.Error(resp, req, 500, err)
	}
}

func githubSetupCallback(resp http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	cfg := config.FromContext(ctx)

	code := req.URL.Query().Get("code")
	if code == "" {
		render.Error(resp, req, 400, errors.New("srv: missing code query parameter"))
		return
	}

	apiURL := config.Get(cfg, GitHubAPIURLConfig)
	creds, err := exchangeManifest(ctx, http.DefaultClient, apiURL, code)
	if err != nil {
		render.Error(resp, req, 502, err)
		return
	}

	save := &SaveGitHubApp{
		AppID:         creds.ID,
		Slug:          creds.Slug,
		PrivateKey:    creds.PEM,
		WebhookSecret: creds.WebhookSecret,
		ClientID:      creds.ClientID,
		ClientSecret:  creds.ClientSecret,
	}
	if err := save.Execute(ctx, nil); errors.Is(err, ErrGitHubAppExists) {
		render.Error(resp, req, 409, err)
		return
	} else if err != nil {
		render.Error(resp, req, 500, err)
		return
	}

	renderHTML(resp, req, "created", creds.Slug)
}

// manifest content per the spec: contents:rw + metadata:r permissions, push +
// registry_package webhooks, callback back to this server.
type githubManifest struct {
	Name               string            `json:"name"`
	URL                string            `json:"url"`
	HookAttributes     hookAttributes    `json:"hook_attributes"`
	RedirectURL        string            `json:"redirect_url"`
	CallbackURLs       []string          `json:"callback_urls"`
	Public             bool              `json:"public"`
	DefaultPermissions map[string]string `json:"default_permissions"`
	DefaultEvents      []string          `json:"default_events"`
}

type hookAttributes struct {
	URL string `json:"url"`
}

func githubAppManifest(serverURL string) githubManifest {
	return githubManifest{
		Name:               "platform",
		URL:                serverURL,
		HookAttributes:     hookAttributes{URL: serverURL + "/api/webhooks/github"},
		RedirectURL:        serverURL + "/setup/github/callback",
		CallbackURLs:       []string{serverURL + "/api/auth/github/callback"},
		Public:             false,
		DefaultPermissions: map[string]string{"contents": "write", "metadata": "read"},
		DefaultEvents:      []string{"push", "registry_package"},
	}
}

// githubAppCreds is the credential set GitHub returns from a manifest conversion.
type githubAppCreds struct {
	ID            int64  `json:"id"`
	Slug          string `json:"slug"`
	PEM           string `json:"pem"`
	WebhookSecret string `json:"webhook_secret"`
	ClientID      string `json:"client_id"`
	ClientSecret  string `json:"client_secret"`
}

func exchangeManifest(ctx context.Context, client *http.Client, apiURL, code string) (*githubAppCreds, error) {
	url := strings.TrimSuffix(apiURL, "/") + "/app-manifests/" + code + "/conversions"
	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<10))
		return nil, fmt.Errorf("srv: manifest conversion failed: %d %s: %s",
			resp.StatusCode, resp.Status, body)
	}

	creds := &githubAppCreds{}
	if err := json.NewDecoder(resp.Body).Decode(creds); err != nil {
		return nil, err
	}
	return creds, nil
}

var setupPages = template.Must(template.New("setup").Parse(`
{{define "form" -}}
<!doctype html>
<title>platform: GitHub App setup</title>
<p>This server has no GitHub App yet. Submitting this form takes you to GitHub to
create one from the manifest below; GitHub then redirects back here with the App's
credentials.</p>
<form id="manifest-form" action="{{.GitHubURL}}/settings/apps/new" method="post">
	<textarea name="manifest" rows="12" cols="80" readonly>{{.Manifest}}</textarea>
	<br>
	<button type="submit">Create GitHub App</button>
</form>
{{- end}}

{{define "configured" -}}
<!doctype html>
<title>platform: GitHub App setup</title>
<p>GitHub App already configured (slug: {{.}}). Nothing to do.</p>
{{- end}}

{{define "created" -}}
<!doctype html>
<title>platform: GitHub App setup</title>
<p>GitHub App created (slug: {{.}}). Credentials stored.</p>
{{- end}}
`))

func renderHTML(resp http.ResponseWriter, req *http.Request, page string, data any) {
	resp.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := setupPages.ExecuteTemplate(resp, page, data); err != nil {
		render.Error(resp, req, 500, err)
	}
}
