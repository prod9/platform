package gitops

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"cuelang.org/go/mod/module"
	"github.com/stretchr/testify/require"
	"platform.prodigy9.co/cuemod"
)

// stubRegistry answers from a fixed requirement graph and records what was fetched. The
// walk is what these tests pin — the real registry's downloading is smoke's business, and
// reaching the network here would make the suite neither fast nor hermetic.
type stubRegistry struct {
	reqs    map[string][]module.Version
	fetched []string
	failOn  string
}

var errStubFetch = errors.New("stub fetch failed")

func (r *stubRegistry) Fetch(_ context.Context, mv module.Version) (module.SourceLoc, error) {
	if mv.String() == r.failOn {
		return module.SourceLoc{}, errStubFetch
	}
	r.fetched = append(r.fetched, mv.String())
	return module.SourceLoc{}, nil
}

func (r *stubRegistry) Requirements(_ context.Context, mv module.Version) ([]module.Version, error) {
	return r.reqs[mv.String()], nil
}

func (r *stubRegistry) ModuleVersions(context.Context, string) ([]string, error) {
	return nil, nil
}

func writeModuleFile(t *testing.T, deps string) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "cue.mod"), 0755))
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, cuemod.ModuleFile),
		[]byte("module: \"test.example\"\nlanguage: version: \"v0.15.4\"\ndeps: {"+deps+"}\n"),
		0644,
	))
	return dir
}

func mustVersion(t *testing.T, path, version string) module.Version {
	t.Helper()
	mv, err := module.NewVersion(path, version)
	require.NoError(t, err)
	return mv
}

func TestFetchDepsWalksTransitively(t *testing.T) {
	dir := writeModuleFile(t, `"prodigy9.co/defs@v0": v: "v0.4.3"`)

	direct := mustVersion(t, "prodigy9.co/defs@v0", "v0.4.3")
	indirect := mustVersion(t, "example.com/deep@v1", "v1.2.0")
	registry := &stubRegistry{reqs: map[string][]module.Version{
		direct.String(): {indirect},
	}}

	require.NoError(t, fetchDeps(context.Background(), dir, registry))
	require.Equal(t, []string{direct.String(), indirect.String()}, registry.fetched)
}

func TestFetchDepsVisitsEachModuleOnce(t *testing.T) {
	dir := writeModuleFile(t, `
		"prodigy9.co/defs@v0": v: "v0.4.3"
		"example.com/other@v1": v: "v1.0.0"
	`)

	defs := mustVersion(t, "prodigy9.co/defs@v0", "v0.4.3")
	other := mustVersion(t, "example.com/other@v1", "v1.0.0")
	shared := mustVersion(t, "example.com/shared@v1", "v1.0.0")

	// Both direct deps require the same module — a diamond, which is the shape that makes
	// an unguarded walk fetch twice (and cycle forever if the graph ever closes on itself).
	registry := &stubRegistry{reqs: map[string][]module.Version{
		defs.String():  {shared},
		other.String(): {shared},
	}}

	require.NoError(t, fetchDeps(context.Background(), dir, registry))
	require.Len(t, registry.fetched, 3)
	require.Contains(t, registry.fetched, shared.String())
}

func TestFetchDepsReportsAFailedFetch(t *testing.T) {
	dir := writeModuleFile(t, `"prodigy9.co/defs@v0": v: "v0.4.3"`)
	registry := &stubRegistry{failOn: mustVersion(t, "prodigy9.co/defs@v0", "v0.4.3").String()}

	err := fetchDeps(context.Background(), dir, registry)
	require.ErrorIs(t, err, errStubFetch)
}

func TestFetchDepsRejectsAMissingModuleFile(t *testing.T) {
	err := fetchDeps(context.Background(), t.TempDir(), &stubRegistry{})
	require.ErrorIs(t, err, os.ErrNotExist)
}
