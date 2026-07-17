package cmd

import (
	"runtime/debug"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestVersionString(t *testing.T) {
	cases := []struct {
		name     string
		info     *debug.BuildInfo
		ok       bool
		expected string
	}{
		{
			name:     "release stamp verbatim",
			info:     stubBuildInfo("v0.9.12"),
			ok:       true,
			expected: "v0.9.12",
		},
		{
			name:     "pseudo-version stamp verbatim",
			info:     stubBuildInfo("v0.9.13-0.20260717031415-abcdef123456"),
			ok:       true,
			expected: "v0.9.13-0.20260717031415-abcdef123456",
		},
		{
			name:     "no module stamp",
			info:     stubBuildInfo(""),
			ok:       true,
			expected: "(devel)",
		},
		{
			name:     "no build info at all",
			info:     nil,
			ok:       false,
			expected: "(devel)",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			require.Equal(t, c.expected, versionString(c.info, c.ok))
		})
	}
}

func TestVersionsTable(t *testing.T) {
	info := stubBuildInfo("v0.9.12",
		dep("dagger.io/dagger", "v0.21.7"),
		dep("cuelang.org/go", "v0.15.4"))
	info.GoVersion = "go1.25.5"

	expected := "platform v0.9.12\n" +
		"dagger   v0.21.7\n" +
		"cue      v0.15.4\n" +
		"go       go1.25.5\n"
	require.Equal(t, expected, versionsTable(info, true))
}

func TestVersionsTableReplacedDep(t *testing.T) {
	replaced := dep("dagger.io/dagger", "v0.21.7")
	replaced.Replace = dep("dagger.io/dagger", "v0.21.8")
	info := stubBuildInfo("v0.9.12", replaced)
	info.GoVersion = "go1.25.5"

	require.Contains(t, versionsTable(info, true), "dagger   v0.21.8\n")
}

func TestVersionsTableUnlinkedDeps(t *testing.T) {
	info := stubBuildInfo("")
	info.GoVersion = "go1.25.5"

	expected := "platform (devel)\n" +
		"dagger   (unknown)\n" +
		"cue      (unknown)\n" +
		"go       go1.25.5\n"
	require.Equal(t, expected, versionsTable(info, true))
}

func stubBuildInfo(version string, deps ...*debug.Module) *debug.BuildInfo {
	return &debug.BuildInfo{Main: debug.Module{Version: version}, Deps: deps}
}

func dep(path, version string) *debug.Module {
	return &debug.Module{Path: path, Version: version}
}
