package framework

import (
	"runtime/debug"

	"golang.org/x/mod/semver"
)

// PlatformVersion reports the binary's valid SemVer verbatim for launcher pins.
// Missing or invalid build versions return empty, which init treats as a hard error.
func PlatformVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	return platformVersionFromModule(info.Main.Version)
}

func platformVersionFromModule(version string) string {
	if !semver.IsValid(version) {
		return ""
	}
	return version
}
