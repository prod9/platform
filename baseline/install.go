package baseline

import (
	"bytes"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"text/template"
)

// InstallFile is a baseline file resolved for installation: Path is relative to the infra
// repo root (routing already applied), Body is the final bytes (templating already applied).
type InstallFile struct {
	Path string
	Body []byte
}

// TemplateData fills the placeholders in `.tmpl` baseline files at init time. Registry creds
// are prompted; DaggerVersion comes from the linked SDK; ModulePath is the infra repo's CUE
// module, needed for the `<module>/defaults` import in templated apps.
type TemplateData struct {
	DaggerVersion    string
	RegistryUsername string
	RegistryPassword string
	ModulePath       string
}

// Render resolves the selected baseline files for installation: it routes each to its
// destination by name prefix and renders `.tmpl` files through text/template with data.
// Non-template files pass through verbatim (their CUE braces must not meet the template
// engine). Output order is deterministic.
func Render(selected map[string][]byte, data TemplateData) ([]InstallFile, error) {
	out := make([]InstallFile, 0, len(selected))
	for _, name := range sortedNames(selected) {
		body, err := renderFile(name, selected[name], data)
		if err != nil {
			return nil, fmt.Errorf("baseline: render %s: %w", name, err)
		}
		out = append(out, InstallFile{Path: destPath(name), Body: body})
	}
	return out, nil
}

// destPath maps a baseline filename to its repo-relative destination: `apps-*` → `apps/`,
// `defaults-*` → `defaults/`, anything else → the repo root. The `.tmpl` suffix is dropped.
func destPath(name string) string {
	rel := strings.TrimSuffix(name, ".tmpl")
	switch {
	case strings.HasPrefix(rel, "apps-"):
		return filepath.Join("apps", strings.TrimPrefix(rel, "apps-"))
	case strings.HasPrefix(rel, "defaults-"):
		return filepath.Join("defaults", strings.TrimPrefix(rel, "defaults-"))
	default:
		return rel
	}
}

func renderFile(name string, body []byte, data TemplateData) ([]byte, error) {
	if !strings.HasSuffix(name, ".tmpl") {
		return body, nil
	}

	tmpl, err := template.New(name).Option("missingkey=error").Parse(string(body))
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func sortedNames(m map[string][]byte) []string {
	out := make([]string, 0, len(m))
	for name := range m {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}
