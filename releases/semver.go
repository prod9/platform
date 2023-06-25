package releases

import (
	"errors"
	"strings"

	"golang.org/x/mod/semver"
	"platform.prodigy9.co/config"
	"platform.prodigy9.co/gitcmd"
)

var (
	ErrNoSemver        = errors.New("a valid semver is required to create release")
	ErrBadSemver       = errors.New("release name is not semver")
	ErrNoRecentVersion = errors.New("no valid semver tag found")
	ErrDirtyWorkdir    = errors.New("working directory has uncommitted changes")
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

	// ensure we have clean worktree (so we don't accidentally release something that's not
	// already in a commit)
	status, err := gitcmd.Status(cfg.ConfigDir)
	if err != nil {
	} else if status != "" && !opts.Force {
		return nil, ErrDirtyWorkdir
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
	if _, err := gitcmd.Tag(cfg.ConfigDir, rel.Name, rel.Message); err != nil {
		return err
	} else if _, err := gitcmd.PushTag(cfg.ConfigDir, rel.Name); err != nil {
		return err
	} else {
		return nil
	}
}

func (s Semver) Publish(cfg *config.Config, rel *Release) error {
	return nil
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
