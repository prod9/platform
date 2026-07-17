package github

import (
	"context"
	"encoding/json"
	"errors"
	"html/template"
	"net/http"
	"strings"

	"fx.prodigy9.co/config"
	"fx.prodigy9.co/httpserver/controllers"
	"fx.prodigy9.co/httpserver/render"
	"github.com/go-chi/chi/v5"
)

var (
	ServerURLConfig = config.Str("SERVER_URL")
	URLConfig       = config.StrDef("GITHUB_URL", "https://github.com")
	APIURLConfig    = config.StrDef("GITHUB_API_URL", "https://api.github.com")
)

// SetupCtr serves the GitHub App bootstrap pages under /setup/ (spec §Auth
// mechanism): srv generates an App Manifest, the operator submits it to GitHub, and
// GitHub redirects back with a one-time code that srv exchanges for the App
// credentials. Operator-facing pages served by srv directly — deliberately not part
// of the webui.
type SetupCtr struct{}

var _ controllers.Interface = SetupCtr{}

func (SetupCtr) Mount(cfg *config.Source, router chi.Router) error {
	router.Get("/setup/github", setupPage)
	router.Get("/setup/github/callback", setupCallback)
	return nil
}

func setupPage(resp http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	cfg := config.FromContext(ctx)

	serverURL, ok := config.GetOK(cfg, ServerURLConfig)
	if !ok {
		render.Error(resp, req, 500, errors.New("github: SERVER_URL must be set to run GitHub App setup"))
		return
	}
	serverURL = strings.TrimSuffix(serverURL, "/")

	app, err := LoadApp(ctx)
	switch {
	case err == nil:
		renderHTML(resp, req, "configured", app.Slug)
	case errors.Is(err, ErrNoApp):
		manifest, err := json.Marshal(appManifest(serverURL))
		if err != nil {
			render.Error(resp, req, 500, err)
			return
		}
		renderHTML(resp, req, "form", struct {
			GitHubURL string
			Manifest  string
		}{config.Get(cfg, URLConfig), string(manifest)})
	default:
		render.Error(resp, req, 500, err)
	}
}

func setupCallback(resp http.ResponseWriter, req *http.Request) {
	ctx := req.Context()
	cfg := config.FromContext(ctx)

	code := req.URL.Query().Get("code")
	if code == "" {
		render.Error(resp, req, 400, errors.New("github: missing code query parameter"))
		return
	}

	apiURL := config.Get(cfg, APIURLConfig)
	creds, err := exchangeManifest(ctx, http.DefaultClient, apiURL, code)
	if err != nil {
		render.Error(resp, req, 502, err)
		return
	}

	save := &SaveApp{
		AppID:         creds.ID,
		Slug:          creds.Slug,
		PrivateKey:    creds.PEM,
		WebhookSecret: creds.WebhookSecret,
		ClientID:      creds.ClientID,
		ClientSecret:  creds.ClientSecret,
	}
	if err := save.Execute(ctx, nil); errors.Is(err, ErrAppExists) {
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
type manifest struct {
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

func appManifest(serverURL string) manifest {
	return manifest{
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

// appCreds is the credential set GitHub returns from a manifest conversion.
type appCreds struct {
	ID            int64  `json:"id"`
	Slug          string `json:"slug"`
	PEM           string `json:"pem"`
	WebhookSecret string `json:"webhook_secret"`
	ClientID      string `json:"client_id"`
	ClientSecret  string `json:"client_secret"`
}

func exchangeManifest(ctx context.Context, client *http.Client, apiURL, code string) (*appCreds, error) {
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
		return nil, RespError("manifest conversion", resp)
	}

	creds := &appCreds{}
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
