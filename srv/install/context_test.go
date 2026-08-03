package install

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"fx.prodigy9.co/config"
	"github.com/stretchr/testify/require"
	"platform.prodigy9.co/srv/github"
	"platform.prodigy9.co/srv/srvtest"
)

func TestTokenMintsFromContextRecord(t *testing.T) {
	ctx := setupToken(t, context.Background())
	ctx = NewContext(ctx, &Record{InstallationID: 7})

	token, client, err := Token(ctx)
	require.NoError(t, err)
	require.Equal(t, "ghs_ctx_tok", token)
	require.NotNil(t, client)
}

func TestTokenFallsBackToLoad(t *testing.T) {
	ctx := srvtest.SetupDB(t, Source)
	claimInstalled(t, ctx)

	token, _, err := Token(setupToken(t, ctx))
	require.NoError(t, err)
	require.Equal(t, "ghs_ctx_tok", token)
}

func TestTokenFailsWhenNotInstalled(t *testing.T) {
	ctx := srvtest.SetupDB(t, Source)

	_, _, err := Token(setupToken(t, ctx))
	require.ErrorIs(t, err, ErrNotInstalled)
}

func TestRecordContextSeedsRequests(t *testing.T) {
	ctx := srvtest.SetupDB(t, Source)
	claimInstalled(t, ctx)

	handler := RecordContext(http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		record, ok := FromContext(req.Context())
		require.True(t, ok)
		require.Equal(t, int64(7), record.InstallationID)
		resp.WriteHeader(204)
	}))

	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, httptest.NewRequest("GET", "/", nil).WithContext(ctx))
	require.Equal(t, 204, resp.Code)
}

func TestRecordContextFailsClosedWithoutDataContext(t *testing.T) {
	handler := RecordContext(http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		t.Error("handler must not run without a data context")
	}))

	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, httptest.NewRequest("GET", "/", nil))
	require.Equal(t, 500, resp.Code)
}

func TestRecordContextFailsClosedWhenNotInstalled(t *testing.T) {
	ctx := srvtest.SetupDB(t, Source)

	handler := RecordContext(http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		t.Error("handler must not run without an install record")
	}))

	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, httptest.NewRequest("GET", "/", nil).WithContext(ctx))
	require.Equal(t, 500, resp.Code)
}

// claimInstalled fills the install.* settings through the claim's own writer — tests never
// hand-write install state the product path could not have produced. (Not an srvtest
// helper: srvtest would have to import install, and this package's internal tests
// import srvtest — the test binary would cycle.)
func claimInstalled(t *testing.T, ctx context.Context) {
	claim := &ClaimInstall{
		InstallationID: 7,
		OrgID:          9, OrgLogin: "prodigy9",
		UserID: 1, UserLogin: "chakrit",
	}
	require.NoError(t, claim.Execute(ctx, nil))
}

// setupToken provides Token's two collaborators: stubbed App credentials and a fake
// GitHub minting tokens for installation 7, wired into ctx config.
func setupToken(t *testing.T, ctx context.Context) context.Context {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /app/installations/7/access_tokens", func(resp http.ResponseWriter, req *http.Request) {
		resp.WriteHeader(201)
		fmt.Fprint(resp, `{"token":"ghs_ctx_tok"}`)
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	srvtest.StubApp(t, srvtest.TestApp(t), nil)

	cfg := config.Configure()
	config.Set(cfg, github.APIURLConfig, server.URL)
	return config.NewContext(ctx, cfg)
}
