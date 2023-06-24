package releases

import (
	"errors"
	"strings"

	"platform.prodigy9.co/config"
	"platform.prodigy9.co/gitcmd"
)

var (
	ErrBadStrategy = errors.New("releases: invalid strategy")
)

type CommitRef struct {
	Hash    string `toml:"hash"`
	Subject string `toml:"subject"`
}

type Release struct {
	Name    string      `toml:"name"`
	Message string      `toml:"message"`
	Commits []CommitRef `toml:"commits"`
}

type Options struct {
	IncrementMajor bool
	IncrementMinor bool
	IncrementPatch bool
}

type Strategy interface {
	Generate(cfg *config.Config, opts *Options) (*Release, error)
	Create(cfg *config.Config, rel *Release) error
	// Build(*Release) error
	// Publish(*Release) error
}

var knownStrategies = map[string]Strategy{
	"semver": Semver{},
}

func FindStrategy(name string) (Strategy, error) {
	name = strings.ToLower(strings.TrimSpace(name))
	if strat, ok := knownStrategies[name]; ok {
		return strat, nil
	} else {
		return nil, ErrBadStrategy
	}
}

func generateMessage(cfg *config.Config, refs []CommitRef) string {
	//* [f3e0f9][https://github.com/prod9/platform/commit/f3e0f9] Sample message
	sb := &strings.Builder{}
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