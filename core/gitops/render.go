// Package gitops renders an infra CUE module's apps to Kubernetes manifests and
// publishes them as OCI artifacts for pull-based GitOps delivery.
package gitops

import (
	"fmt"
	"io/fs"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
	"platform.prodigy9.co/core/baseline"
	"platform.prodigy9.co/core/dsl"
)

// DefaultRegistry maps the infra-defs module prefix to its OCI host for
// `cue export` dependency resolution. An ambient CUE_REGISTRY wins if set.
const DefaultRegistry = "prodigy9.co=ghcr.io/prod9"

const (
	// appsPackage holds the `apps` CUE package: each top-level field is one app
	// (component), valued by a filename->docs map.
	appsPackage = "apps"

	// baselinePackage holds the cluster-baseline `.platform` directive files,
	// gated and assembled by core/baseline.
	baselinePackage = "baseline"

	platformExt = ".platform"
)

// RenderOptions carries the render context: the image tag injected into the apps'
// `@tag(image)`, the [ops.vars] table gating the baseline and feeding directive
// `\(var)` interpolation, and an optional Fetch override for `download` (nil uses
// a plain HTTP GET; tests inject fixtures).
type RenderOptions struct {
	Image string
	Vars  map[string]any
	Fetch func(url string) ([]byte, error)
}

// Render walks srcDir and fuses both render routes into one tree: the `apps` CUE
// package (`.cue` → file-map `cue export`) and the `baseline` directive set
// (`.platform` → baseline.Select → dsl.Apply). Either route is skipped when its
// package directory is absent. Both write <component>/<filename> entries.
func Render(srcDir string, opts RenderOptions) (Tree, error) {
	tree, err := renderApps(srcDir, opts.Image)
	if err != nil {
		return nil, err
	}

	rendered, err := renderBaseline(srcDir, opts.Vars, opts.Fetch)
	if err != nil {
		return nil, err
	}

	maps.Copy(tree, rendered)
	return tree, nil
}

// renderApps exports the apps package under srcDir into a file-map tree. Each app
// field becomes a component directory; each filename key under it becomes a named
// file holding one document (a map value) or a multi-doc stream (a list value).
func renderApps(srcDir, image string) (Tree, error) {
	if !isDir(filepath.Join(srcDir, appsPackage)) {
		return Tree{}, nil
	}

	exported, err := exportApps(srcDir, image)
	if err != nil {
		return nil, err
	}
	return buildTree(exported)
}

// exportApps shells out to `cue export ... --out yaml` over the apps package,
// emitting the app->files->docs structure with faithful scalar types.
func exportApps(srcDir, image string) ([]byte, error) {
	dir, err := filepath.Abs(filepath.Join(srcDir, appsPackage))
	if err != nil {
		return nil, err
	}

	// cue rejects an absolute path as a package arg, so run inside the package
	// directory and export the current directory.
	args := []string{"export", ".", "--out", "yaml"}
	if image != "" {
		args = append(args, "--inject", "image="+image)
	}

	cmd := exec.Command("cue", args...)
	cmd.Dir = dir
	cmd.Env = registryEnv()
	cmd.Stderr = os.Stderr
	return cmd.Output()
}

// renderBaseline assembles the baseline directive set under srcDir/baseline:
// baseline.Select gates the files by vars, then each is run with dsl.Apply into a
// per-component output directory. Results are collected into a tree keyed by
// <component>/<emitted-file>. An absent baseline package renders nothing.
func renderBaseline(srcDir string, vars map[string]any, fetch func(string) ([]byte, error)) (Tree, error) {
	dir := filepath.Join(srcDir, baselinePackage)
	names, err := platformFiles(dir)
	if err != nil || len(names) == 0 {
		return Tree{}, err
	}

	selected, err := baseline.Select(names, vars)
	if err != nil {
		return nil, err
	}

	out, err := os.MkdirTemp("", "platform-render-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(out)

	for _, name := range selected {
		if err := applyDirective(dir, out, name, vars, fetch); err != nil {
			return nil, fmt.Errorf("render: %s: %w", name, err)
		}
	}
	return readTree(out)
}

// applyDirective runs one directive file into outRoot/<component>, where
// component is the file's stem before any @variant / +flag marker.
func applyDirective(srcDir, outRoot, name string, vars map[string]any, fetch func(string) ([]byte, error)) error {
	directives, err := os.ReadFile(filepath.Join(srcDir, name))
	if err != nil {
		return err
	}

	outDir := filepath.Join(outRoot, baseline.Component(name))
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}

	_, err = dsl.Apply(string(directives), dsl.Options{Vars: vars, OutDir: outDir, Fetch: fetch})
	return err
}

// platformFiles lists the .platform directive filenames directly under dir. An
// absent directory yields no files rather than an error.
func platformFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), platformExt) {
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

// isDir reports whether path exists and is a directory.
func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func registryEnv() []string {
	env := os.Environ()
	if os.Getenv("CUE_REGISTRY") == "" {
		env = append(env, "CUE_REGISTRY="+DefaultRegistry)
	}
	return env
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
