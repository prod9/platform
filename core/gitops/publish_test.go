package gitops_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"testing"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/content/oci"
	"platform.prodigy9.co/core/gitops"
)

const sampleManifests = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: demo
---
apiVersion: v1
kind: Service
metadata:
  name: demo
`

// TestPublishRoundTrip pushes the rendered manifests as a Flux-shaped OCI
// artifact into a filesystem oci.Store, then pulls every layer back out and
// asserts the manifests survive byte-for-byte and the media types match what
// Flux's OCIRepository expects to consume in Slice 2.
func TestPublishRoundTrip(t *testing.T) {
	ctx := context.Background()
	store, err := oci.New(t.TempDir())
	if err != nil {
		t.Fatalf("oci.New: %v", err)
	}

	const tag = "staging"
	if _, err := gitops.Publish(ctx, store, tag, sampleManifests); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	manifest := resolveManifest(t, ctx, store, tag)
	if got := manifest.Config.MediaType; got != gitops.FluxConfigMediaType {
		t.Errorf("config media type = %q, want %q", got, gitops.FluxConfigMediaType)
	}
	if n := len(manifest.Layers); n != 1 {
		t.Fatalf("layers = %d, want 1", n)
	}
	if got := manifest.Layers[0].MediaType; got != gitops.FluxLayerMediaType {
		t.Errorf("layer media type = %q, want %q", got, gitops.FluxLayerMediaType)
	}

	if got := extractManifests(t, ctx, store, manifest.Layers[0]); got != sampleManifests {
		t.Errorf("round-tripped manifests mismatch:\n got: %q\nwant: %q", got, sampleManifests)
	}
}

func resolveManifest(t *testing.T, ctx context.Context, store *oci.Store, tag string) ocispec.Manifest {
	t.Helper()
	desc, err := store.Resolve(ctx, tag)
	if err != nil {
		t.Fatalf("Resolve %q: %v", tag, err)
	}
	raw, err := content.FetchAll(ctx, store, desc)
	if err != nil {
		t.Fatalf("fetch manifest: %v", err)
	}

	var manifest ocispec.Manifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatalf("unmarshal manifest: %v", err)
	}
	return manifest
}

func extractManifests(t *testing.T, ctx context.Context, store *oci.Store, layer ocispec.Descriptor) string {
	t.Helper()
	raw, err := content.FetchAll(ctx, store, layer)
	if err != nil {
		t.Fatalf("fetch layer: %v", err)
	}

	gz, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("gzip: %v", err)
	}
	tr := tar.NewReader(gz)
	if _, err := tr.Next(); err != nil {
		t.Fatalf("tar next: %v", err)
	}

	body, err := io.ReadAll(tr)
	if err != nil {
		t.Fatalf("read tar body: %v", err)
	}
	return string(body)
}
