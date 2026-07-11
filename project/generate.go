package project

import (
	"bytes"

	"github.com/BurntSushi/toml"
)

// GenerateInfo carries the operator inputs a fresh platform.toml needs: the
// maintainer line (already formatted "Name <email>") and the repository address.
type GenerateInfo struct {
	Maintainer string
	Repository string
}

// Generate builds a fresh platform.toml from the project defaults, the operator
// info, a framework's contributed module (keyed by name — nil for an
// unrecognized repo), and its default [ops.vars]. Returns the encoded bytes and
// the per-var disposition report (every default is appended on a fresh file). A
// non-empty strategy overrides the project default — a framework seeds it via
// its ScaffoldSpec ("latest" for infra, which cuts no versions and follows the
// moving tag).
func Generate(info GenerateInfo, name string, mod *Module, vars map[string]any, strategy string) ([]byte, []VarChange, error) {
	proj := *ProjectDefaults
	proj.Modules = map[string]*Module{} // don't mutate the shared default map
	proj.Maintainer = info.Maintainer
	proj.Repository = info.Repository
	proj.Ops.Vars = vars
	if strategy != "" {
		proj.Strategy = strategy
	}

	if mod != nil {
		proj.Modules[name] = mod
	}

	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(&proj); err != nil {
		return nil, nil, err
	}
	return buf.Bytes(), classifyVars(vars, nil), nil
}
