package releases

import (
	"errors"
	"sort"
	"strings"

	"platform.prodigy9.co/gitcmd"
	"platform.prodigy9.co/project"
	"platform.prodigy9.co/releases/timeref"
)

var (
	ErrBadTimestamp = errors.New("release name is not timestamp")
)

type Timestamp struct{}

var _ Strategy = Timestamp{}

func (d Timestamp) List(cfg *project.Project) ([]*Release, error) {
	lines, err := gitcmd.ListTags(cfg.ConfigDir)
	if err != nil {
		return nil, err
	}

	var result []*Release
	for _, line := range strings.Split(lines, "\n") {
		if timeref.IsValid(line) {
			result = append(result, &Release{Name: line})
		}
	}
	return result, nil
}

func (d Timestamp) Recover(cfg *project.Project, opts *Options) (*Release, error) {
	// ensure the local wd has all the up-to-date tags
	if branch, err := gitcmd.CurrentBranch(cfg.ConfigDir); err != nil {
		return nil, err
	} else if remote, err := gitcmd.TrackingRemote(cfg.ConfigDir, branch); err != nil {
		return nil, err
	} else if _, err := gitcmd.FetchTags(cfg.ConfigDir, remote); err != nil {
		return nil, err
	}

	// get annotated tag and name
	if opts.Name == "" {
		tagname, err := gitcmd.Describe(cfg.ConfigDir)
		if err != nil {
			return nil, err
		} else if !timeref.IsValid(tagname) {
			return nil, ErrBadTimestamp
		}

		opts.Name = tagname
	}

	tagmsg, err := gitcmd.TagMessage(cfg.ConfigDir, opts.Name)
	if err != nil {
		return nil, err
	}

	return &Release{Name: opts.Name, Message: tagmsg}, nil
}

func (d Timestamp) NextName(cfg *project.Project, comp NameComponent) (string, error) {
	return timeref.Now(), nil
}

func (d Timestamp) Generate(cfg *project.Project, opts *Options) (*Release, error) {
	if opts.Name == "" {
		opts.Name = timeref.Now()
	}

	prevVer, err := d.mostRecentVer(cfg.ConfigDir)
	if err != nil {
		return nil, err
	}

	// TODO: Probably should refactor this logic out of release strategy
	//   since they work exactly the same w semver.
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

	return &Release{
		Name:    opts.Name,
		Message: generateMessage(cfg, opts.Name, refs),
		Commits: refs,
	}, nil
}

func (d Timestamp) Create(cfg *project.Project, rel *Release) error {
	// always fetch remote tags before making changes because someone else might have
	// pushed a tag since we last fetched (or you yourself might have pushed a tag from
	// another machine and forgot)
	if branch, err := gitcmd.CurrentBranch(cfg.ConfigDir); err != nil {
		return err
	} else if remote, err := gitcmd.TrackingRemote(cfg.ConfigDir, branch); err != nil {
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

func (d Timestamp) mostRecentVer(wd string) (string, error) {
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
	if !timeref.IsValid(mostRecent) {
		return "", ErrNoRecentVersion
	} else {
		return mostRecent, nil
	}
}
