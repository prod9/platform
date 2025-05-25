package gitctx

import (
	"errors"
	"sync"

	"platform.prodigy9.co/gitctx/gitcmd"
	"platform.prodigy9.co/project"
)

var (
	ErrDirtyWorkdir = errors.New("git: working directory is dirty")
)

type GitCtx struct {
	proj *project.Project
	
	// Cached functions
	currentBranch func() (string, error)
	mainRemote    func() (string, error)
}

func New(proj *project.Project) *GitCtx {
	ctx := &GitCtx{proj: proj}
	
	// Initialize cached functions
	ctx.currentBranch = sync.OnceValues(func() (string, error) {
		return gitcmd.CurrentBranch(ctx.proj.ConfigDir)
	})
	
	ctx.mainRemote = sync.OnceValues(func() (string, error) {
		branch, err := ctx.currentBranch()
		if err != nil {
			return "", err
		}
		return gitcmd.TrackingRemote(ctx.proj.ConfigDir, branch)
	})
	
	return ctx
}

// CurrentBranch returns the current git branch, caching the result
func (ctx *GitCtx) CurrentBranch() (string, error) {
	return ctx.currentBranch()
}

// MainRemoteName returns the main remote name, caching the result
func (ctx *GitCtx) MainRemoteName() (string, error) {
	return ctx.mainRemote()
}

// IsClean checks if the working directory is clean (no uncommitted changes)
func (ctx *GitCtx) IsClean() error {
	status, err := gitcmd.Status(ctx.proj.ConfigDir)
	if err != nil {
		return err
	}
	if status != "" {
		return ErrDirtyWorkdir
	}
	return nil
}

// Tag Operations following your naming convention

// UpdateEnvironmentTags fetches environment tags with force (equivalent to git fetch -f)
func (ctx *GitCtx) UpdateEnvironmentTags() error {
	remote, err := ctx.MainRemoteName()
	if err != nil {
		return err
	}
	_, err = gitcmd.FetchFTags(ctx.proj.ConfigDir, remote, ctx.proj.Environments)
	return err
}

// UpdateAllTags fetches all version tags from remote
func (ctx *GitCtx) UpdateAllTags() error {
	remote, err := ctx.MainRemoteName()
	if err != nil {
		return err
	}
	_, err = gitcmd.FetchTags(ctx.proj.ConfigDir, remote)
	return err
}

// SetVersionTag creates an annotated version tag with message
func (ctx *GitCtx) SetVersionTag(tagname, message string) (string, error) {
	return gitcmd.Tag(ctx.proj.ConfigDir, tagname, message)
}

// SetEnvironmentTag creates or updates an environment tag (force operation)
func (ctx *GitCtx) SetEnvironmentTag(tagname string) (string, error) {
	return gitcmd.TagF(ctx.proj.ConfigDir, tagname)
}

// PushVersionTag pushes a version tag to remote
func (ctx *GitCtx) PushVersionTag(tagname string) error {
	remote, err := ctx.MainRemoteName()
	if err != nil {
		return err
	}
	_, err = gitcmd.PushTag(ctx.proj.ConfigDir, remote, tagname)
	return err
}

// PushEnvironmentTag pushes an environment tag to remote (force operation)
func (ctx *GitCtx) PushEnvironmentTag(tagname string) error {
	remote, err := ctx.MainRemoteName()
	if err != nil {
		return err
	}
	_, err = gitcmd.PushTagF(ctx.proj.ConfigDir, remote, tagname)
	return err
}

// ListTags lists tags matching the given pattern
func (ctx *GitCtx) ListTags(pattern string) (string, error) {
	return gitcmd.ListTags(ctx.proj.ConfigDir, pattern)
}

// GetTagMessage retrieves the message of an annotated tag
func (ctx *GitCtx) GetTagMessage(tagname string) (string, error) {
	return gitcmd.TagMessage(ctx.proj.ConfigDir, tagname)
}

// RecentCommits returns recent commit history for changelog generation
func (ctx *GitCtx) RecentCommits() (string, error) {
	return gitcmd.Log(ctx.proj.ConfigDir)
}

// CommitsSinceTag returns commit history since a specific tag
func (ctx *GitCtx) CommitsSinceTag(tagname string) (string, error) {
	return gitcmd.LogRange(ctx.proj.ConfigDir, tagname+"..HEAD")
}

// Describe returns git describe output
func (ctx *GitCtx) Describe() (string, error) {
	return gitcmd.Describe(ctx.proj.ConfigDir)
}
