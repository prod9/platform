package conf

import (
	"strings"
	"testing"
	"time"

	"github.com/BurntSushi/toml"
	r "github.com/stretchr/testify/require"
	"platform.prodigy9.co/internal/timeouts"
)

var ModNames []string = strings.Split("alpha,beta,gamma,delta,epsilon", ",")

func TestProject_inferValues(t *testing.T) {
	proj := testProject(1)
	proj.inferValues()
	r.Equal(t, "ghcr.io/prod9/platform", proj.Modules[ModNames[0]].ImageName)

	proj = testProject(2)
	proj.inferValues()
	r.Equal(t, "ghcr.io/prod9/platform/"+ModNames[0], proj.Modules[ModNames[0]].ImageName)
	r.Equal(t, "ghcr.io/prod9/platform/"+ModNames[1], proj.Modules[ModNames[1]].ImageName)
}

// TestRepositoryURL pins the derivation every consumer of a link goes through. The stored
// form is scheme-less, so pasting it into a URL is the bug this function exists to stop;
// a platform.toml predating that rule still carries the scheme, and must not double it.
func TestRepositoryURL(t *testing.T) {
	r.Equal(t, "https://github.com/prod9/platform", RepositoryURL("github.com/prod9/platform"))
	r.Equal(t, "https://github.com/prod9/platform", RepositoryURL("https://github.com/prod9/platform"))
	r.Equal(t, "", RepositoryURL(""))
}

func TestProject_vars(t *testing.T) {
	// [vars] is the verbatim DSL \(var) table — a pure passthrough map.
	// Bools and numbers are strings (TOML has no untyped scalar the DSL wants),
	// and the processor stores them as-is, no per-software fields.
	const config = `
repository = "github.com/prod9/platform"

[vars]
cert_manager_version = "v1.16.0"
nginx_experimental   = "true"
`
	proj := &Model{}
	_, err := toml.Decode(config, proj)
	r.NoError(t, err)
	r.Equal(t, "v1.16.0", proj.Vars["cert_manager_version"])
	r.Equal(t, "true", proj.Vars["nginx_experimental"])

	// No [vars] → nil map, not an empty allocation: true passthrough.
	proj = &Model{}
	_, err = toml.Decode(`repository = "github.com/prod9/platform"`, proj)
	r.NoError(t, err)
	r.Nil(t, proj.Vars)
}

// TestParse pins the byte-level entry the server's manifest read uses: full defaults
// and inference, no filesystem paths and no env overrides — a server env var must
// never leak into a manifest parsed on a repo's behalf.
func TestParse(t *testing.T) {
	proj, err := Parse([]byte(`
repository = "github.com/prod9/app"

[modules.web]
framework = "pnpm/basic"
`))
	r.NoError(t, err)
	r.Equal(t, "github.com/prod9/app", proj.Repository)
	r.Equal(t, "pnpm/basic", proj.Modules["web"].Framework)
	r.Equal(t, ".", proj.Modules["web"].WorkDir)
	r.Equal(t, "ghcr.io/prod9/app", proj.Modules["web"].ImageName)
	r.Equal(t, timeouts.From(1*time.Minute), proj.Modules["web"].Timeout)

	_, err = Parse([]byte(`modules = `))
	r.Error(t, err)
}

func testProject(modCount int) *Model {
	proj := &Model{
		Repository: "github.com/prod9/platform",
		Modules:    map[string]*Module{},
	}

	for i := range modCount {
		name := ModNames[i]
		proj.Modules[name] = &Module{
			WorkDir: "./" + name,
			Timeout: timeouts.Timeout(5 * time.Minute),
			Builder: "go/basic",
		}
	}
	return proj
}

func TestModule_frameworkKeyAlias(t *testing.T) {
	// The [modules] key is `framework`; the legacy `builder` key survives as a
	// deprecated read-alias so existing platform.tomls keep working.
	decode := func(body string) *Model {
		proj := &Model{}
		_, err := toml.Decode(body, proj)
		r.NoError(t, err)
		proj.assignDefaults()
		return proj
	}

	proj := decode("[modules.app]\nframework = \"go/basic\"\n")
	r.Equal(t, "go/basic", proj.Modules["app"].Framework)

	proj = decode("[modules.app]\nbuilder = \"go/basic\"\n")
	r.Equal(t, "go/basic", proj.Modules["app"].Framework, "legacy builder key must alias")

	// When both appear the canonical key wins.
	proj = decode("[modules.app]\nframework = \"go/basic\"\nbuilder = \"pnpm/basic\"\n")
	r.Equal(t, "go/basic", proj.Modules["app"].Framework)

	// Normalization clears the alias so a re-encode emits only `framework`.
	var buf strings.Builder
	r.NoError(t, toml.NewEncoder(&buf).Encode(proj))
	r.Contains(t, buf.String(), "framework = ")
	r.NotContains(t, buf.String(), "builder = ")
}
