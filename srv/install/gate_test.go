package install

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"platform.prodigy9.co/srv/srvtest"
)

// A composed handler reads the durable claim on every request: committing the claim
// changes the exposed surface without rebuilding the handler or mutating local state
// (docs/spec/installation.md, "Boot composition — the application is permanent").
func TestGatesFollowDurableClaimAfterComposition(t *testing.T) {
	ctx := srvtest.SetupDB(t)
	product := ProductGate(nil)(statusHandler(http.StatusNoContent))
	installer := InstallerGate(nil)(statusHandler(http.StatusNoContent))

	require.Equal(t, http.StatusConflict, serve(ctx, product).Code)
	require.Equal(t, http.StatusNoContent, serve(ctx, installer).Code)

	claimInstalled(t, ctx)

	require.Equal(t, http.StatusNoContent, serve(ctx, product).Code)
	require.Equal(t, http.StatusNotFound, serve(ctx, installer).Code)
}

func statusHandler(status int) http.Handler {
	return http.HandlerFunc(func(resp http.ResponseWriter, _ *http.Request) {
		resp.WriteHeader(status)
	})
}

func serve(ctx context.Context, handler http.Handler) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req = req.WithContext(ctx)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	return resp
}
