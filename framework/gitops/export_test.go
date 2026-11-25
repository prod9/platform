package gitops

import (
	"net/http"
	"testing"
)

// UseDownloadClient points platform.download at client for the rest of the test, so a
// test can trust an httptest TLS server.
func UseDownloadClient(t *testing.T, client *http.Client) {
	previous := downloadClient
	downloadClient = client
	t.Cleanup(func() { downloadClient = previous })
}
