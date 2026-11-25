package scaffolding

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	r "github.com/stretchr/testify/require"
	"platform.prodigy9.co/framework"
	fwscaffold "platform.prodigy9.co/framework/scaffold"
)

// vendoringFramework is a framework that needs one library vendored at vendor/lib.
type vendoringFramework struct {
	framework.Dockerfile
	url string
}

func (f vendoringFramework) Vendors() []fwscaffold.Vendor {
	return []fwscaffold.Vendor{{Prefix: "vendor/lib", URL: f.url}}
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	r.NoError(t, err, string(out))
	return strings.TrimSpace(string(out))
}

// isolateGit keeps the operator's git config out of the test's commits, platform's own
// git calls included.
func isolateGit(t *testing.T) {
	t.Helper()
	env := map[string]string{
		"GIT_AUTHOR_NAME": "t", "GIT_AUTHOR_EMAIL": "t@example.com",
		"GIT_COMMITTER_NAME": "t", "GIT_COMMITTER_EMAIL": "t@example.com",
		"GIT_CONFIG_GLOBAL": os.DevNull, "GIT_CONFIG_NOSYSTEM": "1",
	}
	for name, value := range env {
		t.Setenv(name, value)
	}
}

func initRepo(t *testing.T, dir string) {
	t.Helper()
	isolateGit(t)
	r.NoError(t, os.MkdirAll(dir, 0o755))
	runGit(t, dir, "init", "-q", "-b", "main")
}

func commitFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	r.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	r.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	runGit(t, dir, "add", name)
	runGit(t, dir, "commit", "-q", "-m", "add "+name)
}

// library is an upstream git repo carrying tags; the tag named newest holds the file
// src/newest.lua, so a test can see which tag was vendored.
func library(t *testing.T, tags []string, newest string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "lib")
	initRepo(t, dir)
	for _, tag := range tags {
		name := "src/" + strings.NewReplacer(".", "_", "-", "_").Replace(tag) + ".lua"
		if tag == newest {
			name = "src/newest.lua"
		}
		commitFile(t, dir, name, "return {}\n")
		runGit(t, dir, "tag", tag)
	}
	return dir
}

func vendoringTarget(t *testing.T, url string) *Target {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "app")
	initRepo(t, dir)
	return &Target{dir: dir, framework: vendoringFramework{url: url}}
}

// docs/spec/scaffolding.md, step 3: the newest tag is the highest SemVer precedence,
// prereleases included, among valid vMAJOR.MINOR.PATCH[-PRE] tags; the subtree add is a
// commit of its own. Untracked files do not block it.
func TestVendorings_addsNewestTagAsSubtree(t *testing.T) {
	lib := library(t, []string{"v0.1.0", "v0.1.10", "v0.2.0-rc.1", "v0.1.9", "v1", "latest"}, "v0.2.0-rc.1")
	target := vendoringTarget(t, lib)
	commitFile(t, target.dir, "README.md", "app\n")
	r.NoError(t, os.WriteFile(filepath.Join(target.dir, "scratch.txt"), []byte("x"), 0o644))

	vendorings, err := target.Vendorings(context.Background())
	r.NoError(t, err)
	r.Len(t, vendorings, 1)
	r.Equal(t, "v0.2.0-rc.1", vendorings[0].Tag)

	r.NoError(t, vendorings[0].Run(context.Background()))
	r.FileExists(t, filepath.Join(target.dir, "vendor", "lib", "src", "newest.lua"))
	parents := strings.Fields(runGit(t, target.dir, "rev-list", "--parents", "-1", "HEAD"))
	r.Len(t, parents, 3, "the subtree add is a merge commit")
}

// An existing prefix is skipped and never bumped, so no precondition applies.
func TestVendorings_skipsExistingPrefix(t *testing.T) {
	target := vendoringTarget(t, library(t, []string{"v0.1.0"}, "v0.1.0"))
	r.NoError(t, os.MkdirAll(filepath.Join(target.dir, "vendor", "lib"), 0o755))

	vendorings, err := target.Vendorings(context.Background())
	r.NoError(t, err)
	r.Empty(t, vendorings)
}

func TestVendorings_preconditionsAndTags(t *testing.T) {
	cases := map[string]struct {
		tags  []string
		setup func(t *testing.T, dir string)
		want  error
		names string
	}{
		"no commit": {
			tags:  []string{"v0.1.0"},
			setup: func(*testing.T, string) {},
			want:  ErrNoCommit,
		},
		"changed tracked file": {
			tags: []string{"v0.1.0"},
			setup: func(t *testing.T, dir string) {
				commitFile(t, dir, "README.md", "app\n")
				r.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"), []byte("edited\n"), 0o644))
			},
			want:  ErrTrackedChanges,
			names: "README.md",
		},
		"no release tag": {
			tags:  []string{"v1", "latest"},
			setup: func(t *testing.T, dir string) { commitFile(t, dir, "README.md", "app\n") },
			want:  ErrNoReleaseTag,
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			target := vendoringTarget(t, library(t, c.tags, ""))
			c.setup(t, target.dir)

			_, err := target.Vendorings(context.Background())
			r.ErrorIs(t, err, c.want)
			r.ErrorContains(t, err, c.names)
		})
	}
}
