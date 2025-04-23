package releases

import (
	"errors"
	"strings"
	"unicode"

	"github.com/pterm/pterm"
	"github.com/pterm/pterm/putils"
	"platform.prodigy9.co/gitcmd"
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

func Generate(cfg *project.Project, opts *Options) (*Release, error) {
	if err := checkGitStatus(cfg, opts); err != nil {
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

	commits := ""
	prevName := collection.LatestName(strat)
	if prevName != "" {
		commits = prevName + "..HEAD"
	}

	refs, err := listCommits(cfg.ConfigDir, commits)
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
	// always fetch remote tags before making changes because someone else might have
	// pushed a tag since we last fetched (or you yourself might have pushed a tag from
	// another machine and forgot)
	if branch, err := gitcmd.CurrentBranch(cfg.ConfigDir); err != nil {
		return err
	} else if remote, err := gitcmd.TrackingRemote(cfg.ConfigDir, branch); err != nil {
		return err
	} else if _, err := gitcmd.FetchFTags(cfg.ConfigDir, remote, cfg.Environments); err != nil {
		return err
	} else if _, err := gitcmd.FetchTags(cfg.ConfigDir, remote); err != nil {
		return err
	} else if _, err := gitcmd.Tag(cfg.ConfigDir, rel.Name, rel.Message); err != nil {
		return err
	} else if _, err := gitcmd.PushTag(cfg.ConfigDir, remote, rel.Name); err != nil {
		return err
	} else {
		return nil
	}
}

func checkGitStatus(cfg *project.Project, opts *Options) error {
	if status, err := gitcmd.Status(cfg.ConfigDir); err != nil {
		return err
	} else if status != "" && !opts.Force {
		return ErrDirtyWorkdir
	} else {
		return nil
	}
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

func listCommits(wd string, range_ string) (refs []CommitRef, err error) {
	var raw string
	if range_ == "" {
		raw, err = gitcmd.Log(wd)
	} else {
		raw, err = gitcmd.LogRange(wd, range_)
	}
	if err != nil {
		return nil, err
	}

	hashIdx, subjectIdx := 0, 0
	for idx, r := range raw {
		switch {
		case !unicode.IsSpace(r):
			continue
		case hashIdx == 0:
			hashIdx = idx + 1
			continue
		case subjectIdx == 0:
			subjectIdx = idx + 1
			continue
		}

		refs = append(refs, CommitRef{
			Hash:    raw[hashIdx : subjectIdx-1],
			Subject: raw[subjectIdx:idx],
		})
		hashIdx, subjectIdx = 0, 0
	}

	return refs, nil
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
