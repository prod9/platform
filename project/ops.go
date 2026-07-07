package project

import "errors"

// ErrNoOpsImage is returned when `publish` has no infra target: neither an
// explicit [ops] image nor a repository to infer one from.
var ErrNoOpsImage = errors.New("ops: no [ops] image and none inferable from repository")

// Ops configures where the rendered infra manifests land as the published image
// (infra is a builder — see the render-is-a-plain-image decision). Image/Tag fall
// back to convention: Image is inferred from Repository (github.com/x → ghcr.io/x),
// Tag defaults to "latest". Vars is the verbatim DSL \(var) table from [ops.vars] —
// a generic open map whose values keep their TOML type (string/int/bool); the
// per-component assembly layer and the DSL, not the processor, interpret them.
type Ops struct {
	Image string         `toml:"image,omitempty"`
	Tag   string         `toml:"tag,omitempty"`
	Vars  map[string]any `toml:"vars,omitempty"`
}

// Ref resolves the OCI reference the infra publish pushes to. tag overrides the
// configured/default Tag when non-empty (e.g. a per-env publish).
func (o Ops) Ref(tag string) (string, error) {
	if o.Image == "" {
		return "", ErrNoOpsImage
	}
	if tag == "" {
		tag = o.Tag
	}
	return o.Image + ":" + tag, nil
}
