package releases

import (
	"errors"
	"log"
	"strconv"
	"strings"

	"golang.org/x/mod/semver"
	"platform.prodigy9.co/config"
	"platform.prodigy9.co/gitcmd"
)

var (
	ErrNoSemver       = errors.New("no valid semver tag found")
	ErrNoSemverOption = errors.New("at least one of --major, --minor or --patch must be specified")
)

type Semver struct{}

var _ Strategy = Semver{}

func (s Semver) Generate(cfg *config.Config, opts *Options) (*Release, error) {
	nextVer := ""

	prevVer, err := s.mostRecentVer(cfg.ConfigDir)
	if errors.Is(err, ErrNoSemver) {
		nextVer = "v0.0.0"
	} else if err != nil {
		return nil, err
	} else if nextVer, err = s.nextVer(prevVer, opts); err != nil {
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
		return "", ErrNoSemver
	}

	mostRecent := tags[len(tags)-1]
	if !semver.IsValid(mostRecent) {
		return "", ErrNoSemver
	} else {
		return mostRecent, nil
	}
}

func (s Semver) nextVer(ver string, opts *Options) (string, error) {
	parts := strings.Split(semver.Canonical(ver), ".")

	if opts.IncrementMajor {
		n, _ := strconv.Atoi(parts[0][1:])
		return "v" + strconv.Itoa(n+1) + "." + parts[1] + "." + parts[2], nil
	}
	if opts.IncrementMinor {
		n, _ := strconv.Atoi(parts[1])
		return parts[0] + "." + strconv.Itoa(n+1) + "." + parts[2], nil
	}
	if opts.IncrementPatch {
		n, _ := strconv.Atoi(parts[2])
		return parts[0] + "." + parts[1] + "." + strconv.Itoa(n+1), nil
	}

	return "", ErrNoSemverOption
}
