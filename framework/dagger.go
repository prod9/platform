package framework

import "runtime/debug"

// DaggerVersion reports the dagger SDK version this binary is linked against. Empty when
// build info is unavailable or the dagger module isn't linked; init treats empty as a hard
// error rather than emitting an engine ref with no tag.
func DaggerVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	return daggerVersion(info)
}

// daggerVersion finds the linked SDK's version. The in-cluster engine image must track it:
// a freshly-init'd infra repo pins `registry.dagger.io/engine:<version>` to whatever this
// platform binary links, so the engine and the SDK driving it never drift apart.
func daggerVersion(info *debug.BuildInfo) string {
	for _, dep := range info.Deps {
		if dep.Path != "dagger.io/dagger" {
			continue
		}
		if dep.Replace != nil {
			return dep.Replace.Version
		}
		return dep.Version
	}
	return ""
}
