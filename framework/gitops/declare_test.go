package gitops_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"platform.prodigy9.co/framework/gitops"
)

// docs/spec/scaffolding.md, "The Infra framework scaffolds from vendored infra-lua": a
// template runs until its first platform.vars call, and that call's names and defaults
// are what it declares; nothing after the call runs.
func TestDeclaredVarsStopAtFirstCall(t *testing.T) {
	repoDir := newFixtureRepo(t)
	writeRepoFile(t, repoDir, "templates/cert-manager.lua", `
local tree = require "defs.tree"
local vars = platform.vars {CERT_MANAGER_VERSION = "v1.20.2", REPLICAS = 2, RATIO = 0.5, ENABLED = true}
platform.vars {LATER = "never"}
error("ran past the declaration")`)

	declared, err := gitops.DeclaredVars(repoDir, "templates/cert-manager.lua")
	require.NoError(t, err)
	require.Equal(t, map[string]any{
		"CERT_MANAGER_VERSION": "v1.20.2",
		"REPLICAS":             int64(2),
		"RATIO":                0.5,
		"ENABLED":              true,
	}, declared)
}

// A template that returns without calling platform.vars declares none.
func TestDeclaredVarsNoneWithoutCall(t *testing.T) {
	repoDir := t.TempDir()
	writeRepoFile(t, repoDir, "templates/defaults.lua", `return {}`)

	declared, err := gitops.DeclaredVars(repoDir, "templates/defaults.lua")
	require.NoError(t, err)
	require.Empty(t, declared)
}

// docs/spec/lua-render.md, "platform.vars": a default with no TOML form fails, and so
// does a template that errors before declaring.
func TestDeclaredVarsFailures(t *testing.T) {
	cases := map[string]string{
		"table default": `platform.vars {HOSTS = {"a"}}`,
		"error first":   `error("boom")`,
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			repoDir := t.TempDir()
			writeRepoFile(t, repoDir, "templates/web.lua", src)

			_, err := gitops.DeclaredVars(repoDir, "templates/web.lua")
			require.ErrorContains(t, err, "web.lua")
		})
	}
}
