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

	// manifestsFilename names the single multi-doc file inside the layer
	// tarball; kustomize-controller applies every YAML doc it extracts.
	manifestsFilename = "manifests.yaml"
)

// Publish packages manifests (a multi-doc YAML stream from Render) as a
// Flux-shaped OCI artifact and pushes it into target under tag — the moving
// per-env tag Flux's OCIRepository follows.
func Publish(ctx context.Context, target oras.Target, tag, manifests string) (ocispec.Descriptor, error) {
	layer, err := tarGzip(manifests)
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

// tarGzip wraps the manifest stream in a single-entry gzipped tarball — the
// payload shape Flux extracts and applies. Header fields are fixed (no mtime,
// no uid) so identical manifests yield an identical digest.
func tarGzip(manifests string) ([]byte, error) {
	body := []byte(manifests)

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)

	header := &tar.Header{Name: manifestsFilename, Mode: 0o644, Size: int64(len(body))}
	if err := tw.WriteHeader(header); err != nil {
		return nil, err
	}
	if _, err := tw.Write(body); err != nil {
		return nil, err
	}

	if err := tw.Close(); err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
