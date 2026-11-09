package releases

import (
	"slices"
	"testing"

	r "github.com/stretchr/testify/require"
	"platform.prodigy9.co/conf"
)

// TestParseLogOutput pins that every log line is a commit. The git boundary trims its
// output, so the last line — the oldest commit in the range — arrives with no newline,
// and a parser that emitted on newlines dropped it from every tag changelog.
func TestParseLogOutput(t *testing.T) {
	raw := "b4ebcef Require pulling first\nb37a2a5 Install corepack\n2d44ae0 Specify corepack"

	r.Equal(t, []CommitRef{
		{Hash: "b4ebcef", Subject: "Require pulling first"},
		{Hash: "b37a2a5", Subject: "Install corepack"},
		{Hash: "2d44ae0", Subject: "Specify corepack"},
	}, slices.Collect(parseLogOutput(raw)))
	r.Empty(t, slices.Collect(parseLogOutput("")))
}

// TestGenerateMessage pins the annotated-tag body, and with it the link form. The stored
// repository is scheme-less, so a bullet built by concatenation renders text no client
// resolves — which is how v0.9.17's tag shipped with dead links and nothing caught it.
func TestGenerateMessage(t *testing.T) {
	cfg := &conf.Model{Repository: "github.com/prod9/platform"}
	refs := []CommitRef{
		{Hash: "f3e0f9", Subject: "Sample message"},
		{Hash: "a1b2c3", Subject: "Another one"},
	}

	r.Equal(t, `v1.2.3

* [f3e0f9][https://github.com/prod9/platform/commit/f3e0f9] Sample message
* [a1b2c3][https://github.com/prod9/platform/commit/a1b2c3] Another one
`, generateMessage(cfg, "v1.2.3", refs))
}
