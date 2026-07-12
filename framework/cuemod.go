package framework

import (
	"fmt"

	"cuelang.org/go/cue"
	"platform.prodigy9.co/cuemod"
	"platform.prodigy9.co/framework/scaffold"
)

// cueModFile is the Infra framework's greenfield cue.mod/module.cue contribution: the
// operator's module path (a {{.ModulePath}} hole the driver resolves), the linked CUE
// evaluator's language version (so render never demands a newer language than it links),
// and the pinned infra-defs dependency the baseline apps import.
func cueModFile() scaffold.File {
	content := fmt.Sprintf(
		"module: \"{{.ModulePath}}\"\nlanguage: {\n\tversion: %q\n}\ndeps: {\n\t%q: {\n\t\tv: %q\n\t}\n}\n",
		cue.LanguageVersion(), DefsModule, DefsVersion)
	return scaffold.File{Path: cuemod.ModuleFile + ".tmpl", Content: []byte(content), Mode: 0644}
}
