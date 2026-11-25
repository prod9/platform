package gitops_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"platform.prodigy9.co/framework/gitops"
)

// docs/spec/lua-render.md, "Output contract": each apps/<component>.lua returns file name
// to documents; a value is one resource or a list of them.
func TestRenderWritesComponentFiles(t *testing.T) {
	repoDir := t.TempDir()
	writeRepoFile(t, repoDir, "apps/web.lua", `
return {
  ["deploy.yaml"] = {apiVersion = "apps/v1", kind = "Deployment", metadata = {name = "web"},
    spec = {replicas = 2}},
  ["routes.yaml"] = {
    {apiVersion = "v1", kind = "Service", metadata = {name = "web"}},
    {apiVersion = "v1", kind = "ConfigMap", metadata = {name = "web"}},
  },
}`)

	tree, err := gitops.Render(repoDir, nil)
	require.NoError(t, err)
	require.Equal(t, []string{"web/deploy.yaml", "web/routes.yaml"}, tree.Paths())
	require.Equal(t, `apiVersion: apps/v1
kind: Deployment
metadata:
    name: web
spec:
    replicas: 2
`, string(tree["web/deploy.yaml"]))
	require.Equal(t, `apiVersion: v1
kind: Service
metadata:
    name: web
---
apiVersion: v1
kind: ConfigMap
metadata:
    name: web
`, string(tree["web/routes.yaml"]))
}

// docs/spec/lua-render.md, "platform.vars": the call returns exactly the declared names,
// each the [vars] value when set and the default otherwise; names match as written.
func TestRenderVarsDeclaredWithDefaults(t *testing.T) {
	repoDir := t.TempDir()
	writeRepoFile(t, repoDir, "apps/web.lua", `
local vars = platform.vars {APP_VERSION = "v1", APP_PORT = 80, app_name = "default"}
local undeclared = vars.OTHER == nil and "absent" or "present"
return {
  ["config.yaml"] = {apiVersion = "v1", kind = "ConfigMap", metadata = {name = "web"},
    data = {version = vars.APP_VERSION, port = vars.APP_PORT, name = vars.app_name,
      other = undeclared}},
}`)
	vars := map[string]any{"APP_VERSION": "v9", "APP_NAME": "set", "OTHER": true}

	tree, err := gitops.Render(repoDir, vars)
	require.NoError(t, err)
	require.Equal(t, `apiVersion: v1
kind: ConfigMap
metadata:
    name: web
data:
    name: default
    other: absent
    port: 80
    version: v9
`, string(tree["web/config.yaml"]))
}

func TestRenderRejectsNonScalarVarBeforeApps(t *testing.T) {
	repoDir := t.TempDir()
	writeRepoFile(t, repoDir, "apps/web.lua", `error("the app ran")`)

	_, err := gitops.Render(repoDir, map[string]any{"HOSTS": []any{"a", "b"}})
	require.ErrorContains(t, err, "HOSTS")
	require.NotContains(t, err.Error(), "the app ran")
}

// docs/spec/lua-render.md, "Pipeline": a module name two vendored libraries both provide
// fails the render before any app runs.
func TestRenderRejectsModuleInTwoVendoredLibraries(t *testing.T) {
	repoDir := newFixtureRepo(t)
	writeRepoFile(t, repoDir, "vendor/other/src/defs/tree.lua", `return {}`)
	writeRepoFile(t, repoDir, "apps/web.lua", `error("the app ran")`)

	_, err := gitops.Render(repoDir, nil)
	require.ErrorContains(t, err, `"defs.tree"`)
	require.ErrorContains(t, err, filepath.Join("vendor", "infra-lua"))
	require.ErrorContains(t, err, filepath.Join("vendor", "other"))
	require.NotContains(t, err.Error(), "the app ran")
}

// docs/spec/lua-render.md, "YAML": the list mark comes from defs.tree in the app's own
// state, whichever root it resolves from, so an empty marked table is an empty sequence
// and an unmarked one a mapping.
func TestRenderEmptyListFromTreeMark(t *testing.T) {
	roots := map[string]func(t *testing.T) string{
		"vendored": newFixtureRepo,
		"repo": func(t *testing.T) string {
			repoDir := t.TempDir()
			writeRepoFile(t, repoDir, "defs/tree.lua", `local List = {}
return { list = function(items) return setmetatable(items, List) end }`)
			return repoDir
		},
	}
	for name, newRepo := range roots {
		t.Run(name, func(t *testing.T) {
			repoDir := newRepo(t)
			writeRepoFile(t, repoDir, "apps/web.lua", `
local tree = require "defs.tree"
return {
  ["config.yaml"] = {apiVersion = "v1", kind = "List", metadata = {name = "web"},
    items = tree.list({}), labels = {}},
}`)

			tree, err := gitops.Render(repoDir, nil)
			require.NoError(t, err)
			require.Equal(t, `apiVersion: v1
kind: List
metadata:
    name: web
items: []
labels: {}
`, string(tree["web/config.yaml"]))
		})
	}
}

// docs/spec/lua-render.md, "The VM and its sandbox": an app's error names its file and line.
func TestRenderAppErrorNamesFileAndLine(t *testing.T) {
	repoDir := t.TempDir()
	writeRepoFile(t, repoDir, "apps/web.lua", "local x = 1\nerror(\"boom\")\n")

	_, err := gitops.Render(repoDir, nil)
	require.ErrorContains(t, err, filepath.Join("apps", "web.lua")+":2")
	require.ErrorContains(t, err, "boom")
}

// docs/spec/lua-render.md, "Output contract": an app that returns anything else fails.
func TestRenderRejectsMalformedReturns(t *testing.T) {
	cases := map[string]string{
		"not a table":      `return "web.yaml"`,
		"file name a path": `return {["../web.yaml"] = {kind = "x"}}`,
		"list of files":    `return {{kind = "x"}}`,
		"document a value": `return {["web.yaml"] = "x"}`,
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			repoDir := t.TempDir()
			writeRepoFile(t, repoDir, "apps/web.lua", src)

			_, err := gitops.Render(repoDir, nil)
			require.ErrorContains(t, err, "web.lua")
		})
	}
}

func TestRenderWithoutAppsIsEmpty(t *testing.T) {
	tree, err := gitops.Render(t.TempDir(), nil)
	require.NoError(t, err)
	require.Empty(t, tree)
}

func TestTreeWriteDir(t *testing.T) {
	tree := gitops.Tree{"web/a.yaml": []byte("a: 1\n"), "db/b.yaml": []byte("b: 2\n")}
	out := t.TempDir()

	require.NoError(t, tree.WriteDir(out))
	for rel, want := range tree {
		got, err := os.ReadFile(filepath.Join(out, rel))
		require.NoError(t, err)
		require.Equal(t, string(want), string(got))
	}
}
