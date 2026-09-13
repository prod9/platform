package builds

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// Admission distinguishes omission from an explicit empty/null selection.
// docs/spec/platform-server.md, "A webhook build is whole-repo".
func TestModuleSelectionRejectsExplicitEmpty(t *testing.T) {
	for _, raw := range []string{`null`, `[]`, `["api","api"]`} {
		t.Run(raw, func(t *testing.T) {
			var selection ModuleSelection
			require.Error(t, json.Unmarshal([]byte(raw), &selection))
		})
	}
}

func TestModuleSelectionAllowsOmissionAndDistinctNames(t *testing.T) {
	var request struct {
		Modules ModuleSelection `json:"modules"`
	}
	require.NoError(t, json.Unmarshal([]byte(`{}`), &request))
	require.Nil(t, request.Modules)
	require.NoError(t, json.Unmarshal([]byte(`{"modules":["api","web"]}`), &request))
	require.Equal(t, ModuleSelection{"api", "web"}, request.Modules)
}
