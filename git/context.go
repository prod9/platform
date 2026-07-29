package git

import (
	"context"
	"errors"
	"path/filepath"
	"sync"

	"platform.prodigy9.co/conf"
	"platform.prodigy9.co/internal/termlog"
)

var ErrDirtyWorkdir = errors.New("git: working directory is dirty")

// Context runs git against a project's repository, caching the per-process constants
// (current branch, tracking remote) so repeated reads cost one subprocess each.
type Context struct {
	proj *conf.Model

	currentBranch func() (string, error)
	mainRemote    func() (string, error)
}

func New(proj *conf.Model) *Context {
	ctx := &Context{proj: proj}

	ctx.currentBranch = sync.OnceValues(func() (string, error) {
		return ctx.run("branch", "--show-current")
	})
	ctx.mainRemote = sync.OnceValues(func() (string, error) {
		branch, err := ctx.currentBranch()
		if err != nil {
			return "", err
		}
		if branch == "" {
			branch = "main"
		}
		return ctx.run("config", "branch."+branch+".remote")
	})

	return ctx
}

func (ctx *Context) MainRemoteName() (string, error) { return ctx.mainRemote() }

// IsClean reports a dirty tree as ErrDirtyWorkdir rather than a bool, so a caller that
// only wants to abort on one propagates it without a branch of its own.
func (ctx *Context) IsClean() error {
	status, err := ctx.run("status", "--porcelain")
	if err != nil {
		return err
	}
	if status != "" {
		return ErrDirtyWorkdir
	}
	return nil
}

func (ctx *Context) UpdateAllTags() error {
	remote, err := ctx.MainRemoteName()
	if err != nil {
		return err
	}
	_, err = ctx.run("fetch", "--tags", remote)
	return err
}

func (ctx *Context) SetVersionTag(tagname, message string) (string, error) {
	return ctx.run("tag", "-a", "-m", message, tagname)
}

func (ctx *Context) PushVersionTag(tagname string) error {
	remote, err := ctx.MainRemoteName()
	if err != nil {
		return err
	}
	_, err = ctx.run("push", "--porcelain", remote, tagname)
	return err
}

func (ctx *Context) ListTags(pattern string) (string, error) {
	return ctx.run("tag", "-l", pattern)
}

func (ctx *Context) GetTagMessage(tagname string) (string, error) {
	return ctx.run("tag", "-l", "--format=%(contents)", tagname)
}

// RecentCommits and CommitsSinceTag emit `<short-hash> <subject>` lines, the one shape
// releases.parseLogOutput scans.
func (ctx *Context) RecentCommits() (string, error) {
	return ctx.run("log", "--pretty=%h %s")
}

func (ctx *Context) CommitsSinceTag(tagname string) (string, error) {
	return ctx.run("log", "--pretty=%h %s", tagname+"..HEAD")
}

func (ctx *Context) run(args ...string) (string, error) {
	wd, err := filepath.Abs(ctx.proj.ConfigDir)
	if err != nil {
		return "", err
	}

	termlog.Git(args...)
	return RunWithProgress(context.Background(), wd, args...)
}
