package framework

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"platform.prodigy9.co/git"
)

// The build step sees the repository (docs/spec/frameworks.md): .git sits where it does
// relative to the source, and a worktree's pointers resolve at their own host paths.
func TestFindRepository(t *testing.T) {
	main, sha := initRepositoryFixture(t)
	worktree := filepath.Join(filepath.Dir(main), "worktree")
	repoGit(t, main, "worktree", "add", "-q", "--detach", worktree, sha)

	tests := []struct {
		name    string
		hostdir string
		want    repository
	}{
		{"root", main, repository{revision: sha,
			dirs: []gitMount{{at: SrcDir + "/.git", host: filepath.Join(main, ".git")}}}},
		{"below the root", filepath.Join(main, "sub"), repository{revision: sha,
			dirs: []gitMount{{at: "/platform/.git", host: filepath.Join(main, ".git")}}}},
		{"worktree", worktree, repository{revision: sha,
			files: []gitMount{{at: SrcDir + "/.git", host: filepath.Join(worktree, ".git")}},
			dirs: []gitMount{
				{at: filepath.Join(main, ".git"), host: filepath.Join(main, ".git")},
				{at: filepath.Join(main, ".git", "worktrees", "worktree"),
					host: filepath.Join(main, ".git", "worktrees", "worktree")},
			}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repo, err := findRepository(t.Context(), test.hostdir)
			require.NoError(t, err)
			require.Equal(t, test.want, repo)
		})
	}
}

// A module outside any git repository gets no mount.
func TestFindRepositoryOutsideGit(t *testing.T) {
	repo, err := findRepository(t.Context(), t.TempDir())
	require.NoError(t, err)
	require.Equal(t, repository{}, repo)
}

// A repository with no commit yet is still mounted; it just names no revision.
func TestFindRepositoryBeforeFirstCommit(t *testing.T) {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	repoGit(t, dir, "init", "-q")

	repo, err := findRepository(t.Context(), dir)
	require.NoError(t, err)
	require.Equal(t, repository{dirs: []gitMount{{at: SrcDir + "/.git", host: filepath.Join(dir, ".git")}}}, repo)
}

// initRepositoryFixture commits one file under sub/ in a fresh repo. The temp dir is
// resolved because git records worktree pointers by their real path.
func initRepositoryFixture(t *testing.T) (string, string) {
	t.Helper()
	parent, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	main := filepath.Join(parent, "main")

	require.NoError(t, os.MkdirAll(filepath.Join(main, "sub"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(main, "sub", "hello.txt"), []byte("hello"), 0o644))
	repoGit(t, main, "init", "-q", "-b", "main")
	repoGit(t, main, "add", ".")
	repoGit(t, main, "-c", "user.name=platform", "-c", "user.email=platform@test",
		"commit", "-q", "-m", "first")
	return main, repoGit(t, main, "rev-parse", "HEAD")
}

func repoGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := git.Run(t.Context(), dir, args...)
	require.NoError(t, err)
	return out
}
