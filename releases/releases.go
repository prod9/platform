package releases

import (
	"errors"
	"iter"
	"slices"
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

func Generate(cfg *project.Project, git *gitctx.GitCtx, opts *Options) (*Release, error) {
	if err := checkGitStatus(cfg, git, opts); err != nil {
		return nil, err
	}

	collection, err := Recover(cfg, git)
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

	refs, err := listCommits(git, commits)
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

func Create(cfg *project.Project, git *gitctx.GitCtx, rel *Release) error {
	// always fetch remote tags before making changes because someone else might have
	// pushed a tag since we last fetched (or you yourself might have pushed a tag from
	// another machine and forgot)
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

func checkGitStatus(cfg *project.Project, git *gitctx.GitCtx, opts *Options) error {
	if !opts.Force {
		if err := git.IsClean(); err != nil {
			if err == gitctx.ErrDirtyWorkdir {
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

func generateMessage(cfg *project.Project, title string, refs []CommitRef) string {
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

func listCommits(git *gitctx.GitCtx, range_ string) (refs []CommitRef, err error) {
	var raw string
	if range_ == "" {
		raw, err = git.RecentCommits()
	} else {
		raw, err = git.CommitsSinceTag(strings.Split(range_, "..")[0])
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
