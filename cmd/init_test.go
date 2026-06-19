package cmd

import (
	"testing"

	"fx.prodigy9.co/cmd/prompts"
	"platform.prodigy9.co/core/baseline"
)

// TestPickOptions drives the generic picker with a prefilled prompt session
// (args answer prompts in order), asserting toggles and choices land in vars.
func TestPickOptions(t *testing.T) {
	opts := []baseline.Option{
		{Key: "argocd", Kind: baseline.OptionToggle, Default: "false"},
		{Key: "nginx_gateway", Kind: baseline.OptionToggle, Default: "false"},
		{Key: "registry", Kind: baseline.OptionChoice, Variants: []string{"ghcr", "linode"}, Default: "ghcr"},
	}

	sess := prompts.New(nil, []string{"yes", "no", "linode"})
	vars := map[string]any{}
	pickOptions(sess, opts, vars)

	for key, want := range map[string]string{"argocd": "true", "nginx_gateway": "false", "registry": "linode"} {
		if vars[key] != want {
			t.Errorf("vars[%q] = %q, want %q", key, vars[key], want)
		}
	}
}
