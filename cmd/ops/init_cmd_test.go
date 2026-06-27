package ops

import (
	"testing"

	"fx.prodigy9.co/cmd/prompts"
	"platform.prodigy9.co/baseline"
)

// TestSelectComponents: an args comma-list overrides the defaults — exactly that subset of
// the built-in files is selected for install.
func TestSelectComponents(t *testing.T) {
	files := map[string][]byte{
		"cert-manager.platform":      []byte("a"),
		"flux.platform":              []byte("b"),
		"platform.cue":               []byte("c"),
		"nginx-gateway.platform":     []byte("d"),
		"nginx-gateway-exp.platform": []byte("e"),
	}

	sess := prompts.New(nil, []string{"cert-manager.platform,nginx-gateway.platform"})
	got := selectComponents(sess, files)

	if len(got) != 2 {
		t.Fatalf("selected %d components, want 2", len(got))
	}
	for _, name := range []string{"cert-manager.platform", "nginx-gateway.platform"} {
		if _, ok := got[name]; !ok {
			t.Errorf("missing selected component %q", name)
		}
	}
}

// TestSelectComponentsDefaults: with no args (non-interactive) the shipped Defaults install,
// and a present-but-non-default file (stable nginx-gateway) stays out.
func TestSelectComponentsDefaults(t *testing.T) {
	files := map[string][]byte{"nginx-gateway.platform": []byte("x")}
	for _, name := range baseline.Defaults {
		files[name] = []byte("x")
	}

	sess := prompts.New(nil, nil)
	got := selectComponents(sess, files)

	if len(got) != len(baseline.Defaults) {
		t.Fatalf("selected %d, want %d (defaults)", len(got), len(baseline.Defaults))
	}
	if _, ok := got["nginx-gateway.platform"]; ok {
		t.Error("stable nginx-gateway installed despite not being a default")
	}
}
