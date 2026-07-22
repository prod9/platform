package engine

import (
	"fx.prodigy9.co/cmd/prompts"
	fxconfig "fx.prodigy9.co/config"
	"platform.prodigy9.co/conf"
)

// buildArch answers the only arch question there is: does this image outlive the box that
// built it? A plain build is discarded here and takes the host arch for speed — except
// under CI, where the build exists to be pushed and must carry the servers' arch. The rule
// is unexported because it is only ever an input to an entrypoint that is about to build.
func (e *Engine) buildArch(cfg *conf.Model) string {
	if fxconfig.Get(e.cfg, prompts.CIConfig) {
		return cfg.PublishArch
	}
	return cfg.LocalArch
}
