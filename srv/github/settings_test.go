package github

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRegistryTokenRoundtrip(t *testing.T) {
	ctx := connectDB(t)
	migrateSettings(t, ctx)

	require.NoError(t, SaveRegistryToken(ctx, "ghcr.io", "ghp_token"))

	token, err := LoadRegistryToken(ctx, "ghcr.io")
	require.NoError(t, err)
	require.Equal(t, "ghp_token", token)
}

func TestOrgRoundtrip(t *testing.T) {
	ctx := connectDB(t)
	migrateSettings(t, ctx)

	require.NoError(t, SaveOrg(ctx, "prod9"))

	org, err := LoadOrg(ctx)
	require.NoError(t, err)
	require.Equal(t, "prod9", org)
}

// The org slug heads the settings-backed wizard steps — an unset one is its own
// sentinel so the org check can reach not-started
// (docs/spec/installation.md, the state surface).
func TestLoadOrgAbsent(t *testing.T) {
	ctx := connectDB(t)
	migrateSettings(t, ctx)

	_, err := LoadOrg(ctx)
	require.ErrorIs(t, err, ErrNoOrg)
}

// A token saved for one host says nothing about another — the keys are host-keyed
// so more registries can join without a schema change
// (docs/spec/installation.md, "The registry token").
func TestLoadRegistryTokenAbsent(t *testing.T) {
	ctx := connectDB(t)
	migrateSettings(t, ctx)

	require.NoError(t, SaveRegistryToken(ctx, "ghcr.io", "ghp_token"))

	_, err := LoadRegistryToken(ctx, "docker.io")
	require.ErrorIs(t, err, ErrNoRegistryToken)
}
