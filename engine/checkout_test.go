package engine

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCheckoutRootUsesEngineConfig(t *testing.T) {
	t.Setenv("CHECKOUT_CACHE", "/configured/cache")

	root, err := checkoutRoot(t.Context())
	require.NoError(t, err)
	require.Equal(t, "/configured/cache", root)
}

func TestCheckoutRootDefaultsToUserCache(t *testing.T) {
	t.Setenv("CHECKOUT_CACHE", "")

	previous := userCacheDir
	userCacheDir = func() (string, error) { return "/user/cache", nil }
	t.Cleanup(func() { userCacheDir = previous })

	root, err := checkoutRoot(t.Context())
	require.NoError(t, err)
	require.Equal(t, "/user/cache/platform", root)
}

func TestCheckoutMaterializesFullWorktree(t *testing.T) {
	remote, sha := initCheckoutRemote(t)
	cache := t.TempDir()

	worktree, err := checkoutAt(t.Context(), cache, Source{
		URL: "file://" + remote, Revision: sha,
	})
	require.NoError(t, err)
	require.Equal(t, sha, worktree.SHA)
	require.DirExists(t, worktree.Dir)

	content, err := os.ReadFile(filepath.Join(worktree.Dir, "hello.txt"))
	require.NoError(t, err)
	require.Equal(t, "hello v2", string(content))
	require.Equal(t,
		checkoutGit(t, remote, "rev-list", "--count", "HEAD"),
		checkoutGit(t, worktree.Dir, "rev-list", "--count", "HEAD"),
		"checkout history must be full, never shallow")
}

func TestCheckoutFetchesThroughExistingMirror(t *testing.T) {
	remote, firstSHA := initCheckoutRemote(t)
	cache := t.TempDir()

	first, err := checkoutAt(t.Context(), cache, Source{
		URL: "file://" + remote, Revision: firstSHA,
	})
	require.NoError(t, err)
	require.NoError(t, first.Close(t.Context()))

	secondSHA := commitCheckoutFile(t, remote, "hello.txt", "hello v3", "third")
	second, err := checkoutAt(t.Context(), cache, Source{
		URL: "file://" + remote, Revision: secondSHA,
	})
	require.NoError(t, err)
	require.Equal(t, secondSHA, second.SHA)

	content, err := os.ReadFile(filepath.Join(second.Dir, "hello.txt"))
	require.NoError(t, err)
	require.Equal(t, "hello v3", string(content))
}

func TestCheckoutResolvesRevisionAndOwnsCleanup(t *testing.T) {
	remote, sha := initCheckoutRemote(t)
	cache := t.TempDir()

	worktree, err := checkoutAt(t.Context(), cache, Source{
		URL: "file://" + remote, Revision: sha[:8],
	})
	require.NoError(t, err)
	require.Equal(t, sha, worktree.SHA)
	require.NoError(t, worktree.Close(t.Context()))
	require.NoDirExists(t, worktree.Dir)
}

func TestCheckoutCreatesIndependentWorktrees(t *testing.T) {
	remote, sha := initCheckoutRemote(t)
	cache := t.TempDir()

	first, err := checkoutAt(t.Context(), cache, Source{
		URL: "file://" + remote, Revision: sha,
	})
	require.NoError(t, err)
	second, err := checkoutAt(t.Context(), cache, Source{
		URL: "file://" + remote, Revision: sha,
	})
	require.NoError(t, err)

	require.NotEqual(t, first.Dir, second.Dir)
	require.DirExists(t, first.Dir)
	require.DirExists(t, second.Dir)
}

func TestCheckoutSerializesConcurrentMirrorFetches(t *testing.T) {
	remote, sha := initCheckoutRemote(t)
	cache := t.TempDir()

	type result struct {
		worktree *Worktree
		err      error
	}
	results := make(chan result, 2)
	for range 2 {
		go func() {
			worktree, err := checkoutAt(t.Context(), cache, Source{
				URL: "file://" + remote, Revision: sha,
			})
			results <- result{worktree: worktree, err: err}
		}()
	}

	for range 2 {
		result := <-results
		require.NoError(t, result.err)
		require.DirExists(t, result.worktree.Dir)
	}
}

func TestCheckoutStoredRemoteStaysCredentialFree(t *testing.T) {
	remote, sha := initCheckoutRemote(t)
	cache := t.TempDir()

	worktree, err := checkoutAt(t.Context(), cache, Source{
		URL: "file://" + remote, Revision: sha,
		Username: "x-access-token", Password: "tok123",
	})
	require.NoError(t, err)

	gitDir := checkoutGit(t, worktree.Dir, "rev-parse", "--git-common-dir")
	stored := checkoutGit(t, gitDir, "config", "remote.origin.url")
	require.Equal(t, "file://"+remote, stored)
	require.NotContains(t, checkoutGit(t, gitDir, "config", "--list"), "tok123")
}

func TestCheckoutFetchURLInjectsCredentialsForHTTPOnly(t *testing.T) {
	cases := []struct {
		source Source
		want   string
	}{
		{Source{URL: "https://github.com/prod9/app.git", Username: "bot", Password: "tok"},
			"https://bot:tok@github.com/prod9/app.git"},
		{Source{URL: "file:///srv/repos/app", Username: "bot", Password: "tok"},
			"file:///srv/repos/app"},
		{Source{URL: "https://github.com/prod9/app.git"},
			"https://github.com/prod9/app.git"},
	}
	for _, test := range cases {
		got, err := checkoutFetchURL(test.source)
		require.NoError(t, err)
		require.Equal(t, test.want, got)
	}
}

func TestCheckoutFetchURLRejectsCredentialedSourceURL(t *testing.T) {
	_, err := checkoutFetchURL(Source{URL: "https://leak@github.com/prod9/app.git"})
	require.ErrorContains(t, err, "credential-free")
}

func initCheckoutRemote(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()

	checkoutGit(t, dir, "init", "-q", "-b", "main")
	commitCheckoutFile(t, dir, "hello.txt", "hello v1", "first")
	sha := commitCheckoutFile(t, dir, "hello.txt", "hello v2", "second")
	return dir, sha
}

func commitCheckoutFile(t *testing.T, dir, name, content, message string) string {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644))
	checkoutGit(t, dir, "add", ".")
	checkoutGit(t, dir,
		"-c", "user.name=platform", "-c", "user.email=platform@test",
		"commit", "-q", "-m", message)
	return checkoutGit(t, dir, "rev-parse", "HEAD")
}

func checkoutGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git %v: %s", args, out)
	return strings.TrimSpace(string(out))
}
