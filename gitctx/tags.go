package gitctx

import (
	"errors"
	"strings"

	"github.com/go-git/go-git/v5"
	gitconfig "github.com/go-git/go-git/v5/config"
	gitplumb "github.com/go-git/go-git/v5/plumbing"
	"platform.prodigy9.co/internal/plog"
)

// UpdateEnvironmentTags fetches environment tag from the remote. Equivalent to a
// `git fetch -f` call. The force is necessary because environment tags are continually
// updated as people deploy to the same environment from multiple machines/ci/builds etc.
// Your local environment tag is likely often out of date.
func (g *GitCtx) UpdateEnvironmentTags() error {
	plog.Git("fetch", "environment tags")

	remote, err := g.MainRemote()
	if err != nil {
		return err
	}

	var refs []gitconfig.RefSpec
	for _, env := range g.proj.Environments {
		// we only need to update local refs if it doesn't exist to avoid tag clobbering.
		// if the tag never existed, a normal fetch (i.e. during UpdateAllTags) will fetch it
		// just fine so we can skip all non-existent env tags.
		if _, err := g.repo.Tag(env); err != nil {
			if !errors.Is(err, git.ErrTagNotFound) {
				return wrapErr(err)
			}
		} else {
			refs = append(refs, gitconfig.RefSpec(env+":"+env))
		}
	}

	err = g.repo.Fetch(&git.FetchOptions{
		Force:      true,
		RemoteName: remote.Config().Name,
		RefSpecs:   refs,
	})
	if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		return wrapErr(err)
	} else {
		return nil
	}
}

// UpdateAllTags fetches all tags from the remote. This is done to find nay new version
// tags that may have been added. Call `UpdateEnvironmentTags` before this if you are
// getting tag clobberring errors from environment tags.
func (g *GitCtx) UpdateAllTags() error {
	plog.Git("fetch", "version tags")

	remote, err := g.MainRemote()
	if err != nil {
		return err
	}

	err = g.repo.Fetch(&git.FetchOptions{
		Depth:      1,
		RemoteName: remote.Config().Name,
		Tags:       git.AllTags,
	})
	if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		return wrapErr(err)
	} else {
		return nil
	}
}

func (g *GitCtx) ListTags(prefix string) ([]string, error) {
	plog.Git("tag", "finding version tags")

	tags, err := g.repo.Tags()
	if err != nil {
		return nil, wrapErr(err)
	}

	var names []string
	tags.ForEach(func(ref *gitplumb.Reference) error {
		if strings.HasPrefix(ref.Name().String(), prefix) {
			names = append(names, ref.Name().String())
		}
		return nil
	})
	return names, nil
}

func (g *GitCtx) GetTagMessage(tagname string) (string, error) {
	plog.Git("tag", "recovering "+tagname)

	tag, err := g.repo.Tag(tagname)
	if err != nil {
		return "", wrapErr(err)
	}

	obj, err := g.repo.TagObject(tag.Hash())
	if err != nil {
		return "", wrapErr(err)
	}

	return obj.Message, nil
}

func (g *GitCtx) SetEnvironmentTag(tagname string) (string, error) {
	plog.Git("tag", "updating "+tagname)

	head, err := g.repo.Head()
	if err != nil {
		return "", wrapErr(err)
	}

	if existing, err := g.repo.Tag(tagname); err != nil {
		if !errors.Is(err, git.ErrTagNotFound) {
			return "", wrapErr(err)
		}
	} else if existing != nil {
		if err := g.repo.DeleteTag(tagname); err != nil {
			return "", wrapErr(err)
		}
	}

	if _, err := g.repo.CreateTag(tagname, head.Hash(), nil); err != nil {
		return "", wrapErr(err)
	} else {
		return head.Hash().String(), nil
	}
}

func (g *GitCtx) PushEnvironmentTag(tagname string) error {
	plog.Git("tag", "updating "+tagname)

	name, err := g.MainRemoteName()
	if err != nil {
		return err
	}

	err = g.repo.Push(&git.PushOptions{
		RemoteName: name,
		RefSpecs:   []gitconfig.RefSpec{gitconfig.RefSpec(tagname)},
		Force:      true,
	})
	return wrapErr(err)
}

func (g *GitCtx) SetVersionTag(tagname, message string) (string, error) {
	plog.Git("tag", "creating "+tagname)

	head, err := g.repo.Head()
	if err != nil {
		return "", wrapErr(err)
	}

	if existing, err := g.repo.Tag(tagname); err != nil {
		if !errors.Is(err, git.ErrTagNotFound) {
			return "", wrapErr(err)
		}
	} else if existing != nil {
		return "", ErrTagExists
	}

	if hash, err := g.repo.CreateTag(tagname, head.Hash(), &git.CreateTagOptions{
		Message: message,
	}); err != nil {
		return "", wrapErr(err)
	} else {
		return hash.String(), nil
	}
}

func (g *GitCtx) PushVersionTag(tagname string) error {
	plog.Git("tag", "pushing "+tagname)

	name, err := g.MainRemoteName()
	if err != nil {
		return err
	}

	err = g.repo.Push(&git.PushOptions{
		RemoteName: name,
		RefSpecs:   []gitconfig.RefSpec{gitconfig.RefSpec(tagname)},
	})
	return wrapErr(err)
}
