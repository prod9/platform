package builder

import (
	"runtime"
	"testing"

	r "github.com/stretchr/testify/require"
)

func TestResolveArch(t *testing.T) {
	host := "linux/" + runtime.GOARCH

	// "auto" tracks the host arch — fast native local builds.
	r.Equal(t, host, resolveArch("auto"))
	r.Equal(t, host, resolveArch("AUTO"))

	// a bare arch becomes linux/<arch> — these containers are always linux, so a
	// publish build pins amd64 to match the servers regardless of the build host.
	r.Equal(t, "linux/amd64", resolveArch("amd64"))
	r.Equal(t, "linux/arm64", resolveArch("arm64"))

	// a full platform string (the deprecated `platform` key / PLATFORM env) is
	// honored verbatim.
	r.Equal(t, "linux/amd64", resolveArch("linux/amd64"))
}
