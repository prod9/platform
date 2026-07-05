package baseline

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRenderRoutesByPrefix(t *testing.T) {
	selected := map[string][]byte{
		"apps-cert-manager.platform": []byte("cert-manager directive"),
		"defaults-basics.cue.tmpl":   []byte("package defaults"),
		"platform.toml":              []byte("root file"),
	}

	files, err := Render(selected, TemplateData{})
	require.NoError(t, err)

	got := map[string]string{}
	for _, f := range files {
		got[f.Path] = string(f.Body)
	}
	require.Equal(t, "cert-manager directive", got["apps/cert-manager.platform"])
	require.Equal(t, "package defaults", got["defaults/basics.cue"])
	require.Equal(t, "root file", got["platform.toml"])
}

func TestRenderInterpolatesTemplates(t *testing.T) {
	selected := map[string][]byte{
		"defaults-basics.cue.tmpl": []byte(
			`#registry_username: "{{ .RegistryUsername }}"` + "\n" +
				`#registry_password: "{{ .RegistryPassword }}"`),
		"apps-platform.cue.tmpl": []byte(
			`import "{{ .ModulePath }}/defaults"` + "\n" +
				`#image: "registry.dagger.io/engine:{{ .DaggerVersion }}"`),
	}
	data := TemplateData{
		DaggerVersion:    "v0.21.7",
		RegistryUsername: "x9-github",
		RegistryPassword: "secret",
		ModulePath:       "prodigy9.co",
	}

	files, err := Render(selected, data)
	require.NoError(t, err)

	got := map[string]string{}
	for _, f := range files {
		got[f.Path] = string(f.Body)
	}
	require.Equal(t,
		"#registry_username: \"x9-github\"\n#registry_password: \"secret\"",
		got["defaults/basics.cue"])
	require.Equal(t,
		"import \"prodigy9.co/defaults\"\n#image: \"registry.dagger.io/engine:v0.21.7\"",
		got["apps/platform.cue"])
}

func TestRenderRejectsUnknownPlaceholder(t *testing.T) {
	selected := map[string][]byte{"defaults-basics.cue.tmpl": []byte(`{{ .Nonexistent }}`)}
	_, err := Render(selected, TemplateData{})
	require.Error(t, err)
}

func TestRenderPassesNonTemplateVerbatim(t *testing.T) {
	// A bare .cue with no .tmpl suffix must not be run through the template engine —
	// its CUE braces would otherwise risk misparsing.
	selected := map[string][]byte{"apps-flux.platform": []byte(`{{ this is not a template }}`)}
	files, err := Render(selected, TemplateData{})
	require.NoError(t, err)
	require.Len(t, files, 1)
	require.Equal(t, `{{ this is not a template }}`, string(files[0].Body))
}
