package git

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"platform.prodigy9.co/internal/buildlog"
	"platform.prodigy9.co/project"
)

var ErrDirtyWorkdir = errors.New("git: working directory is dirty")

// Context runs git against a project's repository, caching the per-process constants
// (current branch, tracking remote) so repeated reads cost one subprocess each.
type Context struct {
	proj *project.Project

	currentBranch func() (string, error)
	mainRemote    func() (string, error)
}

func New(proj *project.Project) *Context {
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

func (ctx *Context) CurrentBranch() (string, error)  { return ctx.currentBranch() }
func (ctx *Context) MainRemoteName() (string, error) { return ctx.mainRemote() }

// IsClean checks if the working directory is clean (no uncommitted changes)
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

// UpdateAllTags fetches all version tags from remote
func (ctx *Context) UpdateAllTags() error {
	remote, err := ctx.MainRemoteName()
	if err != nil {
		return err
	}
	_, err = ctx.run("fetch", "--tags", remote)
	return err
}

// SetVersionTag creates an annotated version tag with message
func (ctx *Context) SetVersionTag(tagname, message string) (string, error) {
	return ctx.run("tag", "-a", "-m", message, tagname)
}

// PushVersionTag pushes a version tag to remote
func (ctx *Context) PushVersionTag(tagname string) error {
	remote, err := ctx.MainRemoteName()
	if err != nil {
		return err
	}
	_, err = ctx.run("push", "--porcelain", remote, tagname)
	return err
}

// ListTags lists tags matching the given pattern
func (ctx *Context) ListTags(pattern string) (string, error) {
	return ctx.run("tag", "-l", pattern)
}

// GetTagMessage retrieves the message of an annotated tag
func (ctx *Context) GetTagMessage(tagname string) (string, error) {
	return ctx.run("tag", "-l", "--format=%(contents)", tagname)
}

// RecentCommits returns recent commit history for changelog generation
func (ctx *Context) RecentCommits() (string, error) {
	return ctx.run("log", "--pretty=%h %s")
}

// CommitsSinceTag returns commit history since a specific tag
func (ctx *Context) CommitsSinceTag(tagname string) (string, error) {
	return ctx.run("log", "--pretty=%h %s", tagname+"..HEAD")
}

func (ctx *Context) run(args ...string) (string, error) {
	wd, err := filepath.Abs(ctx.proj.ConfigDir)
	if err != nil {
		return "", err
	}

	buildlog.Git("git", args...)
	outbuf := &strings.Builder{}

	cmd := exec.Command("git", args...)
	cmd.Dir = wd
	cmd.Stdout = outbuf
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", err
	}

	return strings.TrimSpace(outbuf.String()), nil
}
