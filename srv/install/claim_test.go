package install

import (
	"context"
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

// claimHarness is a claim-ready server: migrated DB, a fake GitHub whose membership
// answer is configurable, App credentials with a real signing key, and a logged-in
// user ("chakrit") whose session token the cookie carries.
type claimHarness struct {
	ctx    context.Context
	router chi.Router
	token  string
	userID int64
}

func setupClaim(t *testing.T, membershipStatus int, membershipBody string) *claimHarness {
	ctx := srvtest.SetupDB(t, Source, migrator.FromFS(auth.Migrations))

	mux := http.NewServeMux()
	mux.HandleFunc("POST /app/installations/7/access_tokens", func(resp http.ResponseWriter, req *http.Request) {
		resp.WriteHeader(201)
		fmt.Fprint(resp, `{"token":"ghs_tok"}`)
	})
	mux.HandleFunc("GET /app/installations/7", func(resp http.ResponseWriter, req *http.Request) {
		fmt.Fprint(resp, `{"id":7,"account":{"id":9,"login":"prodigy9","type":"Organization"}}`)
	})
	mux.HandleFunc("GET /orgs/prodigy9/memberships/chakrit", func(resp http.ResponseWriter, req *http.Request) {
		require.Equal(t, "Bearer ghs_tok", req.Header.Get("Authorization"))
		resp.WriteHeader(membershipStatus)
		fmt.Fprint(resp, membershipBody)
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

	return &claimHarness{ctx, router, "raw-session-token", userID}
}

func (h *claimHarness) claim(withCookie bool) *httptest.ResponseRecorder {
	req := httptest.NewRequest("POST", "/api/install/claim",
		strings.NewReader(`{"installation_id":7}`)).WithContext(h.ctx)
	if withCookie {
		req.AddCookie(&http.Cookie{Name: "platform_session", Value: h.token})
	}

	resp := httptest.NewRecorder()
	h.router.ServeHTTP(resp, req)
	return resp
}

func TestClaimWritesInstallRecord(t *testing.T) {
	h := setupClaim(t, 200, `{"role":"admin","state":"active"}`)

	resp := h.claim(true)
	require.Equal(t, http.StatusOK, resp.Code)

	record, err := Load(h.ctx)
	require.NoError(t, err)
	require.Equal(t, int64(9), record.OrgID)
	require.Equal(t, "prodigy9", record.OrgLogin)
	require.Equal(t, int64(7), record.InstallationID)
	require.Equal(t, h.userID, record.InstalledByUserID)
	require.Equal(t, "chakrit", record.InstalledByLogin)
	require.False(t, record.InstalledAt.IsZero())
}

func TestClaimByNonOwnerForbidden(t *testing.T) {
	h := setupClaim(t, 200, `{"role":"member","state":"active"}`)

	resp := h.claim(true)
	require.Equal(t, http.StatusForbidden, resp.Code)

	_, err := Load(h.ctx)
	require.ErrorIs(t, err, ErrNotInstalled)
}

func TestClaimWithoutSessionUnauthorized(t *testing.T) {
	h := setupClaim(t, 200, `{"role":"admin","state":"active"}`)

	resp := h.claim(false)
	require.Equal(t, http.StatusUnauthorized, resp.Code)

	_, err := Load(h.ctx)
	require.ErrorIs(t, err, ErrNotInstalled)
}

func TestClaimTwiceConflicts(t *testing.T) {
	h := setupClaim(t, 200, `{"role":"admin","state":"active"}`)

	require.Equal(t, http.StatusOK, h.claim(true).Code)
	require.Equal(t, http.StatusConflict, h.claim(true).Code)
}

// Two truly concurrent claims serialize on the FOR UPDATE row: exactly one fills the
// install.* settings, the other blocks on the lock and then finds the first one's write.
func TestClaimExecuteConcurrentOneWins(t *testing.T) {
	ctx := srvtest.SetupDB(t, Source)

	start := make(chan struct{})
	errs := make(chan error, 2)
	for i := range 2 {
		claim := &ClaimInstall{
			InstallationID: 7, OrgID: 9, OrgLogin: "prodigy9",
			UserID: int64(i + 1), UserLogin: fmt.Sprintf("user%d", i+1),
		}
		go func() {
			<-start
			errs <- claim.Execute(ctx, nil)
		}()
	}
	close(start)

	var wins, conflicts int
	for range 2 {
		err := <-errs
		if err == nil {
			wins++
		} else {
			require.ErrorIs(t, err, ErrAlreadyInstalled)
			conflicts++
		}
	}
	require.Equal(t, 1, wins)
	require.Equal(t, 1, conflicts)

	record, err := Load(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(7), record.InstallationID)
}

func TestClaimWithoutAppUnavailable(t *testing.T) {
	h := setupClaim(t, 200, `{"role":"admin","state":"active"}`)
	srvtest.StubApp(t, nil, github.ErrNoApp)

	resp := h.claim(true)
	require.Equal(t, http.StatusServiceUnavailable, resp.Code)

	_, err := Load(h.ctx)
	require.ErrorIs(t, err, ErrNotInstalled)
}
