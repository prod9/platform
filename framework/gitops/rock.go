package gitops

import (
	"fmt"
	"path/filepath"

	lua "github.com/yuin/gopher-lua"
)

// Rock is one pinned rock as require sees it: each module its rockspec lists under
// build.modules, mapped to that module's file in the rock's cached directory.
type Rock struct {
	modules map[string]string
}

// LoadRock reads the one rockspec at the root of rockDir, the directory rocks.Fetch
// returns. A rockspec is a Lua file of plain assignments, so it runs in a VM with no
// libraries at all.
func LoadRock(rockDir string) (Rock, error) {
	matches, err := filepath.Glob(filepath.Join(rockDir, "*.rockspec"))
	if err != nil {
		return Rock{}, err
	}
	if len(matches) != 1 {
		return Rock{}, fmt.Errorf("%s: want exactly one rockspec, found %d", rockDir, len(matches))
	}
	rockspec := matches[0]

	state := lua.NewState(lua.Options{SkipOpenLibs: true})
	defer state.Close()
	err = state.DoFile(rockspec)
	if err != nil {
		return Rock{}, fmt.Errorf("rockspec %s: %w", rockspec, err)
	}
	build, isTable := state.GetGlobal("build").(*lua.LTable)
	if !isTable {
		return Rock{}, fmt.Errorf("rockspec %s: no build table", rockspec)
	}
	listed, isTable := build.RawGetString("modules").(*lua.LTable)
	if !isTable {
		return Rock{}, fmt.Errorf("rockspec %s: no build.modules table", rockspec)
	}

	modules := map[string]string{}
	for name, file := listed.Next(lua.LNil); name != lua.LNil; name, file = listed.Next(name) {
		path, fileErr := moduleFile(rockDir, file)
		if fileErr != nil {
			return Rock{}, fmt.Errorf("rockspec %s: build.modules[%v]: %w", rockspec, name, fileErr)
		}
		modules[lua.LVAsString(name)] = path
	}
	return Rock{modules: modules}, nil
}

// moduleFile admits only a relative path that stays inside the rock, so a rockspec cannot
// point require at a file elsewhere on the machine.
func moduleFile(rockDir string, file lua.LValue) (string, error) {
	relPath, isString := file.(lua.LString)
	if !isString {
		return "", fmt.Errorf("want a file path, got a %s", file.Type())
	}
	localPath := filepath.FromSlash(string(relPath))
	if !filepath.IsLocal(localPath) {
		return "", fmt.Errorf("%q is not a file inside the rock", string(relPath))
	}
	return filepath.Join(rockDir, localPath), nil
}

// anyLists reports whether one of rocks lists module under build.modules.
func anyLists(rocks []Rock, module string) bool {
	for _, rock := range rocks {
		if _, listed := rock.modules[module]; listed {
			return true
		}
	}
	return false
}
