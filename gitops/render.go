// Package gitops renders an infra CUE module's apps to Kubernetes manifests and
// publishes them as OCI artifacts for pull-based GitOps delivery.
package gitops

import (
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"strings"

	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/build"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/cue/load"
	cueyaml "cuelang.org/go/encoding/yaml"
	"cuelang.org/go/mod/modconfig"
	"gopkg.in/yaml.v3"
	"platform.prodigy9.co/dsl"
	"platform.prodigy9.co/project"
)

// DefaultRegistry maps the infra-defs module prefix to its OCI host for
// `cue export` dependency resolution. An ambient CUE_REGISTRY wins if set.
const DefaultRegistry = "prodigy9.co=ghcr.io/prod9"

const (
	// appsPackage is the one source directory holding both formats: the `apps` CUE
	// package (each top-level field one component, a filename->docs map) and the
	// co-located `.platform` directives. Render routes by extension.
	appsPackage = "apps"

	platformExt = ".platform"
)

// RenderOptions carries the render context: the [ops.vars] table feeding both routes —
// CUE `@tag(name)` holes and directive `\(var)` interpolation — and an optional Fetch
// override for `download` (nil uses a plain HTTP GET; tests inject fixtures).
type RenderOptions struct {
	Vars  map[string]any
	Fetch func(url string) ([]byte, error)
}

// Render walks srcDir's apps/ and fuses both routes into one tree, by extension: the CUE
// package (`.cue` → file-map `cue export`) and the co-located directives (`.platform` →
// dsl.Apply). Each route is skipped when it contributes nothing. Both write
// <component>/<filename> entries.
func Render(srcDir string, opts RenderOptions) (Tree, error) {
	vars := project.NormalizeVars(opts.Vars)

	tree, err := renderCue(srcDir, vars)
	if err != nil {
		return nil, err
	}

	rendered, err := renderDirectives(srcDir, vars, opts.Fetch)
	if err != nil {
		return nil, err
	}

	maps.Copy(tree, rendered)
	return tree, nil
}

// renderCue exports the CUE apps package under srcDir/apps into a file-map tree. Each app
// field becomes a component directory; each filename key under it becomes a named file
// holding one document (a map value) or a multi-doc stream (a list value). Skipped when the
// dir has no `.cue` files (directives-only apps/, or no apps/ at all).
func renderCue(srcDir string, vars map[string]any) (Tree, error) {
	cue, err := filesWithExt(filepath.Join(srcDir, appsPackage), ".cue")
	if err != nil {
		return nil, err
	}
	if len(cue) == 0 {
		return Tree{}, nil
	}

	exported, err := exportCue(srcDir, vars)
	if err != nil {
		return nil, err
	}
	return buildTree(exported)
}

// exportCue evaluates the apps CUE package via the linked CUE engine and encodes it to
// YAML — the app->files->docs structure with faithful scalar types, same shape the old
// `cue export --out yaml` produced. The normalized [ops.vars] feed the apps' `@tag(name)`
// holes as load tags — the committed-config source for every CUE tag.
func exportCue(srcDir string, vars map[string]any) ([]byte, error) {
	dir, err := filepath.Abs(filepath.Join(srcDir, appsPackage))
	if err != nil {
		return nil, err
	}

	registry, err := modconfig.NewRegistry(&modconfig.Config{CUERegistry: cueRegistry()})
	if err != nil {
		return nil, err
	}

	cfg := &load.Config{Dir: dir, Registry: registry}

	// First pass, no tags: discover which `@tag` holes the apps actually declare. [ops.vars]
	// feeds both render routes, so it carries vars meant only for `.platform` directives; those
	// have no CUE `@tag`, and CUE rejects an injected tag that nothing declares ("no tag for X").
	// Inject only the declared subset.
	probe := load.Instances([]string{"."}, cfg)
	if len(probe) == 0 {
		return nil, fmt.Errorf("render: no CUE instance under %s", dir)
	}
	if err := probe[0].Err; err != nil {
		return nil, err
	}
	declared := declaredTags(probe[0])

	if tags := varsToTags(vars, declared); len(tags) > 0 {
		cfg.Tags = tags
	}

	insts := load.Instances([]string{"."}, cfg)
	if len(insts) == 0 {
		return nil, fmt.Errorf("render: no CUE instance under %s", dir)
	}
	if err := insts[0].Err; err != nil {
		return nil, err
	}

	value := cuecontext.New().BuildInstance(insts[0])
	if err := value.Err(); err != nil {
		return nil, err
	}
	return cueyaml.Encode(value)
}

// varsToTags renders the normalized [ops.vars] table into cue load tags ("name=value") — the
// committed-config source for every `@tag(name)` in the apps CUE. Only vars a `@tag` actually
// declares are injected (declared); the rest are directive-only and CUE would reject them.
// Values stringify verbatim; the consuming field's `@tag(name,type=...)` annotation drives any
// non-string coercion.
func varsToTags(vars map[string]any, declared map[string]bool) []string {
	tags := make([]string, 0, len(vars))
	for name, val := range vars {
		if declared[name] {
			tags = append(tags, fmt.Sprintf("%s=%v", name, val))
		}
	}
	return tags
}

