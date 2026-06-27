package project

import (
	"strings"
	"testing"
	"time"

	"github.com/BurntSushi/toml"
	r "github.com/stretchr/testify/require"
	"platform.prodigy9.co/internal/timeouts"
	"platform.prodigy9.co/ops"
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

func TestProject_opsTarget(t *testing.T) {
	// Convention: the config-artifact target is inferred from the repository,
	// same rule as ImageName, with a default moving tag of "latest".
	proj := testProject(1)
	proj.inferValues()
	r.Equal(t, "ghcr.io/prod9/platform", proj.Ops.Image)
	r.Equal(t, "latest", proj.Ops.Tag)

	ref, err := proj.Ops.Ref("")
	r.NoError(t, err)
	r.Equal(t, "ghcr.io/prod9/platform:latest", ref)

	ref, err = proj.Ops.Ref("staging")
	r.NoError(t, err)
	r.Equal(t, "ghcr.io/prod9/platform:staging", ref)

	// Explicit [ops] config wins over the convention.
	proj = testProject(1)
	proj.Ops = ops.Ops{Image: "ghcr.io/prod9/infra-stage9", Tag: "prod"}
	proj.inferValues()
	r.Equal(t, "ghcr.io/prod9/infra-stage9", proj.Ops.Image)
	r.Equal(t, "prod", proj.Ops.Tag)

	// No repository and no [ops] image → no inferable target.
	proj = &Project{Modules: map[string]*Module{}}
	proj.inferValues()
	_, err = proj.Ops.Ref("")
	r.ErrorIs(t, err, ops.ErrNoOpsImage)
}

func TestProject_opsVars(t *testing.T) {
	// [ops.vars] is the verbatim DSL \(var) table — a pure passthrough map.
	// Bools and numbers are strings (TOML has no untyped scalar the DSL wants),
	// and the processor stores them as-is, no per-software fields.
	const config = `
repository = "github.com/prod9/platform"

[ops.vars]
cert_manager_version = "v1.16.0"
nginx_experimental   = "true"
`
	proj := &Project{}
	_, err := toml.Decode(config, proj)
	r.NoError(t, err)
	r.Equal(t, "v1.16.0", proj.Ops.Vars["cert_manager_version"])
	r.Equal(t, "true", proj.Ops.Vars["nginx_experimental"])

	// No [ops.vars] → nil map, not an empty allocation: true passthrough.
	proj = &Project{}
	_, err = toml.Decode(`repository = "github.com/prod9/platform"`, proj)
	r.NoError(t, err)
	r.Nil(t, proj.Ops.Vars)
}

func testProject(modCount int) *Project {
	proj := &Project{
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
