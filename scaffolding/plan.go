package scaffolding

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"platform.prodigy9.co/conf"
	"platform.prodigy9.co/framework"
	fwscaffold "platform.prodigy9.co/framework/scaffold"
	"platform.prodigy9.co/framework/skel"
)

var (
	daggerVersion   = framework.DaggerVersion
	platformVersion = framework.PlatformVersion
)

const (
	FileWrite FileAction = iota
	FileOverwrite
)

// FileAction distinguishes a fresh write from replacing an existing file, so
// the plan can warn the operator before an overwrite.
type FileAction int

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

// Plan is the result of the scaffold planning pass: every file to write and
// the disposition of every default var. Computing it is pure (reads only) so a
// caller can print and confirm it before Apply mutates the tree.
type Plan struct {
	Dir   string
	Files []FileChange
	Vars  []conf.VarChange
}

// Plan computes the scaffold plan without writing anything. The target carries the
// framework resolved by Discover, so repository changes between prompting and planning
// cannot silently select a different framework.
func (t *Target) Plan(info Info, inputs map[string]string) (*Plan, error) {
	spec := fwscaffold.Spec{}
	if t.framework != nil {
		env := fwscaffold.Env{
			Repository:      info.Repository,
			MaintainerEmail: info.MaintainerEmail,
			DaggerVersion:   daggerVersion(),
		}
		var err error
		spec, err = t.framework.Scaffold(context.Background(), t.dir, env, inputs)
		if err != nil {
			return nil, err
		}
	}

	projFile, vars, err := planProjectFile(t.dir, info, spec)
	if err != nil {
		return nil, err
	}

	launcher, err := resolveLauncher()
	if err != nil {
		return nil, err
	}

	files := []FileChange{
		projFile,
		fileChange(t.dir, launcher.Path, launcher.Content, 0744),
	}
	for _, file := range spec.Files {
		files = append(files, fileChange(t.dir, file.Path, file.Content, file.Mode))
	}
	return &Plan{Dir: t.dir, Files: files, Vars: vars}, nil
}

// resolveLauncher fills the launcher's version hole with the release this binary descends
// from. No derivable release is a hard error — a launcher pinned to nothing cannot run.
func resolveLauncher() (fwscaffold.File, error) {
	version := platformVersion()
	if version == "" {
		return fwscaffold.File{}, errors.New("init: no platform release version is derivable from this binary's build info")
	}

	resolved, err := fwscaffold.Resolve(
		[]fwscaffold.File{{Path: "platform.tmpl", Content: skel.Launcher, Mode: 0744}},
		fwscaffold.Data{"PlatformVersion": version})
	if err != nil {
		return fwscaffold.File{}, err
	}
	return resolved[0], nil
}

// planProjectFile decides how platform.toml changes: a surgical [vars]
// merge when it already exists (preserving operator edits), or a freshly
// generated file otherwise, seeded with the framework's strategy value.
func planProjectFile(dir string, info Info, spec fwscaffold.Spec) (FileChange, []conf.VarChange, error) {
	path := filepath.Join(dir, "platform.toml")

	existing, err := os.ReadFile(path)
	if err == nil {
		merged, vars := conf.MergeVars(existing, spec.Vars)
		return fileChange(dir, "platform.toml", merged, 0644), vars, nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return FileChange{}, nil, err
	}

	content, vars, err := conf.Generate(conf.GenerateInfo{
		Maintainer: fmt.Sprintf("%s <%s>", info.Maintainer, info.MaintainerEmail),
		Repository: info.Repository,
		Strategy:   spec.Strategy,
	}, filepath.Base(dir), spec.Module, spec.Vars)
	if err != nil {
		return FileChange{}, nil, err
	}
	return FileChange{Path: "platform.toml", Action: FileWrite, Content: content, Mode: 0644}, vars, nil
}

// Apply writes fresh files and returns the changes it applied, skipping overwrites.
func (p *Plan) Apply() ([]FileChange, error) {
	freshFiles := make([]FileChange, 0, len(p.Files))
	for _, file := range p.Files {
		if file.Action == FileWrite {
			freshFiles = append(freshFiles, file)
		}
	}

	if err := p.write(freshFiles); err != nil {
		return nil, err
	}
	return freshFiles, nil
}

// ForceApply writes every planned file, replacing existing files, and returns the changes.
func (p *Plan) ForceApply() ([]FileChange, error) {
	if err := p.write(p.Files); err != nil {
		return nil, err
	}
	return p.Files, nil
}

func (p *Plan) write(files []FileChange) error {
	for _, f := range files {
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
	fmt.Fprintf(w, "scaffold plan for %s:\n", p.Dir)
	for _, f := range p.Files {
		fmt.Fprintf(w, "  %-9s %s\n", f.Action, f.Path)
	}
	for _, v := range p.Vars {
		if v.Appended {
			fmt.Fprintf(w, "  append    [vars] %s = %v\n", v.Key, v.Value)
		} else {
			fmt.Fprintf(w, "  keep      [vars] %s (operator value)\n", v.Key)
		}
	}
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
