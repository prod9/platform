package project

import (
	"bytes"

	"github.com/BurntSushi/toml"
)

// GenerateInfo carries the operator inputs a fresh platform.toml needs: the
// maintainer line (already formatted "Name <email>"), the repository address, and
// the framework-seeded Strategy and ImportPrefix. Strategy and ImportPrefix are
// empty for app frameworks; the Infra framework seeds both via its ScaffoldSpec.
type GenerateInfo struct {
	Maintainer   string
	Repository   string
	Strategy     string
	ImportPrefix string
}

// Generate builds a fresh platform.toml from the project defaults, the operator
// info, a framework's contributed module (keyed by name — nil for an
// unrecognized repo), and its default [vars]. Returns the encoded bytes and
// the per-var disposition report (every default is appended on a fresh file). A
// non-empty Strategy overrides the project default (the Infra framework seeds
// "rolling", which cuts no versions and follows the moving tag); a non-empty
// ImportPrefix seeds the CUE module namespace (Infra only — apps omit it).
func Generate(info GenerateInfo, name string, mod *Module, vars map[string]any) ([]byte, []VarChange, error) {
	proj := *ProjectDefaults
	proj.Modules = map[string]*Module{} // don't mutate the shared default map
	proj.Maintainer = info.Maintainer
	proj.Repository = info.Repository
	proj.Vars = vars
	if info.Strategy != "" {
		proj.Strategy = info.Strategy
	}
	if info.ImportPrefix != "" {
		proj.ImportPrefix = info.ImportPrefix
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
