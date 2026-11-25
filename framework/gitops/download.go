package gitops

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	lua "github.com/yuin/gopher-lua"
)

// download is platform.download (docs/spec/lua-render.md, "Upstream manifests"): it
// fetches one https URL, uncached, and returns the YAML stream there as a Lua list of
// documents, empty sequences marked as lists with the app's own defs.tree mark.
func download(modules Modules) lua.LGFunction {
	return func(state *lua.LState) int {
		url := state.CheckString(1)

		docs, err := downloadDocuments(state, modules, url)
		if err != nil {
			state.RaiseError("platform.download %s: %s", url, err.Error())
		}
		state.Push(docs)
		return 1
	}
}

func downloadDocuments(state *lua.LState, modules Modules, url string) (*lua.LTable, error) {
	if !strings.HasPrefix(url, "https://") {
		return nil, errors.New("want an https:// URL")
	}

	body, err := fetch(url)
	if err != nil {
		return nil, err
	}
	mark, err := listMark(state, modules)
	if err != nil {
		return nil, err
	}
	return decodeDocuments(state, body, mark)
}

// fetch is the one network call render makes.
func fetch(url string) ([]byte, error) {
	resp, err := downloadClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}
