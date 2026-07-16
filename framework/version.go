package framework

import (
	"regexp"
	"runtime/debug"
	"strconv"
	"strings"
)

// PlatformVersion reports the release version scaffolded launchers pin, resolved to the
// nearest release this binary descends from. Empty when no release is derivable — init
// treats that as a hard error rather than emitting a launcher pinned to nothing.
func PlatformVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	return platformVersionFromModule(info.Main.Version)
}

// exactRelease matches the only tag shape the semver strategy cuts; pseudoVersion matches
// what the toolchain stamps on a between-releases build: the next patch plus a timestamp
// and hash, from which the predecessor release is recovered. A dirty tree appends +dirty
// to either shape — stripped before matching.
var (
	exactRelease  = regexp.MustCompile(`^v\d+\.\d+\.\d+$`)
	pseudoVersion = regexp.MustCompile(`^(v\d+\.\d+\.)(\d+)-0\.\d{14}-[0-9a-f]{12}$`)
)

func platformVersionFromModule(version string) string {
	version = strings.TrimSuffix(version, "+dirty")
	if exactRelease.MatchString(version) {
		return version
	}

	m := pseudoVersion.FindStringSubmatch(version)
	if m == nil {
		return ""
	}
	patch, err := strconv.Atoi(m[2])
	if err != nil || patch == 0 {
		return ""
	}
	return m[1] + strconv.Itoa(patch-1)
}
