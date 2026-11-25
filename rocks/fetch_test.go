package rocks

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

type entry struct {
	header tar.Header
	body   string
}

// serveRock stands up a registry serving one rock, prod9/<name>:<version>, as a one-layer
// image whose layer holds entries. It counts every request it answers and records the
// Authorization header the token endpoint received.
func serveRock(t *testing.T, name, version string, entries []entry) (*atomic.Int32, *string) {
	t.Helper()

	layer := tarGzip(t, entries)
	sum := sha256.Sum256(layer)
	digest := "sha256:" + hex.EncodeToString(sum[:])
	manifest, err := json.Marshal(ociManifest{Layers: []ociDescriptor{
		{MediaType: ociLayerType, Digest: digest},
	}})
	require.NoError(t, err)

	requests, tokenAuth := &atomic.Int32{}, new(string)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /token", func(w http.ResponseWriter, r *http.Request) {
		*tokenAuth = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"token":"pull-token"}`))
	})
	mux.HandleFunc("GET /v2/prod9/"+name+"/manifests/"+version, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(manifest)
	})
	mux.HandleFunc("GET /v2/prod9/"+name+"/blobs/"+digest, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(layer)
	})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		mux.ServeHTTP(w, r)
	}))
	t.Cleanup(server.Close)

	original := registryBase
	registryBase = server.URL
	t.Cleanup(func() { registryBase = original })
	return requests, tokenAuth
}

func tarGzip(t *testing.T, entries []entry) []byte {
	t.Helper()

	var buf bytes.Buffer
	zipped := gzip.NewWriter(&buf)
	archive := tar.NewWriter(zipped)
	for _, e := range entries {
		header := e.header
		header.Size = int64(len(e.body))
		require.NoError(t, archive.WriteHeader(&header))
		_, err := archive.Write([]byte(e.body))
		require.NoError(t, err)
	}
	require.NoError(t, archive.Close())
	require.NoError(t, zipped.Close())
	return buf.Bytes()
}

func file(name, body string) entry {
	return entry{header: tar.Header{Name: name, Typeflag: tar.TypeReg, Mode: 0o644}, body: body}
}

// A version in the cache is never fetched again (docs/spec/lua-render.md, "Cache").
func TestFetchCachesRock(t *testing.T) {
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	requests, tokenAuth := serveRock(t, "infra-lua", "v0.1.0", []entry{
		file("infra-lua-0.1.0-1.rockspec", "package = 'infra-lua'"),
		{header: tar.Header{Name: "templates/", Typeflag: tar.TypeDir, Mode: 0o755}},
		file("templates/gateway.lua", "return {}"),
	})
	fetch := Fetch{Name: "infra-lua", Version: "v0.1.0", Username: "bot", Password: "secret"}

	dir, err := fetch.Run(context.Background())
	require.NoError(t, err)
	require.Equal(t, filepath.Join(cache, "platform", "rocks", "infra-lua", "v0.1.0"), dir)
	gateway, err := os.ReadFile(filepath.Join(dir, "templates", "gateway.lua"))
	require.NoError(t, err)
	require.Equal(t, "return {}", string(gateway))
	require.Equal(t, "Basic Ym90OnNlY3JldA==", *tokenAuth)

	fetched := requests.Load()
	again, err := fetch.Run(context.Background())
	require.NoError(t, err)
	require.Equal(t, dir, again)
	require.Equal(t, fetched, requests.Load())
}

// An archive that would write outside the rock's directory, or anything but a directory
// or regular file, fails the fetch and leaves the cache as it was: no version directory
// and no half-unpacked temporary one.
func TestFetchRejectsUnsafeEntry(t *testing.T) {
	cases := map[string]entry{
		"escaping path": file("../escaped.lua", "return {}"),
		"symlink": {header: tar.Header{
			Name: "link.lua", Typeflag: tar.TypeSymlink, Linkname: "/etc/passwd", Mode: 0o777,
		}},
	}
	for name, unsafe := range cases {
		t.Run(name, func(t *testing.T) {
			cache := t.TempDir()
			t.Setenv("XDG_CACHE_HOME", cache)
			serveRock(t, "infra-lua", "v0.1.0", []entry{file("init.lua", "return {}"), unsafe})

			_, err := Fetch{Name: "infra-lua", Version: "v0.1.0"}.Run(context.Background())
			require.Error(t, err)

			parent := filepath.Join(cache, "platform", "rocks", "infra-lua")
			left, err := os.ReadDir(parent)
			require.NoError(t, err)
			require.Empty(t, left)
			require.NoFileExists(t, filepath.Join(parent, "escaped.lua"))
		})
	}
}
