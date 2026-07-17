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
	router, err := Router(fxtest.Configure())
	require.NoError(t, err)

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest("GET", "/", nil))

	require.Equal(t, http.StatusOK, resp.Code)
	require.Contains(t, resp.Body.String(), "platform")
}

func TestRouterServesAPIHealth(t *testing.T) {
	router, err := Router(fxtest.Configure())
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
