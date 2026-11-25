package gitops

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	lua "github.com/yuin/gopher-lua"
)

const (
	appsDir = "apps"

	// treeModule is the infra-lua module whose list function sets the list mark.
	treeModule = "defs.tree"
)

// Render runs every apps/<component>.lua under repoDir in a sandbox of its own and turns
// what each returns into that component's files (docs/spec/lua-render.md, "Pipeline").
// Nothing is written: a failure at any step leaves no partial tree.
func Render(repoDir string, vars map[string]any) (Tree, error) {
	luaVars, err := NewVars(vars)
	if err != nil {
		return nil, fmt.Errorf("render: %w", err)
	}
	modules, err := LoadModules(repoDir)
	if err != nil {
		return nil, fmt.Errorf("render: %w", err)
	}
	apps, err := appFiles(filepath.Join(repoDir, appsDir))
	if err != nil {
		return nil, fmt.Errorf("render: %w", err)
	}

	tree := Tree{}
	for _, app := range apps {
		files, appErr := renderApp(repoDir, app, modules, luaVars)
		if appErr != nil {
			return nil, fmt.Errorf("render: %w", appErr)
		}
		component := strings.TrimSuffix(app, luaExt)
		for name, content := range files {
			tree[component+"/"+name] = content
		}
	}
	return tree, nil
}

// appFiles lists the app files directly under dir in name order. A repo with no apps/
// directory has no apps.
func appFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var names []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), luaExt) {
			names = append(names, entry.Name())
		}
	}
	return names, nil
}

// renderApp runs one app in a fresh sandbox and encodes the files it returns.
func renderApp(repoDir, app string, modules Modules, vars Vars) (map[string][]byte, error) {
	appPath := filepath.Join(appsDir, app)
	state, err := NewSandbox(modules, vars)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", appPath, err)
	}
	defer state.Close()

	chunk, err := state.LoadFile(filepath.Join(repoDir, appPath))
	if err != nil {
		return nil, err
	}
	state.Push(chunk)
	err = state.PCall(0, 1, nil)
	if err != nil {
		return nil, err
	}
	returned := state.Get(-1)
	state.Pop(1)

	mark, err := listMark(state, modules)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", appPath, err)
	}
	files, err := encodeFiles(state, returned, mark)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", appPath, err)
	}
	return files, nil
}

// listMark is infra-lua's list metatable as the app's own state loaded it
// (docs/spec/lua-render.md, "YAML"), whichever root defs.tree resolves from. When it
// resolves nowhere there is no mark, and nil leaves every empty table a mapping.
func listMark(state *lua.LState, modules Modules) (*lua.LTable, error) {
	_, err := modules.find(treeModule)
	if errors.Is(err, errNoModule) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	err = state.DoString(`return getmetatable(require("` + treeModule + `").list({}))`)
	if err != nil {
		return nil, fmt.Errorf("take the list mark from %s: %w", treeModule, err)
	}
	mark := state.Get(-1)
	state.Pop(1)
	table, isTable := mark.(*lua.LTable)
	if !isTable {
		return nil, fmt.Errorf("%s.list set no metatable", treeModule)
	}
	return table, nil
}

// encodeFiles converts an app's return value, file name to documents, into file contents.
func encodeFiles(state *lua.LState, returned lua.LValue, mark *lua.LTable) (map[string][]byte, error) {
	table, isTable := returned.(*lua.LTable)
	if !isTable {
		return nil, fmt.Errorf("want a table of file name to documents, got a %s", returned.Type())
	}

	files := map[string][]byte{}
	for key, value := table.Next(lua.LNil); key != lua.LNil; key, value = table.Next(key) {
		name, err := fileName(key)
		if err != nil {
			return nil, err
		}
		content, err := EncodeDocuments(documents(state, value, mark), mark)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		files[name] = content
	}
	return files, nil
}

// fileName admits a key that names one file inside the component directory.
func fileName(key lua.LValue) (string, error) {
	name, isString := key.(lua.LString)
	if !isString {
		return "", fmt.Errorf("want file names as keys, got a %s key", key.Type())
	}
	if !filepath.IsLocal(string(name)) || filepath.Base(string(name)) != string(name) {
		return "", fmt.Errorf("%q is not a file name", string(name))
	}
	return string(name), nil
}

// documents is a file's value as a list of documents: a list (a pack, a plain list, or a
// marked empty list) as it is, and any other value as a list of that one value.
func documents(state *lua.LState, value lua.LValue, mark *lua.LTable) lua.LValue {
	table, isTable := value.(*lua.LTable)
	if isTable && (table.RawGetInt(1) != lua.LNil || (mark != nil && table.Metatable == lua.LValue(mark))) {
		return table
	}

	single := state.NewTable()
	single.Append(value)
	return single
}
