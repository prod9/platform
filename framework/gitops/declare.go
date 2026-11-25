package gitops

import (
	"fmt"
	"math"
	"path/filepath"

	lua "github.com/yuin/gopher-lua"
)

// DeclaredVars runs the Lua file at rel under repoDir in declare-only mode
// (docs/spec/scaffolding.md, "The Infra framework scaffolds from vendored infra-lua"): it
// stops at the file's first platform.vars call and returns the names and defaults
// declared there, as TOML-ready Go values. A file that returns without the call declares
// none.
func DeclaredVars(repoDir, rel string) (map[string]any, error) {
	modules, err := LoadModules(repoDir)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", rel, err)
	}
	state, err := newBareSandbox(modules)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", rel, err)
	}
	defer state.Close()

	var declared *lua.LTable
	platform := state.NewTable()
	platform.RawSetString("vars", state.NewFunction(func(state *lua.LState) int {
		if declared == nil {
			declared = state.CheckTable(1)
		}
		state.RaiseError("platform.vars: declare-only run stops here")
		return 0
	}))
	state.SetGlobal("platform", platform)

	chunk, err := state.LoadFile(filepath.Join(repoDir, rel))
	if err != nil {
		return nil, err
	}
	state.Push(chunk)
	err = state.PCall(0, 0, nil)
	if declared == nil && err != nil {
		return nil, err
	}

	return tomlDefaults(rel, declared)
}

// tomlDefaults converts declared defaults to the Go values the TOML encoder writes: a
// string, a boolean, an integer for an integral number and a float otherwise.
func tomlDefaults(rel string, declared *lua.LTable) (map[string]any, error) {
	defaults := map[string]any{}
	if declared == nil {
		return defaults, nil
	}

	for name, value := declared.Next(lua.LNil); name != lua.LNil; name, value = declared.Next(name) {
		key, isString := name.(lua.LString)
		if !isString {
			return nil, fmt.Errorf("%s: platform.vars: want var names as keys, got a %s key", rel, name.Type())
		}
		switch value := value.(type) {
		case lua.LString:
			defaults[string(key)] = string(value)
		case lua.LBool:
			defaults[string(key)] = bool(value)
		case lua.LNumber:
			defaults[string(key)] = tomlNumber(float64(value))
		default:
			return nil, fmt.Errorf("%s: platform.vars: the default for %s is a %s, which has no TOML form", rel, key, value.Type())
		}
	}
	return defaults, nil
}

func tomlNumber(n float64) any {
	if n == math.Trunc(n) && math.Abs(n) < 1<<53 {
		return int64(n)
	}
	return n
}
