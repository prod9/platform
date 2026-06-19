package baseline

import (
	"embed"
	"io/fs"
)

//go:embed files/*.platform
var embedded embed.FS

// DefaultVars is the baseline's shipped [ops.vars]: platform's opinionated
// defaults (version pins and knobs) for the embedded directive set. Bootstrap
// seeds these into a fresh platform.toml and merges them on re-bootstrap (new
// keys appended, operator values preserved). Each key is consumed by a directive
// file via \(var) interpolation.
var DefaultVars = map[string]string{
	"cert_manager_version": "v1.20.2",
}

// EmbeddedFiles returns the baseline directive files shipped in the binary, keyed
// by filename. Bootstrap writes these into a target infra repo's baseline/.
func EmbeddedFiles() (map[string][]byte, error) {
	entries, err := embedded.ReadDir("files")
	if err != nil {
		return nil, err
	}

	files := map[string][]byte{}
	for _, e := range entries {
		content, err := fs.ReadFile(embedded, "files/"+e.Name())
		if err != nil {
			return nil, err
		}
		files[e.Name()] = content
	}
	return files, nil
}
