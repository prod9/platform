package releases

import (
	"errors"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/mod/semver"
	"platform.prodigy9.co/gitcmd"
	"platform.prodigy9.co/project"
)

var (
	ErrNoSemver        = errors.New("a valid semver is required to create release")
	ErrBadSemver       = errors.New("release name is not semver")
	ErrNoRecentVersion = errors.New("no valid semver tag found")
	ErrDirtyWorkdir    = errors.New("working directory has uncommitted changes")
)

type Semver struct{}

var _ Strategy = Semver{}

func (s Semver) List(cfg *project.Project) ([]*Release, error) {
	lines, err := gitcmd.ListTags(cfg.ConfigDir)
	if err != nil {
		return nil, err
	}

	var result []*Release
	for _, line := range strings.Split(lines, "\n") {
		if semver.IsValid(line) {
			result = append(result, &Release{Name: line})
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return semver.Compare(result[i].Name, result[j].Name) < 0
	})
	return result, nil
}

func (s Semver) Recover(cfg *project.Project, opts *Options) (*Release, error) {
	// get annotated tag and name
	if opts.Name == "" {
		tagname, err := gitcmd.Describe(cfg.ConfigDir)
		if err != nil {
			return nil, err
		} else if !semver.IsValid(tagname) {
			return nil, ErrBadSemver
		}

		opts.Name = tagname
	}

	tagmsg, err := gitcmd.TagMessage(cfg.ConfigDir, opts.Name)
	if err != nil {
		return nil, err
	}

	return &Release{Name: opts.Name, Message: tagmsg}, nil
}

func (s Semver) NextName(cfg *project.Project, comp NameComponent) (string, error) {
	if comp == "" {
		comp = NamePatch
	}

	tagname, err := gitcmd.Describe(cfg.ConfigDir)
	if err != nil {
		return "", err
	}
	if idx := strings.IndexRune(tagname, '-'); idx > -1 {
		tagname = tagname[:idx]
	}

	v := semver.Canonical(tagname)
	parts := strings.Split(v, ".")
	switch comp {
	case NamePatch:
		if n, err := strconv.Atoi(parts[2]); err != nil {
			return "", ErrBadSemver
		} else {
			parts[2] = strconv.Itoa(n + 1)
		}

	case NameMinor:
		if n, err := strconv.Atoi(parts[1]); err != nil {
			return "", ErrBadSemver
		} else {
			parts[1] = strconv.Itoa(n + 1)
			parts[2] = "0"
		}

	case NameMajor:
		if n, err := strconv.Atoi(parts[0][1:]); err != nil {
			return "", ErrBadSemver
		} else {
			parts[0] = "v" + strconv.Itoa(n+1)
			parts[1] = "0"
			parts[2] = "0"
		}

	default:
		return "", errors.New("invalid version component: " + string(comp))
	}

	return strings.Join(parts, "."), nil
}

func (s Semver) Generate(cfg *project.Project, opts *Options) (*Release, error) {
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

func (s Semver) Create(cfg *project.Project, rel *Release) error {
	if _, err := gitcmd.Tag(cfg.ConfigDir, rel.Name, rel.Message); err != nil {
		return err
	} else if branch, err := gitcmd.CurrentBranch(cfg.ConfigDir); err != nil {
		return err
	} else if remote, err := gitcmd.TrackingRemote(cfg.ConfigDir, branch); err != nil {
		return err
	} else if _, err := gitcmd.PushTag(cfg.ConfigDir, remote, rel.Name); err != nil {
		return err
	} else {
		return nil
	}
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
