package gitops

import (
	"fmt"

	lua "github.com/yuin/gopher-lua"
)

// Vars is the [vars] table as Lua values, the names kept as platform.toml spells them.
type Vars map[string]lua.LValue

// NewVars admits the TOML value types docs/spec/lua-render.md "platform.vars" lists:
// strings, integers, floats and booleans. Any other value fails, so a render stops before
// any app runs.
func NewVars(vars map[string]any) (Vars, error) {
	converted := make(Vars, len(vars))
	for name, value := range vars {
		switch value := value.(type) {
		case string:
			converted[name] = lua.LString(value)
		case bool:
			converted[name] = lua.LBool(value)
		case int64:
			converted[name] = lua.LNumber(value)
		case float64:
			converted[name] = lua.LNumber(value)
		default:
			return nil, fmt.Errorf("[vars] %s: a %T has no Lua form; want a string, number or boolean", name, value)
		}
	}
	return converted, nil
}

// platformTable is the `platform` global an app sees.
func platformTable(state *lua.LState, vars Vars) *lua.LTable {
	table := state.NewTable()
	table.RawSetString("vars", state.NewFunction(declareVars(vars)))
	return table
}

// declareVars is platform.vars: it takes the declared names with their defaults and
// returns exactly those names, each holding its [vars] value when set and its default
// otherwise.
func declareVars(vars Vars) lua.LGFunction {
	return func(state *lua.LState) int {
		declared := state.CheckTable(1)

		values := state.NewTable()
		for name, fallback := declared.Next(lua.LNil); name != lua.LNil; name, fallback = declared.Next(name) {
			key, isString := name.(lua.LString)
			if !isString {
				state.RaiseError("platform.vars: want var names as keys, got a %s key", name.Type())
			}
			fromVars, isSet := vars[string(key)]
			if isSet {
				values.RawSet(key, fromVars)
			} else {
				values.RawSet(key, fallback)
			}
		}

		state.Push(values)
		return 1
	}
}
