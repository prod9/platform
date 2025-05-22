package gitctx

import (
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"sync"

	"github.com/go-git/go-git/v5"
	"platform.prodigy9.co/internal/plog"
	"platform.prodigy9.co/project"
)

var (
	ErrNoMainRemote    = errors.New("git: could not find primary remote")
	ErrNoCurrentBranch = errors.New("git: could not find current branch")
	ErrTagExists       = errors.New("git: tag already exists")
	ErrDirtyWorkdir    = errors.New("git: working directory is dirty")
)

type (
	GitCtx struct {
		proj *project.Project
		repo *git.Repository

		currentBranch func() (string, error)
		mainRemote    func() (*git.Remote, error)
	}

	CommitRef struct {
		Hash    string `toml:"hash"`
		Subject string `toml:"subject"`
	}
)

// TODO: Optimize:
// only need to fetch tags if local environment tag exist
// because the tag clobberring problem would only happen
// if a remote tag would clobber the local tag
// also the same thing likely need to be done for version tags
// as well, maybe the most recent ones or something

func New(proj *project.Project) (*GitCtx, error) {
	repo, err := git.PlainOpen(proj.ConfigDir)
	if err != nil {
		return nil, wrapErr(err)
	}

	return &GitCtx{
		repo: repo,
		proj: proj,
	}, nil
}

func (g *GitCtx) IsClean() error {
	wt, err := g.repo.Worktree()
	if err != nil {
		return wrapErr(err)
	}

	st, err := wt.Status()
	switch {
	case err != nil:
		return wrapErr(err)
	case !st.IsClean():
		return ErrDirtyWorkdir
	default:
		return nil
	}
}

func (g *GitCtx) CurrentBranch() (string, error) {
	if g.currentBranch != nil {
		if branch, err := g.currentBranch(); err != nil {
			return "", wrapErr(err)
		} else if branch == "" {
			return "", ErrNoCurrentBranch
		} else {
			return branch, nil
		}
	}

	g.currentBranch = sync.OnceValues(func() (string, error) {
		plog.Git("branch", "checking")

		head, err := g.repo.Head()
		if err != nil {
			return "", wrapErr(err)
		}
		return head.Name().Short(), nil
	})

	return g.CurrentBranch()
}

func (g *GitCtx) MainRemoteName() (string, error) {
	if remote, err := g.MainRemote(); err != nil {
		return "", err
	} else {
		return remote.Config().Name, nil
	}
}
func (g *GitCtx) MainRemote() (*git.Remote, error) {
	if g.mainRemote != nil {
		if remote, err := g.mainRemote(); err != nil {
			return nil, wrapErr(err)
		} else if remote == nil {
			return nil, ErrNoMainRemote
		} else {
			return remote, nil
		}
	}

	g.mainRemote = sync.OnceValues(func() (*git.Remote, error) {
		plog.Git("remote", "checking")
		remotes, err := g.repo.Remotes()
		if err != nil {
			return nil, err
		}

		var repoURL *url.URL
		if u, err := url.Parse(g.proj.Repository); err != nil {
			return nil, err
		} else {
			repoURL = u
		}

		for _, remote := range remotes {
			if len(remote.Config().URLs) < 1 {
				continue
			}

			rawURL := remote.Config().URLs[0]
			if u, err := url.Parse(rawURL); err != nil {
				// ignore malformed remote
				plog.Error(err)
				continue
			} else if !repoPathMatch(u, repoURL) {
				// we only care about the path because people can have ssh remotes setup
				// with varying names and protocols
				continue
			} else {
				return remote, nil
			}
		}

		// no main remote found
		return nil, nil
	})

	return g.MainRemote()
}

func (g *GitCtx) RecentCommits() ([]CommitRef, error) {
	plog.Git("log", "recent commits")

	it, err := g.repo.Log(&git.LogOptions{All: true})
	if err != nil {
		return nil, wrapErr(err)
	}

	var commits []CommitRef
	for commit, err := range CommitSeqFromIter(it) {
		if err != nil {
			return nil, wrapErr(err)
		} else {
			commits = append(commits, commit)
		}
	}

	slices.Reverse(commits)
	return commits, nil
}

func (g *GitCtx) CommitsSinceTag(tagname string) ([]CommitRef, error) {
	plog.Git("log", "commits since "+tagname)

	it, err := g.repo.Log(&git.LogOptions{All: true})
	if err != nil {
		return nil, wrapErr(err)
	}

	tag, err := g.repo.Tag(tagname)
	if err != nil {
		return nil, wrapErr(err)
	}

	var commits []CommitRef
	for commit, err := range CommitSeqFromIter(it) {
		if err != nil {
			return nil, wrapErr(err)
		} else if commit.Hash == tag.Hash().String() {
			break
		}
	}

	slices.Reverse(commits)
	return commits, nil
}

func wrapErr(err error) error {
	if err == nil {
		return nil
	} else {
		return fmt.Errorf("git: %w", err)
	}
}

func repoPathMatch(a, b *url.URL) bool {
	if a.Path == b.Path || a.Opaque == b.Opaque {
		return true
	}

	as, bs := a.Path, b.Path
	if strings.HasPrefix(as, "/") {
		as = as[1:]
	}
	if strings.HasPrefix(bs, "/") {
		bs = bs[1:]
	}

	return a.Path == b.Opaque ||
		b.Path == a.Opaque ||
		a.Opaque == bs ||
		b.Opaque == as
}
