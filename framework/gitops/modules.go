package gitops

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	vendorDir = "vendor"
	vendorSrc = "src"
)

var (
	// Lua's dotted module-name form: lowercase identifiers joined by dots. It admits no
	// "/" and no "..", so a name maps to a path inside a module root.
	modulePattern = regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)*$`)
)

// Modules is what require resolves names against in one repo (docs/spec/lua-render.md,
// "`require`"): the modules of every library vendored under vendor/, then the repo's own
// files.
type Modules struct {
	repoDir string

	// srcDirs are the libraries' src/ roots in library-name order, which a failed lookup
	// lists as the paths it searched.
	srcDirs  []string
	vendored map[string]string
}

// LoadModules indexes the modules of every library vendored under repoDir/vendor/. A
// module name two libraries both provide fails here, before any app runs, so directory
// order never picks one.
func LoadModules(repoDir string) (Modules, error) {
	libs, err := os.ReadDir(filepath.Join(repoDir, vendorDir))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return Modules{}, err
	}

	modules := Modules{repoDir: repoDir, vendored: map[string]string{}}
	for _, lib := range libs {
		if !lib.IsDir() {
			continue
		}
		srcDir := filepath.Join(repoDir, vendorDir, lib.Name(), vendorSrc)
		provided, libErr := libraryModules(srcDir)
		if libErr != nil {
			return Modules{}, libErr
		}
		for name, path := range provided {
			taken, isTaken := modules.vendored[name]
			if isTaken {
				return Modules{}, fmt.Errorf("module %q is provided by both %s and %s", name, taken, path)
			}
			modules.vendored[name] = path
		}
		modules.srcDirs = append(modules.srcDirs, srcDir)
	}
	return modules, nil
}

// libraryModules maps each module name one library's src/ root provides to its file. A
// library with no src/ provides none. A file whose path is not a dotted module name, such
// as a.b.lua, is one no require can reach, so it is left out.
func libraryModules(srcDir string) (map[string]string, error) {
	modules := map[string]string{}
	err := filepath.WalkDir(srcDir, func(path string, entry fs.DirEntry, walkErr error) error {
		if path == srcDir && errors.Is(walkErr, fs.ErrNotExist) {
			return fs.SkipAll
		}
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(path, luaExt) {
			return nil
		}

		rel, err := filepath.Rel(srcDir, strings.TrimSuffix(path, luaExt))
		if err != nil {
			return err
		}
		slashed := filepath.ToSlash(rel)
		name := strings.ReplaceAll(slashed, "/", ".")
		if strings.Contains(slashed, ".") || !modulePattern.MatchString(name) {
			return nil
		}
		modules[name] = path
		return nil
	})
	return modules, err
}

// find is the file a module name resolves to. A name found nowhere fails with
// errNoModule, naming every path searched.
func (m Modules) find(name string) (string, error) {
	if !modulePattern.MatchString(name) {
		return "", fmt.Errorf("require %q: not a module name (lowercase identifiers joined by dots)", name)
	}

	vendoredPath, isVendored := m.vendored[name]
	if isVendored {
		return vendoredPath, nil
	}
	rel := filepath.FromSlash(strings.ReplaceAll(name, ".", "/")) + luaExt
	repoPath := filepath.Join(m.repoDir, rel)
	_, err := os.Stat(repoPath)
	if errors.Is(err, fs.ErrNotExist) {
		searched := make([]string, 0, len(m.srcDirs)+1)
		for _, srcDir := range m.srcDirs {
			searched = append(searched, filepath.Join(srcDir, rel))
		}
		searched = append(searched, repoPath)
		return "", fmt.Errorf("require %q: %w; searched %s", name, errNoModule, strings.Join(searched, ", "))
	}
	if err != nil {
		return "", err
	}
	return repoPath, nil
}
