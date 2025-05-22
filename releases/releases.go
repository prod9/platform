package releases

import (
	"errors"
	"strings"

	"github.com/pterm/pterm"
	"github.com/pterm/pterm/putils"
	"platform.prodigy9.co/gitctx"
	"platform.prodigy9.co/project"
)

var (
	ErrNoRelease           = errors.New("cannot find a valid release")
	ErrBadStrategy         = errors.New("invalid strategy")
	ErrBadVersion          = errors.New("invalid version")
	ErrBadVersionComponent = errors.New("invalid version component")
	ErrDirtyWorkdir        = errors.New("working directory has uncommitted changes")
)

type NameComponent string

const (
	NameAny   NameComponent = "any"
	NamePatch NameComponent = "patch"
	NameMinor NameComponent = "minor"
	NameMajor NameComponent = "major"
)

type (
	Release struct {
		Name    string             `toml:"name"`
		Message string             `toml:"message"`
		Commits []gitctx.CommitRef `toml:"commits"`
	}
	Options struct {
		Name      string
		Force     bool
		Component NameComponent
	}

	Strategy interface {
		IsValid(name string) bool
		NextName(prevName string, comp NameComponent) (string, error)
	}
)

var knownStrategies = map[string]Strategy{
	"semver":    Semver{},
	"timestamp": Timestamp{},
	"datestamp": Datestamp{},
}

func Generate(cfg *project.Project, opts *Options) (*Release, error) {
	git, err := gitctx.New(cfg)
	if err != nil {
		return nil, err
	} else if err = git.IsClean(); err != nil {
		return nil, err
	}

	collection, err := Recover(cfg)
	if err != nil {
		return nil, err
	}

	strat, err := FindStrategy(cfg.Strategy)
	if err != nil {
		return nil, err
	}

	var refs []gitctx.CommitRef
	prevName := collection.LatestName(strat)
	if prevName == "" {
		refs, err = git.RecentCommits()
	} else {
		refs, err = git.CommitsSinceTag(prevName)
	}
	if err != nil {
		return nil, err
	}

	nextName, err := strat.NextName(prevName, opts.Component)
	if err != nil {
		return nil, err
	}

	return &Release{
		Name:    nextName,
		Message: generateMessage(cfg, nextName, refs),
		Commits: refs,
	}, nil
}

func Create(cfg *project.Project, rel *Release) error {
	git, err := gitctx.New(cfg)
	if err != nil {
		return err
	}

	if err := git.UpdateEnvironmentTags(); err != nil {
		return err
	} else if err := git.UpdateAllTags(); err != nil {
		return err
	} else if _, err := git.SetVersionTag(rel.Name, rel.Message); err != nil {
		return err
	} else if err := git.PushVersionTag(rel.Name); err != nil {
		return err
	} else {
		return nil
	}
}

func (r *Release) Render() error {
	list := pterm.LeveledList{pterm.LeveledListItem{Level: 0, Text: r.Name}}
	for _, c := range r.Commits {
		list = append(list, pterm.LeveledListItem{Level: 1, Text: c.Hash + ": " + c.Subject})
	}

	return pterm.DefaultTree.
		WithRoot(putils.TreeFromLeveledList(list)).
		Render()
}

func FindStrategy(name string) (Strategy, error) {
	name = strings.ToLower(strings.TrimSpace(name))
	if strat, ok := knownStrategies[name]; ok {
		return strat, nil
	} else {
		return nil, ErrBadStrategy
	}
}

func generateMessage(cfg *project.Project, title string, refs []gitctx.CommitRef) string {
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
