package install

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"fx.prodigy9.co/config"
	"fx.prodigy9.co/data"
	"fx.prodigy9.co/data/migrator"
	"fx.prodigy9.co/fxtest"
	"fx.prodigy9.co/httpserver/middlewares"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"
	"platform.prodigy9.co/srv/auth"
	"platform.prodigy9.co/srv/github"
	"platform.prodigy9.co/srv/migrate"
	"platform.prodigy9.co/srv/srvtest"
)

// fluxHarness is a flux-setup-ready server: migrated DB with the claim already
// written, a fake GitHub recording org-webhook calls, and a logged-in user.
type fluxHarness struct {
	ctx     context.Context
	router  chi.Router
	token   string
	created *map[string]any
}

func setupFlux(t *testing.T, existingHooks string) *fluxHarness {
	ctx := srvtest.SetupDB(t, Source, migrator.FromFS(auth.Migrations))

	var created map[string]any
	mux := http.NewServeMux()
	mux.HandleFunc("POST /app/installations/7/access_tokens", func(resp http.ResponseWriter, req *http.Request) {
		resp.WriteHeader(201)
		fmt.Fprint(resp, `{"token":"ghs_tok"}`)
	})
	mux.HandleFunc("GET /orgs/prodigy9/hooks", func(resp http.ResponseWriter, req *http.Request) {
		require.Equal(t, "Bearer ghs_tok", req.Header.Get("Authorization"))
		fmt.Fprint(resp, existingHooks)
	})
	mux.HandleFunc("POST /orgs/prodigy9/hooks", func(resp http.ResponseWriter, req *http.Request) {
		require.NoError(t, json.NewDecoder(req.Body).Decode(&created))
		resp.WriteHeader(201)
		fmt.Fprint(resp, `{"id":2}`)
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	srvtest.StubApp(t, srvtest.TestApp(t), nil)

	cfg := fxtest.Configure()
	config.Set(cfg, github.APIURLConfig, server.URL)
	router := chi.NewRouter()
	router.Use(middlewares.Configure(cfg))
	db := data.FromContext(ctx)
	ctr := StateCtr{DB: db, Merged: migrate.Merged(Source)}
	require.NoError(t, ctr.Mount(cfg, router))

	var userID int64
	require.NoError(t, data.Get(ctx, &userID,
		`INSERT INTO users (name) VALUES ('chakrit') RETURNING id`))
	create := &auth.CreateSession{
		UserID: userID, Token: "raw-session-token",
		ExpiresAt: time.Now().Add(time.Hour),
	}
	require.NoError(t, create.Execute(ctx, nil))

	claim := &ClaimInstall{InstallationID: 7, UserID: userID, UserLogin: "chakrit",
		OrgID: 9, OrgLogin: "prodigy9"}
	require.NoError(t, claim.Execute(ctx, nil))

	return &fluxHarness{ctx, router, "raw-session-token", &created}
}

func (h *fluxHarness) post(withCookie bool, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("POST", "/api/install/flux",
		strings.NewReader(body)).WithContext(h.ctx)
	if withCookie {
		req.AddCookie(&http.Cookie{Name: "platform_session", Value: h.token})
	}
	resp := httptest.NewRecorder()
	h.router.ServeHTTP(resp, req)
	return resp
}

// The step creates the org webhook through the App and saves the receiver URL; the
// state list comes back with flux-setup fully ready
// (docs/spec/installation.md, the flux-setup step).
func TestSetupFluxCreatesWebhookAndSavesURL(t *testing.T) {
	h := setupFlux(t, `[]`)

	resp := h.post(true, `{"receiver_url":"https://flux.example/hook/abc"}`)
	require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())

	require.Equal(t, "web", (*h.created)["name"])
	require.Equal(t, []any{"registry_package"}, (*h.created)["events"])

	url, err := github.LoadSetting(h.ctx, "flux.receiver_url", errNoReceiver)
	require.NoError(t, err)
	require.Equal(t, "https://flux.example/hook/abc", url)

	var entries []Entry
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &entries))
	require.Equal(t, "flux-setup", entries[4].Name)
}

// A hook already targeting the URL is converged, not duplicated.
func TestSetupFluxAlreadyWired(t *testing.T) {
	h := setupFlux(t,
		`[{"id":1,"config":{"url":"https://flux.example/hook/abc"},"events":["registry_package"],"active":true}]`)

	resp := h.post(true, `{"receiver_url":"https://flux.example/hook/abc"}`)
	require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())
	require.Nil(t, *h.created)
}

// The action mutates the org through the App, so it carries the claim's session gate.
func TestSetupFluxRequiresSession(t *testing.T) {
	h := setupFlux(t, `[]`)

	resp := h.post(false, `{"receiver_url":"https://flux.example/hook/abc"}`)
	require.Equal(t, http.StatusUnauthorized, resp.Code)
}

// A relative or non-https value is the caller's error.
func TestSetupFluxRejectsBadURL(t *testing.T) {
	h := setupFlux(t, `[]`)

	for _, bad := range []string{"", "not-a-url", "http://flux.example/hook"} {
		resp := h.post(true, fmt.Sprintf(`{"receiver_url":%q}`, bad))
		require.Equal(t, http.StatusBadRequest, resp.Code, "url %q", bad)
	}
}
