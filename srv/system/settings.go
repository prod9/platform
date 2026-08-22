package system

import (
	"context"

	"fx.prodigy9.co/app/settings"
	"fx.prodigy9.co/data"
)

const maskedSetting = "········································"

var settingKeys = []string{
	"server.public_url",
	"github.org",
	"github.app_id",
	"github.app_slug",
	"github.app_client_id",
	"github.app_private_key",
	"github.app_webhook_secret",
	"github.app_client_secret",
	"registry.ghcr.io.token",
}

type SettingFact struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type SettingSection struct {
	Name  string        `json:"name"`
	Facts []SettingFact `json:"facts"`
}

func Settings(ctx context.Context) ([]SettingSection, error) {
	values := map[string]string{}
	err := data.Run(ctx, func(scope data.Scope) error {
		for _, key := range settingKeys {
			value, err := settings.Get(scope.Context(), key, "")
			if err != nil {
				return err
			}
			values[key] = value
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return groupSettings(values), nil
}

func groupSettings(values map[string]string) []SettingSection {
	return []SettingSection{
		{Name: "Server", Facts: []SettingFact{
			{Key: "public_url", Value: values["server.public_url"]},
			{Key: "org", Value: values["github.org"]},
		}},
		{Name: "GitHub App", Facts: []SettingFact{
			{Key: "app", Value: appSetting(values)},
			{Key: "client_id", Value: values["github.app_client_id"]},
			{Key: "private_key", Value: maskPresent(values["github.app_private_key"])},
			{Key: "webhook_secret", Value: maskPresent(values["github.app_webhook_secret"])},
			{Key: "client_secret", Value: maskPresent(values["github.app_client_secret"])},
		}},
		{Name: "Registry", Facts: []SettingFact{
			{Key: "ghcr.io token", Value: maskPresent(values["registry.ghcr.io.token"])},
		}},
	}
}

func appSetting(values map[string]string) string {
	slug := values["github.app_slug"]
	id := values["github.app_id"]
	if slug == "" || id == "" {
		return ""
	}
	return slug + " (id " + id + ")"
}

func maskPresent(value string) string {
	if value == "" {
		return ""
	}
	return maskedSetting
}
