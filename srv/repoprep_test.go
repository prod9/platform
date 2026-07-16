package srv

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPrepRepoClonesAndCreatesWorktree(t *testing.T) {
	remote, sha := initTestRemote(t)
	cache := t.TempDir()

	prep := &PrepRepo{
		CacheDir: cache, CloneURL: "file://" + remote,
		Owner: "prod9", Repo: "app", SHA: sha, BuildID: 1,
	}
	workDir, resolved, err := prep.Run(context.Background())
	require.NoError(t, err)

	require.Equal(t, sha, resolved)
	require.Equal(t, filepath.Join(cache, "work", "1"), workDir)
	require.DirExists(t, filepath.Join(cache, "git", "prod9", "app.git"))

	content, err := os.ReadFile(filepath.Join(workDir, "hello.txt"))
	require.NoError(t, err)
	require.Equal(t, "hello v2", string(content))

	remoteCount := testGit(t, remote, "rev-list", "--count", "HEAD")
	workCount := testGit(t, workDir, "rev-list", "--count", "HEAD")
	require.Equal(t, remoteCount, workCount, "worktree history must be full, never shallow")
}

func TestPrepRepoFetchesExistingMirror(t *testing.T) {
	remote, sha1 := initTestRemote(t)
	cache := t.TempDir()

	first := &PrepRepo{
		CacheDir: cache, CloneURL: "file://" + remote,
		Owner: "prod9", Repo: "app", SHA: sha1, BuildID: 1,
	}
	_, _, err := first.Run(context.Background())
	require.NoError(t, err)

	mirror := filepath.Join(cache, "git", "prod9", "app.git")
	marker := filepath.Join(mirror, "test-marker")
	require.NoError(t, os.WriteFile(marker, []byte("x"), 0o644))

	sha2 := commitTestFile(t, remote, "hello.txt", "hello v3", "third")
	second := &PrepRepo{
		CacheDir: cache, CloneURL: "file://" + remote,
		Owner: "prod9", Repo: "app", SHA: sha2, BuildID: 2,
	}
	workDir, resolved, err := second.Run(context.Background())
	require.NoError(t, err)

	require.Equal(t, sha2, resolved)
	require.FileExists(t, marker, "second prep must fetch into the existing mirror, not re-clone")

	content, err := os.ReadFile(filepath.Join(workDir, "hello.txt"))
	require.NoError(t, err)
	require.Equal(t, "hello v3", string(content))
}

func TestPrepRepoResolvesAbbreviatedSHA(t *testing.T) {
	remote, sha := initTestRemote(t)
	cache := t.TempDir()

	prep := &PrepRepo{
		CacheDir: cache, CloneURL: "file://" + remote,
		Owner: "prod9", Repo: "app", SHA: sha[:8], BuildID: 1,
	}
	_, resolved, err := prep.Run(context.Background())
	require.NoError(t, err)
	require.Equal(t, sha, resolved)
}

func TestRemoveWorkTree(t *testing.T) {
	remote, sha := initTestRemote(t)
	cache := t.TempDir()

	prep := &PrepRepo{
		CacheDir: cache, CloneURL: "file://" + remote,
		Owner: "prod9", Repo: "app", SHA: sha, BuildID: 7,
	}
	workDir, _, err := prep.Run(context.Background())
	require.NoError(t, err)
	require.DirExists(t, workDir)

	remove := &RemoveWorkTree{CacheDir: cache, Owner: "prod9", Repo: "app", BuildID: 7}
	require.NoError(t, remove.Run(context.Background()))
	require.NoDirExists(t, workDir)
}

func TestPrepRepoConcurrent(t *testing.T) {
	remote, sha := initTestRemote(t)
	cache := t.TempDir()

	type result struct {
		workDir string
		err     error
	}
	results := make(chan result, 2)
	for buildID := int64(1); buildID <= 2; buildID++ {
		go func(buildID int64) {
			prep := &PrepRepo{
				CacheDir: cache, CloneURL: "file://" + remote,
				Owner: "prod9", Repo: "app", SHA: sha, BuildID: buildID,
			}
			workDir, _, err := prep.Run(context.Background())
			results <- result{workDir, err}
		}(buildID)
	}

	for range 2 {
		res := <-results
		require.NoError(t, res.err)

		content, err := os.ReadFile(filepath.Join(res.workDir, "hello.txt"))
		require.NoError(t, err)
		require.Equal(t, "hello v2", string(content))
	}
}

// initTestRemote creates a local git repo with two commits to stand in for the remote,
// returning its path and HEAD sha.
func initTestRemote(t *testing.T) (dir string, headSHA string) {
	t.Helper()
	dir = t.TempDir()

	testGit(t, dir, "init", "-q", "-b", "main")
	commitTestFile(t, dir, "hello.txt", "hello v1", "first")
	headSHA = commitTestFile(t, dir, "hello.txt", "hello v2", "second")
	return dir, headSHA
}

func commitTestFile(t *testing.T, dir, name, content, message string) (sha string) {
	t.Helper()

	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644))
	testGit(t, dir, "add", ".")
	testGit(t, dir,
		"-c", "user.name=platform", "-c", "user.email=platform@test",
		"commit", "-q", "-m", message)
	return testGit(t, dir, "rev-parse", "HEAD")
}

func testGit(t *testing.T, dir string, args ...string) string {
	t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git %v: %s", args, out)
	return strings.TrimSpace(string(out))
}

func TestPrepRepoRejectsPathEscapingNames(t *testing.T) {
	remote, sha := initTestRemote(t)
	cache := t.TempDir()

	hostile := []struct{ owner, repo string }{
		{"..", "repo"},
		{"owner", ".."},
		{"owner/../..", "repo"},
		{"owner", "repo/../../escape"},
		{"", "repo"},
		{"owner", ""},
		{".hidden", "repo"},
	}
	for _, h := range hostile {
		prep := &PrepRepo{
			CacheDir: cache, CloneURL: "file://" + remote,
			Owner: h.owner, Repo: h.repo, SHA: sha, BuildID: 1,
		}
		_, _, err := prep.Run(t.Context())
		require.ErrorContains(t, err, "invalid repo path", "owner=%q repo=%q", h.owner, h.repo)
	}
}
