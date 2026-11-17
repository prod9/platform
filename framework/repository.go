package framework

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"dagger.io/dagger"
	"platform.prodigy9.co/git"
)

// repository is the host checkout a build step sees: its git metadata, mounted rather
// than copied, and the revision it names (docs/spec/frameworks.md, "The build step sees
// the repository").
type repository struct {
	revision string

	files []gitMount // a worktree's .git pointer file
	dirs  []gitMount // the .git directory, or the commondir and gitdir a worktree points at
}

// gitMount places one host path at one container path.
type gitMount struct{ at, host string }

// findRepository resolves the checkout hostdir belongs to. hostdir is the directory the
// build copies into SrcDir, so the git root's .git lands where it sits relative to the
// source. A worktree's .git is a file whose gitdir and commondir are mounted at their own
// host paths, so the pointers resolve inside the container unchanged. Outside any git
// repository there is nothing to mount, and before the first commit no revision to name.
func findRepository(ctx context.Context, hostdir string) (repository, error) {
	absdir, err := filepath.Abs(hostdir)
	if err != nil {
		return repository{}, err
	}
	root, found := gitRoot(absdir)
	if !found {
		return repository{}, nil
	}
	rel, err := filepath.Rel(absdir, root)
	if err != nil {
		return repository{}, err
	}

	revision, err := git.Head(ctx, absdir)
	if err != nil {
		return repository{}, err
	}
	dotgit := filepath.Join(root, ".git")
	at := path.Join(SrcDir, filepath.ToSlash(rel), ".git")
	info, err := os.Stat(dotgit)
	if err != nil {
		return repository{}, err
	}
	if info.IsDir() {
		return repository{revision: revision, dirs: []gitMount{{at: at, host: dotgit}}}, nil
	}

	gitdir, err := readGitPointer(dotgit, root, "gitdir: ")
	if err != nil {
		return repository{}, err
	}
	commondir, err := readGitPointer(filepath.Join(gitdir, "commondir"), gitdir, "")
	if err != nil {
		return repository{}, err
	}
	return repository{
		revision: revision,
		files:    []gitMount{{at: at, host: dotgit}},
		dirs:     []gitMount{{at: commondir, host: commondir}, {at: gitdir, host: gitdir}},
	}, nil
}

// gitRoot is the nearest directory at or above dir holding a .git entry, if any.
func gitRoot(dir string) (string, bool) {
	for !git.IsRoot(dir) {
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
	return dir, true
}

// readGitPointer reads the path a git pointer file names after prefix, resolved against
// base when git wrote it relative.
func readGitPointer(file, base, prefix string) (string, error) {
	content, err := os.ReadFile(file)
	if err != nil {
		return "", err
	}

	target, found := strings.CutPrefix(strings.TrimSpace(string(content)), prefix)
	if !found {
		return "", fmt.Errorf("%s: %w", file, ErrBadGitPointer)
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(base, target)
	}
	return filepath.Clean(target), nil
}

// withRepository mounts the checkout's git metadata. A mount never enters an image layer,
// and every runner starts from its own base, so the runner never receives it.
func withRepository(client *dagger.Client, base *dagger.Container, repo repository) *dagger.Container {
	for _, mount := range repo.files {
		base = base.WithMountedFile(mount.at, client.Host().File(mount.host))
	}
	for _, mount := range repo.dirs {
		base = base.WithMountedDirectory(mount.at, client.Host().Directory(mount.host))
	}
	return base
}

// withBuildEnv names what the build step is building. BUILD_REVISION stays unset before
// the first commit, and BUILD_VERSION when the build has no release tag.
func withBuildEnv(base *dagger.Container, unit *BuildUnit, revision string) *dagger.Container {
	named := base.
		WithEnvVariable("BUILD_MODULE", unit.Name).
		WithEnvVariable("BUILD_REPOSITORY", unit.Repository)
	named = withEnvWhenSet(named, "BUILD_REVISION", revision)
	named = withEnvWhenSet(named, "BUILD_VERSION", unit.Version)
	return named
}

// withEnvWhenSet leaves key unset, rather than empty, when there is no value for it.
func withEnvWhenSet(base *dagger.Container, key, value string) *dagger.Container {
	if value == "" {
		return base
	}
	return base.WithEnvVariable(key, value)
}
