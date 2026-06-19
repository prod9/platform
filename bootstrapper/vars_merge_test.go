package bootstrapper

import (
	"testing"

	r "github.com/stretchr/testify/require"
)

func TestMergeOpsVars_appendsMissingPreservesExisting(t *testing.T) {
	existing := `maintainer = "a <a@b.co>"
repository = "github.com/prod9/infra"

[ops.vars]
# operator bumped this for a CVE
cert_manager_version = "v1.16.0"
`
	defaults := map[string]any{
		"cert_manager_version": "v1.15.0", // operator's value must win
		"flux_version":         "v2.3.0",  // new key, appended
		"nginx_experimental":   "true",    // new key, appended
	}

	merged, changes := mergeOpsVars([]byte(existing), defaults)
	got := string(merged)

	// Operator's value and their comment survive untouched.
	r.Contains(t, got, `# operator bumped this for a CVE`)
	r.Contains(t, got, `cert_manager_version = "v1.16.0"`)
	r.NotContains(t, got, `v1.15.0`)

	// New keys land under the section, sorted, ahead of any later table.
	r.Contains(t, got, `flux_version = "v2.3.0"`)
	r.Contains(t, got, `nginx_experimental = "true"`)

	// Change report: one entry per default key, preserved vs appended.
	byKey := map[string]VarChange{}
	for _, c := range changes {
		byKey[c.Key] = c
	}
	r.False(t, byKey["cert_manager_version"].Appended)
	r.True(t, byKey["flux_version"].Appended)
	r.True(t, byKey["nginx_experimental"].Appended)
}

func TestMergeOpsVars_encodesByType(t *testing.T) {
	defaults := map[string]any{
		"firewall_id": "11222746", // string → quoted, even though it looks numeric
		"replicas":    int64(3),   // int → bare
		"debug":       true,       // bool → bare
	}

	merged, _ := mergeOpsVars([]byte("[ops.vars]\n"), defaults)
	got := string(merged)

	r.Contains(t, got, `firewall_id = "11222746"`)
	r.Contains(t, got, `replicas = 3`)
	r.Contains(t, got, `debug = true`)
	r.NotContains(t, got, `replicas = "3"`)
}

func TestMergeOpsVars_createsSectionWhenAbsent(t *testing.T) {
	existing := `repository = "github.com/prod9/infra"
`
	defaults := map[string]any{"flux_version": "v2.3.0"}

	merged, changes := mergeOpsVars([]byte(existing), defaults)
	got := string(merged)

	r.Contains(t, got, "[ops.vars]")
	r.Contains(t, got, `flux_version = "v2.3.0"`)
	r.Len(t, changes, 1)
	r.True(t, changes[0].Appended)
}

func TestMergeOpsVars_emptyDefaultsIsNoop(t *testing.T) {
	existing := `repository = "github.com/prod9/infra"

[ops.vars]
flux_version = "v2.3.0"
`
	merged, changes := mergeOpsVars([]byte(existing), nil)
	r.Equal(t, existing, string(merged))
	r.Empty(t, changes)
}

func TestMergeOpsVars_idempotent(t *testing.T) {
	existing := `[ops.vars]
flux_version = "v2.3.0"
`
	defaults := map[string]any{"flux_version": "v2.3.0", "cert_manager_version": "v1.16.0"}

	once, _ := mergeOpsVars([]byte(existing), defaults)
	twice, changes := mergeOpsVars(once, defaults)
	r.Equal(t, string(once), string(twice))
	// Second pass appends nothing — both keys already present.
	for _, c := range changes {
		r.False(t, c.Appended, "key %q should be preserved on re-merge", c.Key)
	}
}
