package releases

import (
	"errors"
	"iter"
	"slices"
	"strings"

	"platform.prodigy9.co/conf"
	"platform.prodigy9.co/git"
	"platform.prodigy9.co/internal/buildinfo"
)

var (
	ErrNoRelease      = errors.New("cannot find a valid release")
	ErrBadStrategy    = errors.New("invalid strategy")
	ErrBadVersion     = errors.New("invalid version")
	ErrBadVersionBump = errors.New("invalid version bump")
	ErrDirtyWorkdir   = errors.New("working directory has uncommitted changes")
)

type Bump string

const (
	BumpAny   Bump = "any"
	BumpPatch Bump = "patch"
	BumpMinor Bump = "minor"
	BumpMajor Bump = "major"
)

type (
	CommitRef struct {
		Hash    string `toml:"hash"`
		Subject string `toml:"subject"`
	}
	Release struct {
		Name    string      `toml:"name"`
		Message string      `toml:"message"`
		Commits []CommitRef `toml:"commits"`
	}
	Options struct {
		Name  string
		Force bool
		Bump  Bump
	}

	Strategy interface {
		IsValid(name string) bool
		NextName(prevName string, bump Bump) (string, error)

		// IsVersioned reports whether names carry a version. Versioned strategies derive
		// the publish target from the latest git tag; a non-versioned one (Rolling) has a
		// single constant name and needs no tag — publishing is the deploy.
		IsVersioned() bool
	}
)

var knownStrategies = map[string]Strategy{
	"semver":    Semver{},
	"timestamp": Timestamp{},
	"datestamp": Datestamp{},
	"rolling":   Rolling{},
}

func Generate(cfg *conf.Model, g *git.Context, opts *Options) (*Release, error) {
	if err := checkGitStatus(cfg, g, opts); err != nil {
		return nil, err
	}

	collection, err := Recover(cfg, g)
	if err != nil {
		return nil, err
	}

	strat, err := FindStrategy(cfg.Strategy)
	if err != nil {
		return nil, err
	}

	commits := ""
	prevName := collection.LatestName(strat)
	if prevName != "" {
		commits = prevName + "..HEAD"
	}

	refs, err := listCommits(g, commits)
	if err != nil {
		return nil, err
	}

	nextName, err := strat.NextName(prevName, opts.Bump)
	if err != nil {
		return nil, err
	}

	return &Release{
		Name:    nextName,
		Message: generateMessage(cfg, nextName, refs),
		Commits: refs,
	}, nil
}

func Create(cfg *conf.Model, g *git.Context, rel *Release) error {
	// always fetch remote tags before making changes because someone else might have
	// pushed a tag since we last fetched (or you yourself might have pushed a tag from
	// another machine and forgot)
	if err := g.UpdateAllTags(); err != nil {
		return err
	}
	if _, err := g.SetVersionTag(rel.Name, rel.Message); err != nil {
		return err
	}
	if err := g.PushVersionTag(rel.Name); err != nil {
		return err
	}

	return nil
}

func (r *Release) Changelog() {
	buildinfo.Header(r.Name)
	for _, c := range r.Commits {
		buildinfo.Item(c.Hash + ": " + c.Subject)
	}
}

func checkGitStatus(cfg *conf.Model, g *git.Context, opts *Options) error {
	if !opts.Force {
		if err := g.IsClean(); err != nil {
			if err == git.ErrDirtyWorkdir {
				return ErrDirtyWorkdir
			}
			return err
		}
	}
	return nil
}

func FindStrategy(name string) (Strategy, error) {
	name = strings.ToLower(strings.TrimSpace(name))
	if strat, ok := knownStrategies[name]; ok {
		return strat, nil
	} else {
		return nil, ErrBadStrategy
	}
}

func generateMessage(cfg *conf.Model, title string, refs []CommitRef) string {
	//* [f3e0f9][https://github.com/prod9/platform/commit/f3e0f9] Sample message
	sb := &strings.Builder{}
	sb.WriteString(title)
	sb.WriteRune('\n')
	sb.WriteRune('\n')

	for _, ref := range refs {
		sb.WriteString("* [")
		sb.WriteString(ref.Hash)
		sb.WriteString("][")
		sb.WriteString(cfg.Repository)
		sb.WriteString("/commit/")
		sb.WriteString(ref.Hash)
		sb.WriteString("] ")
		sb.WriteString(ref.Subject)
		sb.WriteRune('\n')
	}
	return sb.String()
}

func listCommits(g *git.Context, range_ string) (refs []CommitRef, err error) {
	var raw string
	if range_ == "" {
		raw, err = g.RecentCommits()
	} else {
		raw, err = g.CommitsSinceTag(strings.Split(range_, "..")[0])
	}
	if err != nil {
		return nil, err
	}

	return slices.Collect(parseLogOutput(raw)), nil
}

func parseLogOutput(raw string) iter.Seq[CommitRef] {
	return func(yield func(CommitRef) bool) {
		hashStart, subjStart := 0, 0
		ref := CommitRef{}
		for idx, r := range raw {
			// example:
			// f3e0f9: Sample message
			if ref.Hash == "" && r == ' ' {
				ref.Hash = raw[hashStart:idx]
				subjStart = idx + 1

			} else if ref.Subject == "" && r == '\n' {
				ref.Subject = raw[subjStart:idx]
				if !yield(ref) {
					return
				}
				ref = CommitRef{}
				hashStart = idx + 1
			}
		}
	}
}
