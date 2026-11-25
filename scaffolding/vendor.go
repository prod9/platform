package scaffolding

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/mod/semver"
	fwscaffold "platform.prodigy9.co/framework/scaffold"
	"platform.prodigy9.co/git"
)

var (
	ErrNoCommit       = errors.New("scaffold: the current branch has no commit; commit, then re-run init")
	ErrTrackedChanges = errors.New("scaffold: tracked files have changes; commit them, then re-run init")
	ErrNoReleaseTag   = errors.New("scaffold: no release tag (vMAJOR.MINOR.PATCH[-PRE])")
	ErrNoSubtree      = errors.New("scaffold: git subtree is missing from this git")
)

// Vendoring is one library init vendors as a git subtree (docs/spec/scaffolding.md,
// step 3): the framework's Vendor and the release tag resolved for it.
type Vendoring struct {
	dir    string
	Vendor fwscaffold.Vendor
	Tag    string
}

// Vendorings resolves every library the target's framework vendors that the repo lacks;
// an existing prefix is skipped and never bumped. When any is missing, the repo must
// meet git subtree add's precondition first. Reads only.
func (t *Target) Vendorings(ctx context.Context) ([]Vendoring, error) {
	missing, err := t.missingVendors()
	if err != nil || len(missing) == 0 {
		return nil, err
	}
	if err := subtreePrecondition(ctx, t.dir); err != nil {
		return nil, err
	}

	vendorings := make([]Vendoring, 0, len(missing))
	for _, vendor := range missing {
		tag, err := newestReleaseTag(ctx, t.dir, vendor.URL)
		if err != nil {
			return nil, err
		}
		vendorings = append(vendorings, Vendoring{dir: t.dir, Vendor: vendor, Tag: tag})
	}
	return vendorings, nil
}

func (t *Target) missingVendors() ([]fwscaffold.Vendor, error) {
	if t.framework == nil {
		return nil, nil
	}

	var missing []fwscaffold.Vendor
	for _, vendor := range t.framework.Vendors() {
		_, err := os.Stat(filepath.Join(t.dir, filepath.FromSlash(vendor.Prefix)))
		if errors.Is(err, fs.ErrNotExist) {
			missing = append(missing, vendor)
			continue
		}
		if err != nil {
			return nil, err
		}
	}
	return missing, nil
}

// subtreePrecondition holds the repo to what git subtree add needs: a commit on the
// current branch and no staged or unstaged change to a tracked file.
func subtreePrecondition(ctx context.Context, dir string) error {
	head, err := git.Head(ctx, dir)
	if err != nil {
		return err
	}
	if head == "" {
		return ErrNoCommit
	}

	changed, err := git.Run(ctx, dir, "status", "--porcelain", "--untracked-files=no")
	if err != nil {
		return err
	}
	if changed != "" {
		return fmt.Errorf("%w:\n%s", ErrTrackedChanges, changed)
	}
	return nil
}

// newestReleaseTag is the highest SemVer precedence among url's tags in the
// vMAJOR.MINOR.PATCH[-PRE] form, prereleases included.
func newestReleaseTag(ctx context.Context, dir, url string) (string, error) {
	refs, err := git.Run(ctx, dir, "ls-remote", "--tags", "--refs", url)
	if err != nil {
		return "", err
	}

	newest := ""
	for _, line := range strings.Split(refs, "\n") {
		_, ref, found := strings.Cut(line, "\t")
		tag := strings.TrimPrefix(ref, "refs/tags/")
		if !found || !isReleaseTag(tag) {
			continue
		}
		if newest == "" || semver.Compare(tag, newest) > 0 {
			newest = tag
		}
	}
	if newest == "" {
		return "", fmt.Errorf("%w at %s", ErrNoReleaseTag, url)
	}
	return newest, nil
}

// isReleaseTag admits the full vMAJOR.MINOR.PATCH[-PRE] form only: semver also accepts
// the v1 and v1.2 shorthands and build metadata, which are not release tags.
func isReleaseTag(tag string) bool {
	return semver.IsValid(tag) && semver.Canonical(tag) == tag
}

// Command is the git command Run executes, for the confirmation that precedes it.
func (v Vendoring) Command() []string {
	return []string{"git", "subtree", "add", "--prefix=" + v.Vendor.Prefix, v.Vendor.URL, v.Tag, "--squash"}
}

// Run adds the library as a squashed git subtree: a merge commit on the current branch,
// the only commit init makes.
func (v Vendoring) Run(ctx context.Context) error {
	_, err := git.Run(ctx, v.dir, v.Command()[1:]...)
	if err != nil && strings.Contains(err.Error(), "'subtree' is not a git command") {
		return fmt.Errorf("%w: %w", ErrNoSubtree, err)
	}
	return err
}
