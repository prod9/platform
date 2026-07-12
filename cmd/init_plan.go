package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"platform.prodigy9.co/framework"
	fwscaffold "platform.prodigy9.co/framework/scaffold"
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

// Plan is the result of the scaffold analysis pass: every file to write and
// the disposition of every default var. Computing it is pure (reads only) so a
// caller can print and confirm it before Apply mutates the tree. NeedsGitRepo
// carries the framework's git need up to the driver, which creates the repo
// before writing.
type Plan struct {
	Dir          string
	Files        []FileChange
	Vars         []project.VarChange
	NeedsGitRepo bool
}

// Analyze computes the scaffold plan for a repo without writing anything — one uniform
// path for every framework. It discovers the framework, folds in its scaffold
// contribution (platform.toml module + default [vars] + files, resolved), and writes
// the version-pinned launcher. What a repo gets is entirely the framework's Scaffold
// output — there is no app-vs-infra branch.
func Analyze(dir string, info *Info) (*Plan, error) {
	dir, err := resolveWD(dir)
	if err != nil {
		return nil, err
	}

	if err := validateDir(dir); err != nil {
		return nil, err
	}
	spec, err := discoverSpec(dir)
	if err != nil {
		return nil, err
	}
	// The git gate is framework-set: a scaffold that creates its own repo needs none;
	// everything else hard-gates on an existing repo (the appliance baseline is delivered
	// through GitOps, so a non-repo target is virtually always a mistake).
	if !spec.NeedsGitRepo && !IsGitRepo(dir) {
		return nil, ErrWDNotGit
	}

	projFile, vars, err := planProjectFile(dir, info, spec)
	if err != nil {
		return nil, err
	}
	specFiles, err := planSpecFiles(dir, info, spec)
	if err != nil {
		return nil, err
	}

	files := []FileChange{
		projFile,
		fileChange(dir, "platform", []byte(platformTemplate), 0744),
	}
	files = append(files, specFiles...)
	return &Plan{Dir: dir, Files: files, Vars: vars, NeedsGitRepo: spec.NeedsGitRepo}, nil
}

// discoverSpec finds the framework rooting dir and returns its scaffold contribution. A
// missing framework is not an error (an unrecognised repo still gets platform.toml +
// launcher); the zero spec carries no module, vars, or files.
func discoverSpec(dir string) (fwscaffold.Spec, error) {
	fw, err := framework.Discover(dir)
	if err != nil && !errors.Is(err, framework.ErrNoFramework) {
		return fwscaffold.Spec{}, err
	}
	if fw == nil {
		return fwscaffold.Spec{}, nil
	}
	return fw.Scaffold(context.Background(), dir)
}

// planProjectFile decides how platform.toml changes: a surgical [vars]
// merge when it already exists (preserving operator edits), or a freshly
// generated file otherwise, seeded with the framework's strategy value.
func planProjectFile(dir string, info *Info, spec fwscaffold.Spec) (FileChange, []project.VarChange, error) {
	path := filepath.Join(dir, "platform.toml")

	existing, err := os.ReadFile(path)
	if err == nil {
		merged, vars := project.MergeVars(existing, spec.Vars)
		return fileChange(dir, "platform.toml", merged, 0644), vars, nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return FileChange{}, nil, err
	}

	content, vars, err := project.Generate(project.GenerateInfo{
		Maintainer:   fmt.Sprintf("%s <%s>", info.Maintainer, info.MaintainerEmail),
		Repository:   info.Repository,
		Strategy:     spec.Strategy,
		ImportPrefix: spec.ImportPrefix,
	}, filepath.Base(dir), spec.Module, spec.Vars)
	if err != nil {
		return FileChange{}, nil, err
	}
	return FileChange{Path: "platform.toml", Action: FileWrite, Content: content, Mode: 0644}, vars, nil
}

// daggerVersion is framework.DaggerVersion, seamed because `go test` binaries carry no
// dependency versions in their build info — tests stub it; production reads the real SDK.
var daggerVersion = framework.DaggerVersion

// planSpecFiles resolves the framework's contributed files with the init-time data —
// DaggerVersion from the linked SDK, ModulePath from an existing cue.mod (or the spec's
// ImportPrefix on a greenfield one), ImageBase derived from the repository. ModulePath
// is the operator's CUE namespace, deliberately separate from the GitHub repository.
func planSpecFiles(dir string, info *Info, spec fwscaffold.Spec) ([]FileChange, error) {
	if len(spec.Files) == 0 {
		return nil, nil
	}

	version := daggerVersion()
	if version == "" {
		return nil, errors.New("scaffold: could not determine the linked dagger SDK version")
	}
	modulePath := spec.ImportPrefix
	if framework.HasCueModule(dir) {
		path, err := framework.CueModulePath(dir)
		if err != nil {
			return nil, err
		}
		modulePath = path
	}

	resolved, err := fwscaffold.Resolve(spec.Files, fwscaffold.Data{
		DaggerVersion: version,
		ModulePath:    modulePath,
		ImageBase:     project.InferImageBase(info.Repository),
	})
	if err != nil {
		return nil, err
	}

	files := make([]FileChange, 0, len(resolved))
	for _, f := range resolved {
		files = append(files, fileChange(dir, f.Path, f.Content, f.Mode))
	}
	return files, nil
}

// Apply writes the plan's files, skipping any that would overwrite an existing file.
func (p *Plan) Apply() error {
	return p.write(false)
}

// ApplyOverwrite writes the plan's files, replacing existing files in place.
func (p *Plan) ApplyOverwrite() error {
	return p.write(true)
}

func (p *Plan) write(replace bool) error {
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
