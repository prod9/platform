package releases

import (
	"errors"
	"strings"

	"platform.prodigy9.co/gitcmd"
	"platform.prodigy9.co/project"
)

var (
	ErrBadStrategy = errors.New("releases: invalid strategy")
	ErrBadEnv      = errors.New("releases: invalid environment")
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
		Name  string
		Force bool
	}

	Strategy interface {
		List(cfg *project.Project) ([]*Release, error)
		Recover(cfg *project.Project, opts *Options) (*Release, error)

		NextName(cfg *project.Project, comp NameComponent) (string, error)
		Generate(cfg *project.Project, opts *Options) (*Release, error)
		Create(cfg *project.Project, rel *Release) error
	}
)

var knownStrategies = map[string]Strategy{
	"semver":    Semver{},
	"timestamp": Timestamp{},
	"datestamp": Datestamp{},
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

	// TODO: Something a bit more efficient than Split->Split
	lines := strings.Split(raw, "\n")
	for _, line := range lines {
		if len(line) < 7 { // abbrev-hash is min 7 chars
			continue
		}

		parts := strings.SplitN(line, " ", 2)
		refs = append(refs, CommitRef{
			Hash:    parts[0],
			Subject: parts[1],
		})
	}

	return refs, nil
}

func IsBadRelease(err error) bool {
	switch err {
	case ErrBadDatestamp, ErrBadTimestamp, ErrBadSemver:
		return true
	default:
		return false
	}
}

func MatchEnv(cfg *project.Project) (bool, error) {
	tagname, err := gitcmd.Describe(cfg.ConfigDir)
	if err != nil {
		return false, err
	}
	for _, env := range cfg.Environments {
		if env == tagname {
			return true, nil
		}
	}
	return false, ErrBadEnv
}
