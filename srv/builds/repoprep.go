package builds

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"syscall"

	"fx.prodigy9.co/config"
	"platform.prodigy9.co/git"
	"platform.prodigy9.co/srv/github"
)

// CacheDirConfig roots the server's persistent clone cache (spec §Cache layout):
// git/<owner>/<repo>.git bare mirrors + work/<build-id>/ per-build worktrees.
var CacheDirConfig = config.StrDef("CACHE_DIR", "/var/cache/platform")

// PrepRepo produces a local working tree for a build (spec §Repo preparation): a full
// bare mirror of the repo (init-bare once, credentialed mirror-fetch every sync — never
// shallow), the input sha resolved to a full commit sha (the committed-image-pin
// anchor), and a per-build worktree added off the mirror. Only the mirror mutation
// locks; worktrees are independent and removed by RemoveWorkTree after the build.
type PrepRepo struct {
	CacheDir string
	CloneURL string
	Token    string
	Owner    string
	Repo     string
	SHA      string
	BuildID  int64
}

func (p *PrepRepo) Run(ctx context.Context) (workDir string, resolvedSHA string, err error) {
	if err := github.CheckRepoPath(p.Owner, p.Repo); err != nil {
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

// syncMirror initializes (first sync) then fetches the bare mirror under an exclusive
// flock, serializing concurrent preps of the same repo on its one mutation. The fetch
// passes the credentialed URL as a command argument so the token rides that one
// invocation only — the stored remote keeps the clean CloneURL, the spec-chosen record
// of the mirror's source; nothing in the sync path fetches through it (spec §Repo
// preparation).
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
		if _, err := git.Run(ctx, filepath.Dir(mirror), "init", "--bare", "-q", mirror); err != nil {
			return err
		}
		if _, err := git.Run(ctx, mirror, "remote", "add", "--mirror=fetch", "origin", p.CloneURL); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	_, err = git.Run(ctx, mirror, "fetch", "--prune", credentialedURL(p.CloneURL, p.Token), "+refs/*:refs/*")
	return err
}

// credentialedURL injects token as URL userinfo for a single git invocation; only
// http(s) remotes authenticate this way, so anything else passes through untouched.
func credentialedURL(cloneURL, token string) string {
	parsed, err := url.Parse(cloneURL)
	if err != nil || token == "" {
		return cloneURL
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return cloneURL
	}

	parsed.User = url.UserPassword("x-access-token", token)
	return parsed.String()
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
		return nil, fmt.Errorf("builds: flock %s: %w", path, err)
	}
	return file, nil
}
