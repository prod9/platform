package srv

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// The dependency rule of docs/spec/platform-server.md: the shared packages are leaves
// and must never import server concerns, and only the serve command wires srv in.
// go/parser in ImportsOnly mode keeps this hermetic and fast.

var sharedPackages = []string{
	"../conf", "../cuemod", "../framework", "../engine", "../gitops",
	"../releases", "../git", "../internal",
}

var serverPackages = []string{"platform.prodigy9.co/srv", "platform.prodigy9.co/webui"}

func TestSharedPackagesNeverImportServer(t *testing.T) {
	for _, pkg := range sharedPackages {
		for file, imports := range packageImports(t, pkg) {
			for _, imported := range imports {
				for _, banned := range serverPackages {
					require.NotEqual(t, banned, imported,
						"%s imports %s across the shared→server boundary", file, imported)
				}
			}
		}
	}
}

func TestOnlyServeCmdImportsSrv(t *testing.T) {
	for file, imports := range packageImports(t, "../cmd") {
		if filepath.Base(file) == "serve.go" {
			continue
		}
		for _, imported := range imports {
			require.NotEqual(t, "platform.prodigy9.co/srv", imported,
				"%s imports srv; only cmd/serve.go may", file)
		}
	}
}

// packageImports maps every .go file under dir (recursively) to its import paths.
func packageImports(t *testing.T, dir string) map[string][]string {
	imports := map[string][]string{}

	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.HasSuffix(path, ".go") {
			return err
		}

		parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, spec := range parsed.Imports {
			imported, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				return err
			}
			imports[path] = append(imports[path], imported)
		}
		return nil
	})
	require.NoError(t, err)

	return imports
}
