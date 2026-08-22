package cmd

import (
	"fmt"
	"runtime/debug"

	"github.com/spf13/cobra"
)

var VersionsCmd = &cobra.Command{
	Use:   "versions",
	Short: "Print this binary's version and the engine versions baked into it",
	Run:   runVersionsCmd,
}

func runVersionsCmd(cmd *cobra.Command, args []string) {
	info, ok := debug.ReadBuildInfo()
	fmt.Print(versionsTable(info, ok))
}

// versionString reports the raw module stamp `go install module@version` wrote —
// verbatim, pseudo-versions included; a between-releases build must never report the
// release it descends from (that derivation is framework.PlatformVersion's, for
// stamping new launchers).
func versionString(info *debug.BuildInfo, ok bool) string {
	if !ok || info.Main.Version == "" {
		return "(devel)"
	}
	return info.Main.Version
}

// versionsTable adds the baked-in versions that change build/render behavior: the
// dagger SDK (what engine provisioning pairs to) and the linked CUE evaluator, plus
// the Go toolchain stamp.
func versionsTable(info *debug.BuildInfo, ok bool) string {
	goVersion := "(unknown)"
	if ok && info.GoVersion != "" {
		goVersion = info.GoVersion
	}

	return fmt.Sprintf("platform %s\ndagger   %s\ncue      %s\ngo       %s\n",
		versionString(info, ok),
		depVersion(info, ok, "dagger.io/dagger"),
		depVersion(info, ok, "cuelang.org/go"),
		goVersion)
}

func depVersion(info *debug.BuildInfo, ok bool, path string) string {
	if !ok {
		return "(unknown)"
	}

	for _, dep := range info.Deps {
		if dep.Path != path {
			continue
		}
		if dep.Replace != nil {
			return dep.Replace.Version
		}
		return dep.Version
	}
	return "(unknown)"
}
