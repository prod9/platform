package builds

import (
	"testing"

	"github.com/stretchr/testify/require"
	"platform.prodigy9.co/conf"
)

// The publish credential is looked up by the registry the image names — the host is
// the image name's first path segment (docs/spec/platform-server.md, "The publish
// credential is the wizard-saved registry token").
func TestRegistryHost(t *testing.T) {
	host, err := registryHost(&conf.Model{Modules: map[string]*conf.Module{
		"app": {ImageName: "ghcr.io/prod9/platform"},
		"web": {ImageName: "ghcr.io/prod9/platform/web"},
	}})
	require.NoError(t, err)
	require.Equal(t, "ghcr.io", host)
}

// Modules naming more than one registry in one build is unsupported — the session
// holds one credential, so the mix fails the run rather than half-publishing.
func TestRegistryHostMixedHostsFails(t *testing.T) {
	_, err := registryHost(&conf.Model{Modules: map[string]*conf.Module{
		"app": {ImageName: "ghcr.io/prod9/platform"},
		"web": {ImageName: "docker.io/prod9/web"},
	}})
	require.Error(t, err)
}

// No image name means no registry to credential against — a server build exists to
// publish, so an unset image is a failure, not a skip.
func TestRegistryHostNoImageFails(t *testing.T) {
	_, err := registryHost(&conf.Model{Modules: map[string]*conf.Module{
		"app": {},
	}})
	require.Error(t, err)
}
