package cmd

import (
	"testing"

	"fx.prodigy9.co/cmd/prompts"
	"platform.prodigy9.co/core/baseline"
)

// TestPickOptions drives the generic picker with a prefilled prompt session (args answer
// prompts in order): each choice consumes one variant arg; all toggles fold into one
// MultiSelect consuming a single comma-separated arg naming the keys to enable.
func TestPickOptions(t *testing.T) {
	opts := []baseline.Option{
		{Key: "argocd", Kind: baseline.OptionToggle, Default: "false"},
		{Key: "nginx_gateway", Kind: baseline.OptionToggle, Default: "false"},
		{Key: "registry", Kind: baseline.OptionChoice, Variants: []string{"ghcr", "linode"}, Default: "ghcr"},
	}

	// "linode" answers the registry choice; "argocd" is the toggle comma-list — argocd on,
	// nginx_gateway omitted, so off.
	sess := prompts.New(nil, []string{"linode", "argocd"})
	vars := map[string]any{}
	pickOptions(sess, opts, vars)

	for key, want := range map[string]string{"argocd": "true", "nginx_gateway": "false", "registry": "linode"} {
		if vars[key] != want {
			t.Errorf("vars[%q] = %q, want %q", key, vars[key], want)
		}
	}
}

// TestPickTogglesDefaults checks the pre-check path: with no args (non-interactive) the
// toggle MultiSelect returns its defaults, so a toggle enabled in vars stays enabled.
func TestPickTogglesDefaults(t *testing.T) {
	toggles := []baseline.Option{
		{Key: "argocd", Kind: baseline.OptionToggle, Default: "false"},
		{Key: "ngf_experimental", Kind: baseline.OptionToggle, Default: "false"},
	}

	sess := prompts.New(nil, nil) // no args, non-interactive → MultiSelect returns defaults
	vars := map[string]any{"argocd": "false", "ngf_experimental": "true"}
	pickToggles(sess, toggles, vars)

	if vars["ngf_experimental"] != "true" {
		t.Errorf("default-on toggle dropped: ngf_experimental = %q, want true", vars["ngf_experimental"])
	}
	if vars["argocd"] != "false" {
		t.Errorf("default-off toggle enabled: argocd = %q, want false", vars["argocd"])
	}
}
