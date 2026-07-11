package framework

import "runtime/debug"

// daggerModule is the SDK module whose version the in-cluster engine image must track:
// a freshly-init'd infra repo pins `registry.dagger.io/engine:<version>` to whatever this
// platform binary is linked against, so the engine and the SDK driving it never drift apart.
const daggerModule = "dagger.io/dagger"

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

func daggerVersion(info *debug.BuildInfo) string {
	for _, dep := range info.Deps {
		if dep.Path != daggerModule {
			continue
		}
		if dep.Replace != nil {
			return dep.Replace.Version
		}
		return dep.Version
	}
	return ""
}
