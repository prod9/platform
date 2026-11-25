package gitops_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	lua "github.com/yuin/gopher-lua"
	"platform.prodigy9.co/framework/gitops"
)

// newTestSandbox is a sandbox over repoDir's modules.
func newTestSandbox(t *testing.T, repoDir string) *lua.LState {
	t.Helper()
	modules, err := gitops.LoadModules(repoDir)
	require.NoError(t, err)
	state, err := gitops.NewSandbox(modules, gitops.Vars{})
	require.NoError(t, err)
	t.Cleanup(state.Close)
	return state
}

// newFixtureRepo is a fresh repo holding testdata/repo: infra-lua vendored at
// vendor/infra-lua with a defs.tree whose list sets a metatable.
func newFixtureRepo(t *testing.T) string {
	t.Helper()
	repoDir := t.TempDir()
	require.NoError(t, os.CopyFS(repoDir, os.DirFS(filepath.Join("testdata", "repo"))))
	return repoDir
}

func writeRepoFile(t *testing.T, repoDir, name, content string) {
	t.Helper()
	path := filepath.Join(repoDir, filepath.FromSlash(name))
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

// evalLua runs src and returns its one result.
func evalLua(t *testing.T, state *lua.LState, src string) lua.LValue {
	t.Helper()
	require.NoError(t, state.DoString(src))
	value := state.Get(-1)
	state.Pop(1)
	return value
}

// docs/spec/lua-render.md, "The VM and its sandbox": no files, processes, clock,
// randomness, or way to load code except require.
func TestSandboxRemovesForbiddenGlobals(t *testing.T) {
	state := newTestSandbox(t, t.TempDir())
	forbidden := []string{
		"os", "io", "debug", "coroutine", "package", "load", "loadstring", "loadfile",
		"dofile", "print", "math.random", "math.randomseed",
	}
	for _, name := range forbidden {
		require.Equal(t, lua.LNil, evalLua(t, state, "return "+name), name)
	}
	require.Equal(t, lua.LString("function"), evalLua(t, state, "return type(math.floor)"))
}

// docs/spec/lua-render.md, "`require`": vendor/<lib>/src/<path>.lua first, the repo's
// <path>.lua second.
func TestRequireFindsVendoredModuleBeforeRepo(t *testing.T) {
	repoDir := newFixtureRepo(t)
	writeRepoFile(t, repoDir, "defs/tree.lua", `return { from = "repo" }`)
	state := newTestSandbox(t, repoDir)

	from := evalLua(t, state, `return require("defs.tree").from`)
	require.Equal(t, lua.LString("vendor"), from, "a vendored module comes before the repo's")
}

func TestRequireFindsRepoModule(t *testing.T) {
	repoDir := t.TempDir()
	writeRepoFile(t, repoDir, "lib/shared.lua", `return { from = "repo" }`)
	state := newTestSandbox(t, repoDir)

	from := evalLua(t, state, `return require("lib.shared").from`)
	require.Equal(t, lua.LString("repo"), from)
}

func TestRequireMissingNamesEveryPathSearched(t *testing.T) {
	repoDir := newFixtureRepo(t)
	writeRepoFile(t, repoDir, "vendor/other/src/other.lua", `return {}`)
	state := newTestSandbox(t, repoDir)

	err := state.DoString(`require "defs.absent"`)
	for _, root := range []string{"vendor/infra-lua/src", "vendor/other/src", ""} {
		require.ErrorContains(t, err, filepath.Join(repoDir, filepath.FromSlash(root), "defs", "absent.lua"))
	}
}

func TestRequireRunsModuleOnce(t *testing.T) {
	repoDir := t.TempDir()
	writeRepoFile(t, repoDir, "counter.lua", "runs = (runs or 0) + 1\nreturn {}")
	state := newTestSandbox(t, repoDir)

	src := `local first = require "counter"
local second = require "counter"
return runs, first == second`
	require.NoError(t, state.DoString(src))
	require.Equal(t, lua.LNumber(1), state.Get(-2))
	require.Equal(t, lua.LTrue, state.Get(-1))
}
