package srv

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"fx.prodigy9.co/fxtest"
	"github.com/stretchr/testify/require"
)

func TestRouterServesUIIndex(t *testing.T) {
	for _, installed := range []bool{false, true} {
		router, err := Router(fxtest.Configure(), nil, installed)
		require.NoError(t, err)

		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, httptest.NewRequest("GET", "/", nil))

		require.Equal(t, http.StatusOK, resp.Code)
		require.Contains(t, resp.Body.String(), "platform")
	}
}

func TestRouterServesAPIHealth(t *testing.T) {
	for _, installed := range []bool{false, true} {
		router, err := Router(fxtest.Configure(), nil, installed)
		require.NoError(t, err)

		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, httptest.NewRequest("GET", "/health", nil))

		require.Equal(t, http.StatusOK, resp.Code)
		require.Equal(t, "application/json", resp.Header().Get("Content-Type"))

		var health struct {
			Time time.Time `json:"time"`
		}
		require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &health))
		require.False(t, health.Time.IsZero())
	}
}

// The not-installed composition mounts the installer surface and no product /api/*;
// the installed composition is the reverse. The installer state read works with a nil
// DB (it reports db-reachable as an error), so this needs no postgres.
func TestNotInstalledMountsInstallerNotProduct(t *testing.T) {
	router, err := Router(fxtest.Configure(), nil, false)
	require.NoError(t, err)

	require.Equal(t, http.StatusOK, get(router, "/api/install").Code)
	require.Equal(t, http.StatusNotFound, get(router, "/api/builds").Code)
}

func TestInstalledMountsProductNotInstaller(t *testing.T) {
	router, err := Router(fxtest.Configure(), nil, true)
	require.NoError(t, err)

	require.Equal(t, http.StatusNotFound, get(router, "/api/install").Code)
	require.NotEqual(t, http.StatusNotFound, get(router, "/api/builds").Code)
}

func get(router http.Handler, path string) *httptest.ResponseRecorder {
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest("GET", path, nil))
	return resp
}
