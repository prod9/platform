// Package gitops renders infra CUE modules to Kubernetes manifests and
// publishes them as OCI artifacts for pull-based GitOps delivery.
package gitops

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// DefaultRegistry maps the infra-defs module prefix to its OCI host for
// `cue export` dependency resolution. An ambient CUE_REGISTRY wins if set.
const DefaultRegistry = "prodigy9.co=ghcr.io/prod9"

// Render runs `cue export` over the infra CUE module at dir, injecting image
// into its `@tag(image)`, and returns the module's `objects` list as a
// multi-document YAML stream — one Kubernetes object per document.
func Render(dir, image string) (string, error) {
	sequence, err := exportObjects(dir, image)
	if err != nil {
		return "", err
	}

	return splitDocuments(sequence)
}

// exportObjects shells out to `cue export ... --out yaml`, which emits the
// objects list as a single YAML sequence with faithful scalar types.
func exportObjects(dir, image string) ([]byte, error) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}

	// cue rejects an absolute path as a package arg, so run inside dir and
	// export the current directory.
	args := []string{"export", ".", "-e", "objects", "--out", "yaml"}
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

// splitDocuments turns cue's YAML sequence of objects into a multi-document
// stream. It walks the parsed nodes rather than splitting text, so nested
// lists and scalar fidelity (ints stay ints) survive the round-trip.
func splitDocuments(sequence []byte) (string, error) {
	var root yaml.Node
	if err := yaml.Unmarshal(sequence, &root); err != nil {
		return "", err
	}
	if len(root.Content) == 0 {
		return "", nil
	}

	items := root.Content[0].Content
	docs := make([]string, 0, len(items))
	for _, item := range items {
		out, err := yaml.Marshal(item)
		if err != nil {
			return "", err
		}
		docs = append(docs, string(out))
	}

	return strings.Join(docs, "---\n"), nil
}
