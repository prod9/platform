package framework

import (
	"path/filepath"

	"platform.prodigy9.co/conf"
)

// defaultModule is the single-module platform.toml contribution shared by the
// frameworks, with WorkDir set per layout (workspace layouts nest the module under
// ./<name>, basic ones sit at the root). The driver keys it by the directory name.
func defaultModule(fw Framework, wd string) *conf.Module {
	mod := *conf.ModuleDefaults
	mod.Framework = fw.Name()
	if fw.Layout() == LayoutWorkspace {
		mod.WorkDir = "./" + filepath.Base(wd)
	} else {
		mod.WorkDir = "."
	}
	return &mod
}
