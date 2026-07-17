package releases

import (
	"cmp"
	"fmt"
	"iter"
	"slices"
	"strings"
	"time"

	"golang.org/x/mod/semver"
	"platform.prodigy9.co/conf"
	"platform.prodigy9.co/git"
	"platform.prodigy9.co/releases/dateref"
	"platform.prodigy9.co/releases/timeref"
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

// sortReleaseNames orders names newest-first under compareNames.
func sortReleaseNames(names []string) {
	slices.SortFunc(names, func(a, b string) int {
		return compareNames(b, a)
	})
}

// Name classes, ascending precedence. Chronological refs sit above semver so the
// repo-wide newest (LatestName(nil), the changelog base) tracks the calendar even in a
// repo that migrated strategies.
const (
	classOther = iota
	classSemver
	classChrono
)

// compareNames is a total order over every tag-name class in the wild: chronological
// refs (datestamp/timestamp, by the moment they name, counter breaking ties) above
// semver tags (numeric — a string sort puts v0.9.9 above v0.9.10) above anything else
// (byte order). Delegating per *pair* was not a strict weak order (semver reads a
// 12-digit timestamp as a huge major and a datestamp counter as a prerelease), and
// SortFunc scrambled the mixed list — LatestName then picked a stale prev and release
// re-cut an existing tag.
func compareNames(a, b string) int {
	aClass, bClass := classOf(a), classOf(b)
	if aClass != bClass {
		return cmp.Compare(aClass, bClass)
	}

	switch aClass {
	case classChrono:
		aTime, aCounter := chronoKey(a)
		bTime, bCounter := chronoKey(b)
		if c := aTime.Compare(bTime); c != 0 {
			return c
		}
		return cmp.Compare(aCounter, bCounter)
	case classSemver:
		return semver.Compare(a, b)
	default:
		return strings.Compare(a, b)
	}
}

func classOf(name string) int {
	switch {
	case dateref.IsValid(name) || timeref.IsValid(name):
		return classChrono
	case semver.IsValid(name):
		return classSemver
	default:
		return classOther
	}
}

// chronoKey reads the moment a chronological ref names; timestamp refs carry no
// counter and tie-break at zero. Only called on classChrono names, so parses succeed.
func chronoKey(name string) (time.Time, int) {
	if ref, err := dateref.Parse(name); err == nil {
		return ref.Time(), ref.Counter()
	}

	moment, err := timeref.Parse(name)
	if err != nil {
		panic("chronoKey on non-chronological name " + name)
	}
	return moment, 0
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
