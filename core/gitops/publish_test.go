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

// sampleTree is a two-component render output: each file lands at its
// <component>/<filename> path inside the published layer tarball.
var sampleTree = gitops.Tree{
	"gateway/gateway.yaml": []byte("apiVersion: v1\nkind: Gateway\nmetadata:\n  name: gw\n"),
	"demo/deploy.yaml":     []byte("apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: demo\n"),
}

// TestPublishRoundTrip pushes a rendered tree as a Flux-shaped OCI artifact into
// a filesystem oci.Store, then pulls the layer back out and asserts every
// component file survives byte-for-byte at its tree path and the media types
// match what Flux's OCIRepository expects to consume in Slice 2.
func TestPublishRoundTrip(t *testing.T) {
	ctx := context.Background()
	store, err := oci.New(t.TempDir())
	if err != nil {
		t.Fatalf("oci.New: %v", err)
	}

	const tag = "staging"
	if _, err := gitops.Publish(ctx, store, tag, sampleTree); err != nil {
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

	files := extractLayer(t, ctx, store, manifest.Layers[0])
	for rel, want := range sampleTree {
		if got, ok := files[rel]; !ok {
			t.Errorf("layer missing %q", rel)
		} else if got != string(want) {
			t.Errorf("%s round-trip mismatch:\n got: %q\nwant: %q", rel, got, want)
		}
	}
	if len(files) != len(sampleTree) {
		t.Errorf("layer has %d files, want %d", len(files), len(sampleTree))
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

// extractLayer ungzips and untars the layer, returning every entry keyed by its
// tar path (the tree's <component>/<filename>).
func extractLayer(t *testing.T, ctx context.Context, store *oci.Store, layer ocispec.Descriptor) map[string]string {
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

	files := map[string]string{}
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("tar next: %v", err)
		}
		body, err := io.ReadAll(tr)
		if err != nil {
			t.Fatalf("read %s: %v", header.Name, err)
		}
		files[header.Name] = string(body)
	}
	return files
}
