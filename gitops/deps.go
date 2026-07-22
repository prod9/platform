package gitops

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"cuelang.org/go/mod/modconfig"
	"cuelang.org/go/mod/modfile"
	"platform.prodigy9.co/cuemod"
)

// FetchDeps downloads every CUE module the repo depends on, transitively, into the local
// module cache. Render resolves the same modules on its own, so this is never required for
// correctness — it exists so the download is a step of its own: it is the one part of a
// render that touches the network and can take real time, and a stage nobody can see is a
// stage nobody can diagnose. Running it twice is free; the second pass is cache hits.
func FetchDeps(ctx context.Context, srcDir string) error {
	registry, err := newRegistry()
	if err != nil {
		return err
	}
	return fetchDeps(ctx, srcDir, registry)
}

func fetchDeps(ctx context.Context, srcDir string, registry modconfig.Registry) error {
	body, err := os.ReadFile(filepath.Join(srcDir, cuemod.ModuleFile))
	if err != nil {
		return fmt.Errorf("fetch deps: read cue.mod: %w", err)
	}

	file, err := modfile.Parse(body, cuemod.ModuleFile)
	if err != nil {
		return fmt.Errorf("fetch deps: parse cue.mod: %w", err)
	}

	// Breadth-first over the requirement graph: a module's own requirements are only
	// readable once it is fetched, so discovery and download are the same walk.
	seen := map[string]bool{}
	queue := file.DepVersions()
	for len(queue) > 0 {
		mv := queue[0]
		queue = queue[1:]

		if seen[mv.String()] {
			continue
		}
		seen[mv.String()] = true

		if _, err := registry.Fetch(ctx, mv); err != nil {
			return fmt.Errorf("fetch deps: %s: %w", mv, err)
		}

		reqs, err := registry.Requirements(ctx, mv)
		if err != nil {
			return fmt.Errorf("fetch deps: %s requirements: %w", mv, err)
		}
		queue = append(queue, reqs...)
	}
	return nil
}

// newRegistry builds the module registry both the dependency fetch and the CUE export
// resolve through, so the two always agree on where modules come from.
func newRegistry() (modconfig.Registry, error) {
	return modconfig.NewRegistry(&modconfig.Config{CUERegistry: cueRegistry()})
}
