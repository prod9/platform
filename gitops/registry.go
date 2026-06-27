package gitops

import (
	"errors"
	"strings"

	fxconfig "fx.prodigy9.co/config"
	"oras.land/oras-go/v2/registry"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/retry"
)

// Push credentials, sourced from the agent environment — same env contract as
// builder's REGISTRY_USERNAME/PASSWORD. The registry host comes from the push
// reference itself, so there is no separate REGISTRY var here. See the secrets-
// system flag in docs/notes/2026-06-17-slice1-open-questions.md.
var (
	RegistryUsernameConfig = fxconfig.Str("REGISTRY_USERNAME")
	RegistryPasswordConfig = fxconfig.Str("REGISTRY_PASSWORD")
)

var ErrNoTag = errors.New("gitops: push reference is missing a :tag")

// RemoteRepository resolves an oci://host/repo:tag reference into an
// authenticated push target and the moving per-env tag to publish under.
func RemoteRepository(ref string) (*remote.Repository, string, error) {
	parsed, err := registry.ParseReference(strings.TrimPrefix(ref, "oci://"))
	if err != nil {
		return nil, "", err
	}
	if parsed.Reference == "" {
		return nil, "", ErrNoTag
	}

	repo, err := remote.NewRepository(parsed.Registry + "/" + parsed.Repository)
	if err != nil {
		return nil, "", err
	}
	repo.Client = registryClient(parsed.Registry)

	return repo, parsed.Reference, nil
}

// registryClient returns an authenticated client when credentials are present
// in the environment, or nil to fall back to oras's anonymous default.
func registryClient(host string) remote.Client {
	cfg := fxconfig.Configure()
	username := fxconfig.Get(cfg, RegistryUsernameConfig)
	if username == "" {
		return nil
	}

	return &auth.Client{
		Client: retry.DefaultClient,
		Cache:  auth.NewCache(),
		Credential: auth.StaticCredential(host, auth.Credential{
			Username: username,
			Password: fxconfig.Get(cfg, RegistryPasswordConfig),
		}),
	}
}
