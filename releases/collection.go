package releases

import (
	"fmt"
	"iter"
	"slices"
	"strings"

	"golang.org/x/mod/semver"
	"platform.prodigy9.co/conf"
	"platform.prodigy9.co/git"
	"platform.prodigy9.co/releases/dateref"
)

// Collection is a collection of names of release irrespective of which naming strategy is
// used. It encodes git operations required to list, create, and recover releases.
type Collection struct {
	cfg   *conf.Model
	names []string
}

func Recover(cfg *conf.Model, g *git.Context) (*Collection, error) {
	// ensure the local wd has all the up-to-date tags
	if err := g.UpdateAllTags(); err != nil {
		return nil, err
	}

	lines, err := g.ListTags("v*")
	if err != nil {
		return nil, err
	}

	names := strings.Split(lines, "\n")
	sortReleaseNames(names)
	return &Collection{
		cfg:   cfg,
		names: names,
	}, nil
}

// sortReleaseNames orders names newest-first. Datestamp refs compare by date then
// counter — semver would read v20260717-1 as a *prerelease* of v20260717 and sort it
// below the bare tag, making LatestName re-yield an existing counter. Semver tags
// compare numerically — a string sort puts v0.9.9 above v0.9.10. Everything else
// keeps byte order.
func sortReleaseNames(names []string) {
	slices.SortFunc(names, func(a, b string) int {
		aRef, aErr := dateref.Parse(a)
		bRef, bErr := dateref.Parse(b)
		switch {
		case aErr == nil && bErr == nil:
			return bRef.Compare(aRef)
		case semver.IsValid(a) && semver.IsValid(b):
			return semver.Compare(b, a)
		default:
			return strings.Compare(b, a)
		}
	})
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

func (c *Collection) Get(g *git.Context, name string) (*Release, error) {
	if name == "" {
		return nil, ErrNoRelease
	}

	msg, err := g.GetTagMessage(name)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrNoRelease, err)
	}

	return &Release{
		Name:    name,
		Message: msg,
	}, nil
}

func (c *Collection) GetLatest(g *git.Context, strat Strategy) (*Release, error) {
	name := c.LatestName(strat)
	if name == "" {
		return nil, ErrNoRelease
	}

	return c.Get(g, name)
}

func (c *Collection) PendingChanges(g *git.Context) ([]CommitRef, error) {
	last := c.LatestName(nil)
	if last == "" {
		return nil, nil
	}

	return listCommits(g, last+"..HEAD")
}
