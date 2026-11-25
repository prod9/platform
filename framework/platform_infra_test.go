package framework

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	r "github.com/stretchr/testify/require"
	"platform.prodigy9.co/framework/scaffold"
)

func writeTemplate(t *testing.T, wd, name, content string) {
	t.Helper()
	path := filepath.Join(wd, "vendor", "infra-lua", "templates", name)
	r.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	r.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

// docs/spec/scaffolding.md, "The Infra framework scaffolds from vendored infra-lua":
// defaults.lua goes to the repo root, every other .lua template to apps/, byte for byte;
// non-Lua files are skipped; the declared vars are the seeded [vars].
func TestPlatformInfraScaffoldCopiesTemplates(t *testing.T) {
	wd := filepath.Join(t.TempDir(), "test-infra")
	certManager := "local vars = platform.vars {CERT_MANAGER_VERSION = \"v1.20.2\"}\nreturn {}\n"
	writeTemplate(t, wd, "cert-manager.lua", certManager)
	writeTemplate(t, wd, "flux-sync.lua", "local vars = platform.vars {FLUX_HOSTNAME = \"flux.example.com\", CERT_MANAGER_VERSION = \"v1.20.2\"}\nreturn {}\n")
	writeTemplate(t, wd, "defaults.lua", "return {}\n")
	writeTemplate(t, wd, "README.md", "# templates\n")

	spec, err := PlatformInfra{}.Scaffold(context.Background(), wd, scaffold.Env{}, nil)
	r.NoError(t, err)

	paths := map[string]string{}
	for _, file := range spec.Files {
		paths[file.Path] = string(file.Content)
	}
	r.Equal(t, map[string]string{
		"apps/cert-manager.lua": certManager,
		"apps/flux-sync.lua":    "local vars = platform.vars {FLUX_HOSTNAME = \"flux.example.com\", CERT_MANAGER_VERSION = \"v1.20.2\"}\nreturn {}\n",
		"defaults.lua":          "return {}\n",
	}, paths)
	r.Equal(t, map[string]any{
		"CERT_MANAGER_VERSION": "v1.20.2",
		"FLUX_HOSTNAME":        "flux.example.com",
	}, spec.Vars)
	r.Equal(t, "rolling", spec.Strategy)
}

// Two templates that declare one name with different defaults fail init.
func TestPlatformInfraScaffoldRejectsConflictingDefaults(t *testing.T) {
	wd := filepath.Join(t.TempDir(), "test-infra")
	writeTemplate(t, wd, "a.lua", "platform.vars {FLUX_VERSION = \"v2.8.8\"}\n")
	writeTemplate(t, wd, "b.lua", "platform.vars {FLUX_VERSION = \"v2.9.0\"}\n")

	_, err := PlatformInfra{}.Scaffold(context.Background(), wd, scaffold.Env{}, nil)
	r.ErrorContains(t, err, "FLUX_VERSION")
}
