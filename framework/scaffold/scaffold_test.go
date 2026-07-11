package scaffold

import (
	"testing"

	r "github.com/stretchr/testify/require"
)

func TestResolveResolvesTemplates(t *testing.T) {
	files := []File{{
		Path: "apps/platform.cue.tmpl",
		Content: []byte(`import "{{ .ModulePath }}/defaults"` + "\n" +
			`#image: "registry.dagger.io/engine:{{ .DaggerVersion }}"`),
		Mode: 0644,
	}}
	data := Data{DaggerVersion: "v0.21.7", ModulePath: "prodigy9.co"}

	out, err := Resolve(files, data)
	r.NoError(t, err)
	r.Len(t, out, 1)

	// The .tmpl suffix is stripped and the holes are filled; Mode carries over.
	r.Equal(t, "apps/platform.cue", out[0].Path)
	r.Equal(t,
		"import \"prodigy9.co/defaults\"\n#image: \"registry.dagger.io/engine:v0.21.7\"",
		string(out[0].Content))
	r.Equal(t, files[0].Mode, out[0].Mode)
}

func TestResolveRejectsUnknownPlaceholder(t *testing.T) {
	files := []File{{Path: "defaults/basics.cue.tmpl", Content: []byte(`{{ .Nonexistent }}`)}}
	_, err := Resolve(files, Data{})
	r.Error(t, err)
}

func TestResolvePassesNonTemplateVerbatim(t *testing.T) {
	// A file without the .tmpl suffix must not meet the template engine — its CUE
	// braces would otherwise risk misparsing.
	files := []File{{Path: "apps/flux.platform", Content: []byte(`{{ not a template }}`)}}
	out, err := Resolve(files, Data{})
	r.NoError(t, err)
	r.Len(t, out, 1)
	r.Equal(t, "apps/flux.platform", out[0].Path)
	r.Equal(t, `{{ not a template }}`, string(out[0].Content))
}

func TestResolvePreservesInputOrder(t *testing.T) {
	files := []File{
		{Path: "b.txt", Content: []byte("b")},
		{Path: "a.txt.tmpl", Content: []byte("a")},
	}
	out, err := Resolve(files, Data{})
	r.NoError(t, err)
	r.Equal(t, "b.txt", out[0].Path)
	r.Equal(t, "a.txt", out[1].Path)
}
