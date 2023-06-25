package releases

import (
	"errors"
	"log"
	"strings"

	"golang.org/x/mod/semver"
	"platform.prodigy9.co/config"
	"platform.prodigy9.co/gitcmd"
)

var (
	ErrNoSemver        = errors.New("a valid semver is required to create release")
	ErrBadSemver       = errors.New("release name is not semver")
	ErrNoRecentVersion = errors.New("no valid semver tag found")
)

type Semver struct{}

var _ Strategy = Semver{}

func (s Semver) Generate(cfg *config.Config, opts *Options) (*Release, error) {
	nextVer := opts.Name
	if nextVer == "" {
		return nil, ErrNoSemver
	} else if nextVer != "" && !semver.IsValid(nextVer) {
		return nil, ErrBadSemver
	}

	prevVer, err := s.mostRecentVer(cfg.ConfigDir)
	if err != nil {
		return nil, err
	}

	var commits string
	if prevVer == "" { // first version
		commits = ""
	} else {
		commits = prevVer + "..HEAD"
	}

	refs, err := listCommits(cfg.ConfigDir, commits)
	if err != nil {
		return nil, err
	}

	rel := &Release{
		Name:    nextVer,
		Message: generateMessage(cfg, nextVer, refs),
		Commits: refs,
	}
	return rel, nil
}

func (s Semver) Create(cfg *config.Config, rel *Release) error {
	log.Println("tagging release...")
	if _, err := gitcmd.Tag(cfg.ConfigDir, rel.Name, rel.Message); err != nil {
		return err
	} else {
		return nil
	}
}

// Build implements Releaser
func (Semver) Build(*Release) error {
	panic("unimplemented")
}

// Publish implements Releaser
func (Semver) Publish(*Release) error {
	panic("unimplemented")
}

func (s Semver) mostRecentVer(wd string) (string, error) {
	raw, err := gitcmd.ListTags(wd)
	if err != nil {
		return "", err
	}

	tags := strings.Split(raw, "\n")
	semver.Sort(tags)
	if len(tags) == 0 {
		return "", ErrNoRecentVersion
	}

	mostRecent := tags[len(tags)-1]
	if !semver.IsValid(mostRecent) {
		return "", ErrNoRecentVersion
	} else {
		return mostRecent, nil
	}
}
