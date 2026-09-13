package engine

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dagger.io/dagger"
	"github.com/stretchr/testify/require"
)

// publicationConn answers only the registry operation; all container evaluation is stubbed.
// The SDK's public WithConn surface keeps this fixture independent of a running engine.
type publicationConn struct {
	err      error
	requests int
	closed   bool
}

func (c *publicationConn) Do(*http.Request) (*http.Response, error) {
	c.requests++
	if c.err != nil {
		return nil, c.err
	}

	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"data":{"host":{"directory":{"dockerBuild":{"publish":"ghcr.io/prod9/app@sha256:abc"}}}}}`)),
	}, nil
}

func (*publicationConn) Host() string { return "engine.test" }

func (c *publicationConn) Close() error {
	c.closed = true
	return nil
}

func TestModulePublicationCompletesBeforeReturningItsOutcome(t *testing.T) {
	for _, test := range []struct {
		name       string
		publishErr error
		wantHash   string
	}{
		{name: "success", wantHash: "ghcr.io/prod9/app@sha256:abc"},
		{name: "registry failure", publishErr: errors.New("registry rejected push")},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("REGISTRY_USERNAME", "")
			t.Setenv("DAGGER_ENGINE", "")
			conn := &publicationConn{err: test.publishErr}
			previousConnect := connectEngine
			connectEngine = func(ctx context.Context, _ ...dagger.ClientOpt) (*dagger.Client, error) {
				return dagger.Connect(ctx, dagger.WithConn(conn))
			}
			t.Cleanup(func() { connectEngine = previousConnect })
			stubContainerEvaluation(t)
			path := filepath.Join(t.TempDir(), "platform.toml")
			require.NoError(t, os.WriteFile(path, []byte("repository='github.com/prod9/app'\n[modules.web]\nframework='dockerfile'\n"), 0o644))
			obs := &recorder{}
			sess := NewSession(rosterCtx())

			results, err := sess.BuildAndPublish(t.Context(), Local{ConfigPath: path}, []string{"web"}, "v1", obs)

			require.Len(t, results, 1)
			result := results[0]
			require.Equal(t, "ghcr.io/prod9/app:v1", result.Image())
			require.Equal(t, test.wantHash, result.Hash())
			require.Equal(t, []string{
				"runstart web", "configstart web", "configdone web/local",
				"started web/build", "done web/build", "publishstart web", "publishdone web",
				"rundone web/ghcr.io/prod9/app:v1/" + test.wantHash,
			}, obs.lines)
			require.Equal(t, 1, conn.requests)
			require.False(t, conn.closed, "returned images remain session-owned")
			if test.publishErr == nil {
				require.NoError(t, err)
				require.NoError(t, result.Err)
				require.NotNil(t, result.UnsafeContainer())
			} else {
				require.ErrorContains(t, err, test.publishErr.Error())
				require.Equal(t, result.Err, obs.errs[2], "PublishDone carries the operation's failure")
				require.Equal(t, result.Err, obs.errs[3], "RunDone closes with the same failure")
				require.Nil(t, result.UnsafeContainer())
			}
			require.NoError(t, sess.Close())
			require.True(t, conn.closed)
		})
	}
}
