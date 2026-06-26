package bootstrapper

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"platform.prodigy9.co/builder"
	"platform.prodigy9.co/project"
)

// FileAction distinguishes a fresh write from replacing an existing file, so
// the plan can warn the operator before an overwrite.
type FileAction int

const (
	FileWrite FileAction = iota
	FileOverwrite
)

func (a FileAction) String() string {
	if a == FileOverwrite {
		return "overwrite"
	}
	return "write"
}

// FileChange is one file the plan will materialise. Path is relative to the
// plan's Dir; Content is the exact bytes Apply writes.
type FileChange struct {
	Path    string
	Action  FileAction
	Content []byte
	Mode    fs.FileMode
}

// Plan is the result of the bootstrap analysis pass: every file to write and
// the disposition of every baseline var. Computing it is pure (reads only) so a
// caller can print and confirm it before Apply mutates the tree.
type Plan struct {
	Dir   string
	Files []FileChange
	Vars  []VarChange
}

// Analyze validates the target directory and computes the bootstrap plan
// without writing anything. defaultVars is the baseline's default [ops.vars];
// on a fresh repo they seed platform.toml, on a re-bootstrap they merge in
// (new keys appended, operator values preserved).
func Analyze(dir string, info *Info, defaultVars map[string]any) (*Plan, error) {
	dir, err := resolveWD(dir)
	if err != nil {
		return nil, err
	}
	if err := validateWD(dir); err != nil {
		return nil, err
	}

	projFile, vars, err := planProjectFile(dir, info, defaultVars)
	if err != nil {
		return nil, err
	}

	script, err := renderTemplate(platformTemplate, info)
	if err != nil {
		return nil, err
	}
	pipeline, err := renderTemplate(buildkitePipelineYamlTemplate, info)
	if err != nil {
		return nil, err
	}

	files := []FileChange{
		projFile,
		fileChange(dir, "platform", script, 0744),
		fileChange(dir, filepath.Join(".buildkite", "pipeline.yaml"), pipeline, 0644),
	}
	return &Plan{Dir: dir, Files: files, Vars: vars}, nil
}

// Apply writes the plan, creating parent directories as needed. Fresh writes
// always land; an existing file is overwritten only when replace is set,
// otherwise it is left untouched.
func (p *Plan) Apply(replace bool) error {
	for _, f := range p.Files {
		if f.Action == FileOverwrite && !replace {
			continue
		}
		dest := filepath.Join(p.Dir, f.Path)
		if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(dest, f.Content, f.Mode); err != nil {
			return err
		}
	}
	return nil
}

// Overwrites counts the existing files the plan would replace.
func (p *Plan) Overwrites() int {
	n := 0
	for _, f := range p.Files {
		if f.Action == FileOverwrite {
			n++
		}
	}
	return n
}

// Print renders the plan for operator review before applying.
func (p *Plan) Print(w io.Writer) {
	fmt.Fprintf(w, "bootstrap plan for %s:\n", p.Dir)
	for _, f := range p.Files {
		fmt.Fprintf(w, "  %-9s %s\n", f.Action, f.Path)
	}
	for _, v := range p.Vars {
		if v.Appended {
			fmt.Fprintf(w, "  append    [ops.vars] %s = %v\n", v.Key, v.Value)
		} else {
			fmt.Fprintf(w, "  keep      [ops.vars] %s (operator value)\n", v.Key)
		}
	}
}

// planProjectFile decides how platform.toml changes: a surgical [ops.vars]
// merge when it already exists (preserving operator edits), or a freshly
// generated file otherwise.
func planProjectFile(dir string, info *Info, defaultVars map[string]any) (FileChange, []VarChange, error) {
	path := filepath.Join(dir, "platform.toml")

	existing, err := os.ReadFile(path)
	if err == nil {
		merged, vars := mergeOpsVars(existing, defaultVars)
		return fileChange(dir, "platform.toml", merged, 0644), vars, nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return FileChange{}, nil, err
	}

	content, vars, err := generateProjectFile(dir, info, defaultVars)
	if err != nil {
		return FileChange{}, nil, err
	}
	return FileChange{Path: "platform.toml", Action: FileWrite, Content: content, Mode: 0644}, vars, nil
}

// generateProjectFile builds a fresh platform.toml from defaults, the operator
// info, discovered modules, and the seeded baseline vars.
func generateProjectFile(dir string, info *Info, defaultVars map[string]any) ([]byte, []VarChange, error) {
	proj := *project.ProjectDefaults
	proj.Modules = map[string]*project.Module{} // don't mutate the shared default map
	proj.Maintainer = fmt.Sprintf("%s <%s>", info.Maintainer, info.MaintainerEmail)
	proj.Repository = info.Repository
	proj.Ops.Vars = defaultVars

	mods, err := builder.Discover(dir)
	if err != nil && !errors.Is(err, builder.ErrNoBuilder) {
		return nil, nil, err
	}
	for name, bldr := range mods {
		mod := *project.ModuleDefaults
		mod.Builder = bldr.Name()

		switch bldr.Layout() {
		case builder.LayoutWorkspace:
			mod.WorkDir = "./" + name
		default: // LayoutBasic
			mod.WorkDir = "."
		}

		switch bldr.Class() {
		case builder.ClassNative: // run the compiled binary directly
			mod.CommandName = name
		default: // bytecode/interpreted need an interpreter or vm binary
			mod.CommandName = ""
		}

		proj.Modules[name] = &mod
	}

	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(&proj); err != nil {
		return nil, nil, err
	}
	return buf.Bytes(), classifyVars(defaultVars, nil), nil
}

// fileChange builds a FileChange, marking it an overwrite when the target
// already exists.
func fileChange(dir, rel string, content []byte, mode fs.FileMode) FileChange {
	action := FileWrite
	if _, err := os.Stat(filepath.Join(dir, rel)); err == nil {
		action = FileOverwrite
	}
	return FileChange{Path: rel, Action: action, Content: content, Mode: mode}
}
