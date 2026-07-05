package gitops

import (
	"errors"
	"strings"

	fxconfig "fx.prodigy9.co/config"
	"oras.land/oras-go/v2/registry"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/credentials"
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

// registryClient authenticates pushes, preferring REGISTRY_USERNAME/PASSWORD from the
// environment and otherwise falling back to the docker credential store (config.json +
// OS keychain) — the same creds `publish` uses via Dagger, so a local push needs no env
// vars. Returns nil (oras's anonymous default) only when neither source has anything.
func registryClient(host string) remote.Client {
	client := &auth.Client{
		Client: retry.DefaultClient,
		Cache:  auth.NewCache(),
	}

	cfg := fxconfig.Configure()
	if username := fxconfig.Get(cfg, RegistryUsernameConfig); username != "" {
		client.Credential = auth.StaticCredential(host, auth.Credential{
			Username: username,
			Password: fxconfig.Get(cfg, RegistryPasswordConfig),
		})
		return client
	}

	store, err := credentials.NewStoreFromDocker(credentials.StoreOptions{})
	if err != nil {
		return nil
	}
	client.Credential = credentials.Credential(store)
	return client
}
