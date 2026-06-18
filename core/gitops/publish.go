package gitops

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content"
)

// Flux media types for the rendered-manifest config artifact. The OCIRepository
// + kustomize-controller in Slice 2 consume this exact shape, so we mirror what
// `flux push artifact` produces rather than inventing our own.
const (
	FluxConfigMediaType = "application/vnd.cncf.flux.config.v1+json"
	FluxLayerMediaType  = "application/vnd.cncf.flux.content.v1.tar+gzip"
)

// Publish packages a rendered tree from Render as a Flux-shaped OCI artifact and
// pushes it into target under tag — the moving per-env tag Flux's OCIRepository
// follows. Each tree entry lands at its <component>/<filename> path inside the
// layer tarball; kustomize-controller applies every YAML doc it extracts.
func Publish(ctx context.Context, target oras.Target, tag string, tree Tree) (ocispec.Descriptor, error) {
	layer, err := tarGzip(tree)
	if err != nil {
		return ocispec.Descriptor{}, err
	}

	config := []byte("{}")
	configDesc := content.NewDescriptorFromBytes(FluxConfigMediaType, config)
	if err := target.Push(ctx, configDesc, bytes.NewReader(config)); err != nil {
		return ocispec.Descriptor{}, err
	}

	layerDesc := content.NewDescriptorFromBytes(FluxLayerMediaType, layer)
	if err := target.Push(ctx, layerDesc, bytes.NewReader(layer)); err != nil {
		return ocispec.Descriptor{}, err
	}

	opts := oras.PackManifestOptions{
		ConfigDescriptor: &configDesc,
		Layers:           []ocispec.Descriptor{layerDesc},
	}
	manifestDesc, err := oras.PackManifest(ctx, target, oras.PackManifestVersion1_1, FluxConfigMediaType, opts)
	if err != nil {
		return ocispec.Descriptor{}, err
	}

	if err := target.Tag(ctx, manifestDesc, tag); err != nil {
		return ocispec.Descriptor{}, err
	}
	return manifestDesc, nil
}

// tarGzip wraps the rendered tree in a gzipped tarball — the payload shape Flux
// extracts and applies. Entries are written in sorted path order with fixed
// header fields (no mtime, no uid) so an identical tree yields an identical
// digest.
func tarGzip(tree Tree) ([]byte, error) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)

	for _, rel := range tree.Paths() {
		body := tree[rel]
		header := &tar.Header{Name: rel, Mode: 0o644, Size: int64(len(body))}
		if err := tw.WriteHeader(header); err != nil {
			return nil, err
		}
		if _, err := tw.Write(body); err != nil {
			return nil, err
		}
	}

	if err := tw.Close(); err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
