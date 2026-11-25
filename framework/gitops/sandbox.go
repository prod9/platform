package gitops

import (
	"errors"
	"fmt"

	lua "github.com/yuin/gopher-lua"
)

var (
	// The base globals an app keeps (docs/spec/lua-render.md, "The VM and its sandbox").
	// Every other global is removed, so nothing reaches files, processes, the clock or
	// randomness.
	sandboxGlobals = map[string]bool{
		"assert": true, "error": true, "getmetatable": true, "ipairs": true, "next": true,
		"pairs": true, "pcall": true, "rawget": true, "select": true, "setmetatable": true, "tonumber": true,
		"tostring": true, "type": true, "string": true, "table": true, "math": true,
	}
)

// NewSandbox builds the VM state one app runs in: the base, table, string and math
// libraries, math without random and randomseed, only the allowed base globals, the
// platform table over vars, and platform's require, resolving against modules.
// The caller closes the state.
func NewSandbox(modules Modules, vars Vars) (*lua.LState, error) {
	state := lua.NewState(lua.Options{SkipOpenLibs: true})
	err := openLibraries(state)
	if err != nil {
		state.Close()
		return nil, err
	}
	removeGlobals(state)
	state.SetGlobal("platform", platformTable(state, modules, vars))
	state.SetGlobal("require", state.NewFunction(requireFrom(modules)))
	return state, nil
}

func openLibraries(state *lua.LState) error {
	libraries := []struct {
		name string
		open lua.LGFunction
	}{
		{lua.BaseLibName, lua.OpenBase},
		{lua.TabLibName, lua.OpenTable},
		{lua.StringLibName, lua.OpenString},
		{lua.MathLibName, lua.OpenMath},
	}
	for _, library := range libraries {
		opener := lua.P{Fn: state.NewFunction(library.open), NRet: 0, Protect: true}
		err := state.CallByParam(opener, lua.LString(library.name))
		if err != nil {
			return fmt.Errorf("open Lua library %q: %w", library.name, err)
		}
	}

	mathLib, isTable := state.GetGlobal(lua.MathLibName).(*lua.LTable)
	if !isTable {
		return errors.New("open Lua library \"math\": no math table")
	}
	mathLib.RawSetString("random", lua.LNil)
	mathLib.RawSetString("randomseed", lua.LNil)
	return nil
}

func removeGlobals(state *lua.LState) {
	globals := state.Get(lua.GlobalsIndex).(*lua.LTable)
	var dropped []lua.LValue
	globals.ForEach(func(name, _ lua.LValue) {
		if !sandboxGlobals[lua.LVAsString(name)] {
			dropped = append(dropped, name)
		}
	})
	for _, name := range dropped {
		globals.RawSet(name, lua.LNil)
	}
}

// requireFrom is platform's require. Each module runs once per state: loaded keeps what
// it returned, nil included, so later calls return the same value.
func requireFrom(modules Modules) lua.LGFunction {
	loaded := map[string]lua.LValue{}
	return func(state *lua.LState) int {
		name := state.CheckString(1)
		if cached, done := loaded[name]; done {
			state.Push(cached)
			return 1
		}

		path, err := modules.find(name)
		if err != nil {
			state.RaiseError("%s", err.Error())
		}
		chunk, err := state.LoadFile(path)
		if err != nil {
			state.RaiseError("%s", err.Error())
		}
		state.Push(chunk)
		state.Call(0, 1)
		module := state.Get(-1)
		state.Pop(1)

		loaded[name] = module
		state.Push(module)
		return 1
	}
}
