package releases

import (
	"errors"
	"sort"
	"strings"

	"platform.prodigy9.co/gitcmd"
	"platform.prodigy9.co/project"
	"platform.prodigy9.co/releases/dateref"
)

var (
	ErrBadDatestamp = errors.New("release name is not datestamp")
)

type Datestamp struct{}

var _ Strategy = Datestamp{}

func (d Datestamp) List(cfg *project.Project) ([]*Release, error) {
	lines, err := gitcmd.ListTags(cfg.ConfigDir)
	if err != nil {
		return nil, err
	}

	var result []*Release
	for _, line := range strings.Split(lines, "\n") {
		if dateref.IsValid(line) {
			result = append(result, &Release{Name: line})
		}
	}
	return result, nil
}

func (d Datestamp) Recover(cfg *project.Project, opts *Options) (*Release, error) {
	// get annotated tag and name
	if opts.Name == "" {
		tagname, err := gitcmd.Describe(cfg.ConfigDir)
		if err != nil {
			return nil, err
		} else if !dateref.IsValid(tagname) {
			return nil, ErrBadDatestamp
		}

		opts.Name = tagname
	}

	tagmsg, err := gitcmd.TagMessage(cfg.ConfigDir, opts.Name)
	if err != nil {
		return nil, err
	}

	return &Release{Name: opts.Name, Message: tagmsg}, nil
}

func (d Datestamp) NextName(cfg *project.Project, comp NameComponent) (string, error) {
	prevVer, err := d.mostRecentVer(cfg.ConfigDir)
	if err != nil {
		if errors.Is(err, ErrNoRecentVersion) {
			return dateref.Now(0).String(), nil
		} else {
			return "", err
		}
	}

	ref, err := dateref.Parse(prevVer)
	if err != nil {
		// probably should prefix this error as it actually should never happen
		return "", ErrBadDatestamp
	}

	if ref.IsToday() {
		return ref.NextCounter().String(), nil
	} else {
		return dateref.Now(0).String(), nil
	}
}

func (d Datestamp) Generate(cfg *project.Project, opts *Options) (*Release, error) {
	// TODO: Probably should refactor this logic out of release strategy
	//   since they work exactly the same w all others
	// ensure we have clean worktree (so we don't accidentally release something that's not
	// already in a commit)
	status, err := gitcmd.Status(cfg.ConfigDir)
	if err != nil {
	} else if status != "" && !opts.Force {
		return nil, ErrDirtyWorkdir
	}

	nextVer := opts.Name
	if nextVer == "" {
		if nv, err := d.NextName(cfg, NameAny); err != nil {
			return nil, err
		} else {
			nextVer = nv
		}
	}

	prevVer, err := d.mostRecentVer(cfg.ConfigDir)
	if err != nil {
		if errors.Is(err, ErrNoRecentVersion) {
			prevVer = ""
		} else {
			return nil, err
		}
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

	return &Release{
		Name:    nextVer,
		Message: generateMessage(cfg, opts.Name, refs),
		Commits: refs,
	}, nil
}

func (d Datestamp) Create(cfg *project.Project, rel *Release) error {
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

func (d Datestamp) mostRecentVer(wd string) (string, error) {
	raw, err := gitcmd.ListTags(wd)
	if err != nil {
		return "", err
	}

	tags := strings.Split(raw, "\n")
	if len(tags) == 0 {
		return "", ErrNoRecentVersion
	}

	sort.Strings(tags)
	mostRecent := tags[len(tags)-1]
	if !dateref.IsValid(mostRecent) {
		return "", ErrNoRecentVersion
	} else {
		return mostRecent, nil
	}
}
