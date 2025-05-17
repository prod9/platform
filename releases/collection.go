package releases

import (
	"fmt"
	"iter"
	"slices"
	"sort"
	"strings"

	"platform.prodigy9.co/gitctx/gitcmd"
	"platform.prodigy9.co/project"
)

// Collection is a collection of names of release irrespective of which naming strategy is
// used. It encodes git operations required to list, create, and recover releases.
type Collection struct {
	cfg   *project.Project
	names []string
}

func Recover(cfg *project.Project) (*Collection, error) {
	// ensure the local wd has all the up-to-date tags
	//
	// note that environment tags will change as we deploy to the environment so we need to
	// do a fetch --force just for those tags.
	if branch, err := gitcmd.CurrentBranch(cfg.ConfigDir); err != nil {
		return nil, err
	} else if remote, err := gitcmd.TrackingRemote(cfg.ConfigDir, branch); err != nil {
		return nil, err
	} else if _, err := gitcmd.FetchFTags(cfg.ConfigDir, remote, cfg.Environments); err != nil {
		return nil, err
	} else if _, err := gitcmd.FetchTags(cfg.ConfigDir, remote); err != nil {
		return nil, err
	}

	lines, err := gitcmd.ListTags(cfg.ConfigDir, "v*")
	if err != nil {
		return nil, err
	}

	names := strings.Split(lines, "\n")
	sort.Sort(sort.Reverse(sort.StringSlice(names)))
	return &Collection{
		cfg:   cfg,
		names: names,
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
	if name == "" {
		return nil, ErrNoRelease
	} else if msg, err := gitcmd.TagMessage(c.cfg.ConfigDir, name); err != nil {
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

func (c *Collection) PendingChanges() ([]CommitRef, error) {
	last := c.LatestName(nil)
	if last == "" {
		return nil, nil
	}

	return listCommits(c.cfg.ConfigDir, last)
}
