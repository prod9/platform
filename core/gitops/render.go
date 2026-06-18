// Package gitops renders an infra CUE module's apps to Kubernetes manifests and
// publishes them as OCI artifacts for pull-based GitOps delivery.
package gitops

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// DefaultRegistry maps the infra-defs module prefix to its OCI host for
// `cue export` dependency resolution. An ambient CUE_REGISTRY wins if set.
const DefaultRegistry = "prodigy9.co=ghcr.io/prod9"

// appsPackage is the conventional subdirectory holding the `apps` CUE package:
// each top-level field is one app (component), valued by a filename->docs map.
const appsPackage = "apps"

// Render exports the apps package under srcDir, injecting image into its
// `@tag(image)`, and returns the rendered file-map tree. Each app field becomes
// a component directory; each filename key under it becomes a named file holding
// one document (a map value) or a multi-doc stream (a list value).
func Render(srcDir, image string) (Tree, error) {
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