// declaredTags walks the loaded apps' syntax for `@tag(name)` attributes and returns the set of
// declared tag names — the holes the CUE package is willing to receive from [ops.vars].
func declaredTags(inst *build.Instance) map[string]bool {
	declared := map[string]bool{}
	for _, f := range inst.Files {
		ast.Walk(f, func(n ast.Node) bool {
			if a, ok := n.(*ast.Attribute); ok {
				if name, ok := tagAttrName(a); ok {
					declared[name] = true
				}
			}
			return true
		}, nil)
	}
	return declared
}

// tagAttrName extracts the tag name from an `@tag(name,opts...)` attribute, false for any other.
func tagAttrName(a *ast.Attribute) (string, bool) {
	key, body := a.Split()
	if key != "tag" {
		return "", false
	}
	name, _, _ := strings.Cut(body, ",")
	return name, name != ""
}

// renderDirectives runs every `.platform` directive co-located with the CUE apps (selection
// happened at install time, so whatever is present applies). Each runs with dsl.Apply into a
// per-component output directory; results collect into a tree keyed by <component>/<emitted-file>.
// No `.platform` files renders nothing.
func renderDirectives(srcDir string, vars map[string]any, fetch func(string) ([]byte, error)) (Tree, error) {
	dir := filepath.Join(srcDir, appsPackage)
	names, err := filesWithExt(dir, platformExt)
	if err != nil || len(names) == 0 {
		return Tree{}, err
	}

	out, err := os.MkdirTemp("", "platform-render-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(out)

	for _, name := range names {
		if err := applyDirective(dir, out, name, vars, fetch); err != nil {
			return nil, fmt.Errorf("render: %s: %w", name, err)
		}
	}
	return readTree(out)
}

// applyDirective runs one directive file into outRoot/<outputName>.
func applyDirective(srcDir, outRoot, name string, vars map[string]any, fetch func(string) ([]byte, error)) error {
	directives, err := os.ReadFile(filepath.Join(srcDir, name))
	if err != nil {
		return err
	}

	outDir := filepath.Join(outRoot, outputName(name))
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}

	_, err = dsl.Apply(string(directives), dsl.Options{Vars: vars, OutDir: outDir, Fetch: fetch})
	return err
}

// outputName is the directory a directive's emitted manifests render under: its filename
// without the .platform extension (so they land in k8s/<outputName>/).
func outputName(file string) string {
	return strings.TrimSuffix(file, platformExt)
}

// filesWithExt lists filenames with ext directly under dir. An absent directory yields
// no files rather than an error.
func filesWithExt(dir, ext string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ext) {
			names = append(names, e.Name())
		}
	}
	return names, nil
}

// readTree reads every file under root into a tree keyed by its slash-separated
// path relative to root (i.e. <component>/<filename>).
func readTree(root string) (Tree, error) {
	tree := Tree{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		tree[filepath.ToSlash(rel)] = content
		return nil
	})
	return tree, err
}

// cueRegistry is the module registry spec for resolving prodigy9.co/defs: the ambient
// CUE_REGISTRY if set, else the project default.
func cueRegistry() string {
	if r := os.Getenv("CUE_REGISTRY"); r != "" {
		return r
	}
	return DefaultRegistry
}

// buildTree walks cue's exported app->files mapping into a Tree, keying each
// file by <app>/<filename>. It walks parsed nodes rather than re-marshalling
// through interface{}, so scalar fidelity (ints stay ints) survives.
func buildTree(exported []byte) (Tree, error) {
	var root yaml.Node
	if err := yaml.Unmarshal(exported, &root); err != nil {
		return nil, err
	}
	if len(root.Content) == 0 {
		return Tree{}, nil
	}

	apps := root.Content[0]
	if apps.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("render: apps export is not a mapping (got %v)", apps.Kind)
	}

	tree := Tree{}
	for i := 0; i+1 < len(apps.Content); i += 2 {
		app := apps.Content[i].Value
		files := apps.Content[i+1]
		if files.Kind != yaml.MappingNode {
			return nil, fmt.Errorf("render: app %q is not a filename map", app)
		}

		for j := 0; j+1 < len(files.Content); j += 2 {
			name := files.Content[j].Value
			content, err := encodeFile(files.Content[j+1])
			if err != nil {
				return nil, fmt.Errorf("render: %s/%s: %w", app, name, err)
			}
			tree[app+"/"+name] = content
		}
	}
	return tree, nil
}

// encodeFile renders one filename value: a sequence node becomes a multi-doc
// YAML stream (one doc per element), any other node becomes a single document.
func encodeFile(node *yaml.Node) ([]byte, error) {
	if node.Kind != yaml.SequenceNode {
		return yaml.Marshal(node)
	}

	docs := make([]string, 0, len(node.Content))
	for _, item := range node.Content {
		out, err := yaml.Marshal(item)
		if err != nil {
			return nil, err
		}
		docs = append(docs, string(out))
	}
	return []byte(strings.Join(docs, "---\n")), nil
}
