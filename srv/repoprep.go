package srv

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"syscall"

	"fx.prodigy9.co/config"
	"platform.prodigy9.co/git"
)

// CacheDirConfig roots the server's persistent clone cache (spec §Cache layout):
// git/<owner>/<repo>.git bare mirrors + work/<build-id>/ per-build worktrees.
var CacheDirConfig = config.StrDef("CACHE_DIR", "/var/cache/platform")

// PrepRepo produces a local working tree for a build (spec §Repo preparation): a full
// bare mirror of the repo (clone --mirror once, incremental fetch after — never
// shallow), the input sha resolved to a full commit sha (the committed-image-pin
// anchor), and a per-build worktree added off the mirror. Only the mirror mutation
// locks; worktrees are independent and removed by RemoveWorkTree after the build.
type PrepRepo struct {
	CacheDir string
	CloneURL string
	Owner    string
	Repo     string
	SHA      string
	BuildID  int64
}

func (p *PrepRepo) Run(ctx context.Context) (workDir string, resolvedSHA string, err error) {
	if err := checkRepoPath(p.Owner, p.Repo); err != nil {
		return "", "", err
	}

	mirror := mirrorPath(p.CacheDir, p.Owner, p.Repo)
	if err := p.syncMirror(ctx, mirror); err != nil {
		return "", "", err
	}

	resolvedSHA, err = git.Run(ctx, mirror, "rev-parse", p.SHA+"^{commit}")
	if err != nil {
		return "", "", err
	}

	workDir = workPath(p.CacheDir, p.BuildID)
	if err := os.MkdirAll(filepath.Dir(workDir), 0o755); err != nil {
		return "", "", err
	}
	if _, err := git.Run(ctx, mirror, "worktree", "add", "--detach", workDir, resolvedSHA); err != nil {
		return "", "", err
	}
	return workDir, resolvedSHA, nil
}

// syncMirror clones or fetches the bare mirror under an exclusive flock, serializing
// concurrent preps of the same repo on its one mutation.
func (p *PrepRepo) syncMirror(ctx context.Context, mirror string) error {
	if err := os.MkdirAll(filepath.Dir(mirror), 0o755); err != nil {
		return err
	}
	lock, err := lockFile(mirror + ".lock")
	if err != nil {
		return err
	}
	defer lock.Close()

	if _, err := os.Stat(mirror); os.IsNotExist(err) {
		_, err := git.Run(ctx, filepath.Dir(mirror), "clone", "--mirror", p.CloneURL, mirror)
		return err
	} else if err != nil {
		return err
	}

	_, err = git.Run(ctx, mirror, "fetch", "--prune", "origin")
	return err
}

// RemoveWorkTree is the post-build cleanup for a PrepRepo worktree: it owns removing
// the build's worktree and pruning the mirror's worktree records.
type RemoveWorkTree struct {
	CacheDir string
	Owner    string
	Repo     string
	BuildID  int64
}

func (r *RemoveWorkTree) Run(ctx context.Context) error {
	mirror := mirrorPath(r.CacheDir, r.Owner, r.Repo)

	if _, err := git.Run(ctx, mirror, "worktree", "remove", "--force", workPath(r.CacheDir, r.BuildID)); err != nil {
		return err
	}
	_, err := git.Run(ctx, mirror, "worktree", "prune")
	return err
}

// checkRepoPath admits only names GitHub itself allows (letters, digits, '-', plus
// '._' in repo names, never leading '.') — owner/repo land in filesystem paths, so the
// whitelist is what keeps a hostile payload from escaping the cache dir.
func checkRepoPath(owner, repo string) error {
	if !repoNamePattern.MatchString(owner) || !repoNamePattern.MatchString(repo) {
		return fmt.Errorf("srv: invalid repo path: %q/%q", owner, repo)
	}
	return nil
}

var repoNamePattern = regexp.MustCompile(`^[A-Za-z0-9-][A-Za-z0-9._-]*$`)

func mirrorPath(cacheDir, owner, repo string) string {
	return filepath.Join(cacheDir, "git", owner, repo+".git")
}

func workPath(cacheDir string, buildID int64) string {
	return filepath.Join(cacheDir, "work", strconv.FormatInt(buildID, 10))
}

// lockFile opens (creating as needed) path and takes an exclusive flock on it; Close
// releases both.
func lockFile(path string) (*os.File, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX); err != nil {
		file.Close()
		return nil, fmt.Errorf("srv: flock %s: %w", path, err)
	}
	return file, nil
}
