package system

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGroupSettingsOrdersFactsAndMasksSecrets(t *testing.T) {
	values := map[string]string{
		"server.public_url":         "https://platform.example.com",
		"github.org":                "prod9",
		"github.app_id":             "42",
		"github.app_slug":           "prod9-platform",
		"github.app_client_id":      "Iv1.abc",
		"github.app_private_key":    "private-key",
		"github.app_webhook_secret": "webhook-secret",
		"github.app_client_secret":  "client-secret",
		"registry.ghcr.io.token":    "ghp_token",
	}

	sections := groupSettings(values)

	require.Equal(t, []SettingSection{
		{Name: "Server", Facts: []SettingFact{
			{Key: "public_url", Value: "https://platform.example.com"},
			{Key: "org", Value: "prod9"},
		}},
		{Name: "GitHub App", Facts: []SettingFact{
			{Key: "app", Value: "prod9-platform (id 42)"},
			{Key: "client_id", Value: "Iv1.abc"},
			{Key: "private_key", Value: maskedSetting},
			{Key: "webhook_secret", Value: maskedSetting},
			{Key: "client_secret", Value: maskedSetting},
		}},
		{Name: "Registry", Facts: []SettingFact{
			{Key: "ghcr.io token", Value: maskedSetting},
		}},
	}, sections)

	encoded, err := json.Marshal(sections)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "private-key")
	require.NotContains(t, string(encoded), "webhook-secret")
	require.NotContains(t, string(encoded), "client-secret")
	require.NotContains(t, string(encoded), "ghp_token")
}
