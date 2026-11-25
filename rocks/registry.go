package rocks

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

type ociManifest struct {
	Manifests []ociDescriptor `json:"manifests"`
	Layers    []ociDescriptor `json:"layers"`
}

type ociDescriptor struct {
	MediaType string `json:"mediaType"`
	Digest    string `json:"digest"`
}

// pull fetches the rock's one layer and checks it against its digest: token, manifest
// (through an index when the tag names one), then the layer.
func (f Fetch) pull(ctx context.Context) ([]byte, error) {
	repository := "prod9/" + f.Name
	token, err := f.token(ctx, repository)
	if err != nil {
		return nil, err
	}
	manifest, err := fetchManifest(ctx, repository, f.Version, token)
	if err != nil {
		return nil, err
	}
	if len(manifest.Manifests) > 0 {
		child := manifest.Manifests[0].Digest
		if !digestPattern.MatchString(child) {
			return nil, fmt.Errorf("index entry digest %q is not a sha256 digest", child)
		}
		manifest, err = fetchManifest(ctx, repository, child, token)
		if err != nil {
			return nil, err
		}
	}

	if len(manifest.Layers) != 1 {
		return nil, fmt.Errorf("want a one-layer image, manifest has %d layers", len(manifest.Layers))
	}
	layer := manifest.Layers[0]
	if layer.MediaType != ociLayerType && layer.MediaType != dockerLayerType {
		return nil, fmt.Errorf("layer has media type %q, want a tar+gzip layer", layer.MediaType)
	}
	if !digestPattern.MatchString(layer.Digest) {
		return nil, fmt.Errorf("layer digest %q is not a sha256 digest", layer.Digest)
	}
	blob, err := get(ctx, registryBase+"/v2/"+repository+"/blobs/"+layer.Digest, http.Header{
		"Authorization": {"Bearer " + token},
	})
	if err != nil {
		return nil, err
	}

	sum := sha256.Sum256(blob)
	if "sha256:"+hex.EncodeToString(sum[:]) != layer.Digest {
		return nil, fmt.Errorf("layer does not match its digest %s", layer.Digest)
	}
	return blob, nil
}

// token asks ghcr for a pull token, sending the credentials when there are any.
func (f Fetch) token(ctx context.Context, repository string) (string, error) {
	header := http.Header{}
	if f.Username != "" {
		login := base64.StdEncoding.EncodeToString([]byte(f.Username + ":" + f.Password))
		header.Set("Authorization", "Basic "+login)
	}
	body, err := get(ctx, registryBase+"/token?service=ghcr.io&scope=repository:"+repository+":pull", header)
	if err != nil {
		return "", err
	}

	var grant struct {
		Token string `json:"token"`
	}
	err = json.Unmarshal(body, &grant)
	if err != nil {
		return "", fmt.Errorf("read pull token: %w", err)
	}
	if grant.Token == "" {
		return "", errors.New("registry granted no pull token")
	}
	return grant.Token, nil
}

func fetchManifest(ctx context.Context, repository, reference, token string) (ociManifest, error) {
	body, err := get(ctx, registryBase+"/v2/"+repository+"/manifests/"+reference, http.Header{
		"Authorization": {"Bearer " + token},
		"Accept":        {manifestTypes},
	})
	if err != nil {
		return ociManifest{}, err
	}

	var manifest ociManifest
	err = json.Unmarshal(body, &manifest)
	if err != nil {
		return ociManifest{}, fmt.Errorf("read manifest %s: %w", reference, err)
	}
	return manifest, nil
}

func get(ctx context.Context, url string, header http.Header) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	request.Header = header

	response, err := registry.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", url, response.Status)
	}
	return io.ReadAll(response.Body)
}
