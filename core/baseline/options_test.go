package baseline

import (
	"testing"

	r "github.com/stretchr/testify/require"
)

// the discovered baseline file set used across the selection tests.
var sampleFiles = []string{
	"nginx-gateway+toleration.platform",
	"flux.platform",
	"nginx-gateway@experimental.platform",
	"cert-manager.platform",
	"nginx-gateway@stable.platform",
}

func TestScanOptions(t *testing.T) {
	opts := ScanOptions(sampleFiles)

	// Plain files (flux, cert-manager) declare no option; only the @ choice
	// group and the + toggle surface. Returned sorted by key.
	r.Len(t, opts, 2)

	r.Equal(t, "nginx-gateway", opts[0].Key)
	r.Equal(t, OptionChoice, opts[0].Kind)
	r.Equal(t, []string{"experimental", "stable"}, opts[0].Variants) // sorted
	r.Equal(t, "experimental", opts[0].Default)                      // lexically first

	r.Equal(t, "toleration", opts[1].Key)
	r.Equal(t, OptionToggle, opts[1].Kind)
	r.Empty(t, opts[1].Variants)
	r.Equal(t, "false", opts[1].Default)
}

func TestSelect_defaultsWhenUnset(t *testing.T) {
	// No vars: plain files always apply, the choice group falls back to its
	// default variant, and toggles stay off.
	got, err := Select(sampleFiles, nil)
	r.NoError(t, err)
	r.Equal(t, []string{
		"cert-manager.platform",
		"flux.platform",
		"nginx-gateway@experimental.platform",
	}, got)
}

func TestSelect_honoursChoiceAndToggle(t *testing.T) {
	got, err := Select(sampleFiles, map[string]string{
		"nginx-gateway": "stable",
		"toleration":    "true",
	})
	r.NoError(t, err)
	r.Equal(t, []string{
		"cert-manager.platform",
		"flux.platform",
		"nginx-gateway+toleration.platform",
		"nginx-gateway@stable.platform",
	}, got)
}

func TestSelect_toggleOffUnlessTrue(t *testing.T) {
	// Any value other than "true" leaves the overlay out — only the exact
	// string-bool enables it.
	got, err := Select(sampleFiles, map[string]string{"toleration": "yes"})
	r.NoError(t, err)
	r.NotContains(t, got, "nginx-gateway+toleration.platform")
}

func TestSelect_unknownVariantIsError(t *testing.T) {
	// A typo'd choice must fail loudly, not silently fall back to a default.
	_, err := Select(sampleFiles, map[string]string{"nginx-gateway": "bogus"})
	r.Error(t, err)
}
