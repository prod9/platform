package releases

import (
	"fmt"
	"iter"
	"slices"

	"platform.prodigy9.co/gitctx"
	"platform.prodigy9.co/project"
)

// Collection is a collection of names of release irrespective of which naming strategy is
// used. It encodes git operations required to list, create, and recover releases.
type Collection struct {
	cfg   *project.Project
	names []string
}

func Recover(cfg *project.Project) (*Collection, error) {
	git, err := gitctx.New(cfg)
	if err != nil {
		return nil, err
	}

	// ensure all local tags are updated
	if err := git.UpdateEnvironmentTags(); err != nil {
		return nil, err
	} else if err := git.UpdateAllTags(); err != nil {
		return nil, err
	}

	tags, err := git.ListTags("v")
	return &Collection{
		cfg:   cfg,
		names: tags,
	}, nil
}

func (c *Collection) Len() int                      { return len(c.names) }
func (c *Collection) Names() iter.Seq2[int, string] { return slices.All(c.names) }

func (c *Collection) LatestName(strat Strategy) string {
	if len(c.names) == 0 {
		return ""
	}
	if strat == nil {
		return c.names[0]
	}
	for _, name := range c.names {
		if strat.IsValid(name) {
			return name
		}
	}
	return ""
}

func (c *Collection) Get(name string) (*Release, error) {
	// we should have a tighter mapping of release to tags and vice versa
	git, err := gitctx.New(c.cfg)
	if err != nil {
		return nil, err
	}

	if name == "" {
		return nil, ErrNoRelease
	} else if msg, err := git.GetTagMessage(name); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrNoRelease, err)
	} else {
		return &Release{
			Name:    name,
			Message: msg,
		}, nil
	}
}

func (c *Collection) GetLatest(strat Strategy) (*Release, error) {
	if name := c.LatestName(strat); name == "" {
		return nil, ErrNoRelease
	} else {
		return c.Get(name)
	}
}
