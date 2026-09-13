package engine

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"platform.prodigy9.co/conf"
)

func TestLocalPreparationFailureClosesItsOperation(t *testing.T) {
	input := Local{ConfigPath: filepath.Join(t.TempDir(), "missing.toml")}
	obs := &recorder{}
	sess := NewSession(t.Context())
	results, err := sess.Build(t.Context(), input, []string{"web"}, obs)
	require.Error(t, err)
	require.Empty(t, results)
	require.Equal(t, []string{"runstart web", "configstart web", "configdone web/", "rundone web//"}, obs.lines)
	require.Len(t, obs.errs, 2)
	require.ErrorIs(t, obs.errs[1], obs.errs[0])
	require.NoError(t, sess.Close())
}

func TestSelectionPreflightDoesNotInventModuleOperations(t *testing.T) {
	for _, input := range []Input{Source{URL: "file:///not-read"}, Local{ConfigPath: filepath.Join(t.TempDir(), "missing.toml")}} {
		obs := &recorder{}
		results, err := NewSession(t.Context()).Build(t.Context(), input, nil, obs)
		require.Error(t, err)
		require.Empty(t, results)
		require.Empty(t, obs.lines)
	}
}

func TestRemoteCloneFailureClosesBeforeConfiguration(t *testing.T) {
	obs := &recorder{}
	sess := NewSession(t.Context())
	results, err := sess.Build(t.Context(), Source{}, []string{"web"}, obs)
	require.ErrorContains(t, err, "checkout URL")
	require.Empty(t, results)
	require.Equal(t, []string{"runstart web", "clonestart web", "clonedone web", "rundone web//"}, obs.lines)
	require.NoError(t, sess.Close())
}

func TestRemoteConfigIsExactAndWorktreeLivesUntilSessionClose(t *testing.T) {
	previousFilename := conf.PlatformFilename
	conf.PlatformFilename = "alternate.toml"
	t.Cleanup(func() { conf.PlatformFilename = previousFilename })
	remote, sha := initCheckoutRemote(t)
	cache := t.TempDir()
	t.Setenv("CHECKOUT_CACHE", cache)
	require.NoError(t, os.WriteFile(filepath.Join(cache, "platform.toml"), []byte("[modules.web]\nframework='go/basic'\n"), 0o644))
	sess := NewSession(t.Context())
	obs := &recorder{}
	results, err := sess.Build(t.Context(), Source{URL: "file://" + remote, Revision: sha}, []string{"web"}, obs)
	require.ErrorIs(t, err, os.ErrNotExist)
	require.ErrorContains(t, err, "platform.toml")
	require.Empty(t, results)
	require.Equal(t, []string{"runstart web", "clonestart web", "clonedone web", "configstart web", "configdone web/", "rundone web//"}, obs.lines)
	require.Len(t, sess.worktrees, 1)
	path := sess.worktrees[0].Dir
	require.DirExists(t, path)
	require.NoError(t, sess.Close())
	require.NoDirExists(t, path)
}

func TestPublishCredentialsFailConfigurationBeforeDial(t *testing.T) {
	for _, test := range []struct{ name, registry, password, want string }{
		{"missing password", "ghcr.io", "", "registry password is required"},
		{"different registry", "example.com", "token", "does not match image"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("REGISTRY", test.registry)
			t.Setenv("REGISTRY_USERNAME", "installation")
			t.Setenv("REGISTRY_PASSWORD", test.password)
			obs := &recorder{}
			sess := NewSession(rosterCtx())
			results, err := sess.BuildAndPublish(t.Context(), localModuleConfig(t), []string{"web"}, "v1", obs)
			require.ErrorContains(t, err, test.want)
			require.Empty(t, results)
			require.Equal(t, []string{"runstart web", "configstart web", "configdone web/", "rundone web//"}, obs.lines)
			require.NoError(t, sess.Close())
		})
	}
}

func TestBuildIgnoresPublishCredentialsAndAttributesDialFailureToConfig(t *testing.T) {
	t.Setenv("REGISTRY_USERNAME", "installation")
	t.Setenv("REGISTRY_PASSWORD", "")
	t.Setenv("DAGGER_ENGINE", "engine.invalid")
	dialErr := errors.New("engine discovery failed")
	stubLookup(t, func(context.Context, string) ([]string, error) { return nil, dialErr })
	obs := &recorder{}
	sess := NewSession(rosterCtx())
	results, err := sess.Build(t.Context(), localModuleConfig(t), []string{"web"}, obs)
	require.ErrorIs(t, err, dialErr)
	require.Empty(t, results)
	require.Equal(t, []string{"runstart web", "configstart web", "configdone web/", "rundone web//"}, obs.lines)
	require.NoError(t, sess.Close())
}

func TestSelectedModuleFailuresAreJoined(t *testing.T) {
	sess := NewSession(t.Context())
	results, err := sess.Build(t.Context(), localModuleConfig(t), []string{"missing-first", "missing-second"}, nil)
	require.Empty(t, results)
	require.ErrorContains(t, err, "missing-first")
	require.ErrorContains(t, err, "missing-second")
	require.NoError(t, sess.Close())
}

func localModuleConfig(t *testing.T) Local {
	t.Helper()
	path := filepath.Join(t.TempDir(), "platform.toml")
	require.NoError(t, os.WriteFile(path, []byte("repository='github.com/prod9/app'\n[modules.web]\nframework='go/basic'\n"), 0o644))
	return Local{ConfigPath: path}
}
