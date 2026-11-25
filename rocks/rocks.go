// Package rocks fetches pinned Lua rocks from ghcr and keeps each version unpacked in the
// XDG cache (docs/spec/lua-render.md, "Rocks"). A rock is a one-layer OCI image at
// ghcr.io/prod9/<name>, tagged by its version.
package rocks

import (
	"net/http"
	"regexp"
	"time"
)

const (
	ociLayerType    = "application/vnd.oci.image.layer.v1.tar+gzip"
	dockerLayerType = "application/vnd.docker.image.rootfs.diff.tar.gzip"
	manifestTypes   = "application/vnd.oci.image.manifest.v1+json, application/vnd.oci.image.index.v1+json"
)

var (
	// registryBase is the registry seam: swapped in tests, never at runtime.
	registryBase = "https://ghcr.io"
	// The rocks are under 1 MB and ghcr answered each request of the spike that measured
	// them in well under a second.
	registry = &http.Client{Timeout: 10 * time.Second}

	// The OCI distribution grammar for one repository path component, a tag and a sha256
	// digest. Each lands in a cache path or a registry URL, and none admits a "/" or a
	// leading ".".
	namePattern    = regexp.MustCompile(`^[a-z0-9]+((\.|_|__|-+)[a-z0-9]+)*$`)
	versionPattern = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9._-]{0,127}$`)
	digestPattern  = regexp.MustCompile(`^sha256:[a-f0-9]{64}$`)
)
