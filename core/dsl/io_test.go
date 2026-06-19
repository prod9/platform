package dsl

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

// --- archive builders for extract tests ---

func gzipBytes(t *testing.T, payload []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	if _, err := w.Write(payload); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func tarBytes(t *testing.T, name string, payload []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := tar.NewWriter(&buf)
	hdr := &tar.Header{Name: name, Mode: 0o644, Size: int64(len(payload))}
	if err := w.WriteHeader(hdr); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(payload); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func zipBytes(t *testing.T, name string, payload []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	f, err := w.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write(payload); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestExtract(t *testing.T) {
	yaml := []byte("kind: Deployment\n")

	cases := []struct {
		name   string
		buf    []byte
		member string
		want   []byte
	}{
		{"bare gzip", gzipBytes(t, yaml), "", yaml},
		{"tar member", tarBytes(t, "install.yaml", yaml), "install.yaml", yaml},
		{"zip member", zipBytes(t, "install.yaml", yaml), "install.yaml", yaml},
		{"tar.gz member", gzipBytes(t, tarBytes(t, "install.yaml", yaml)), "install.yaml", yaml},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := extract(tc.buf, tc.member)
			if err != nil {
				t.Fatalf("extract: %v", err)
			}
			if !bytes.Equal(got, tc.want) {
				t.Fatalf("extract = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestExtractErrors(t *testing.T) {
	yaml := []byte("kind: Deployment\n")

	cases := []struct {
		name   string
		buf    []byte
		member string
	}{
		{"archive needs member", tarBytes(t, "install.yaml", yaml), ""},
		{"member not found", tarBytes(t, "install.yaml", yaml), "absent.yaml"},
		{"member on plain buffer", yaml, "install.yaml"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := extract(tc.buf, tc.member); err == nil {
				t.Fatalf("extract(%q) expected error, got nil", tc.name)
			}
		})
	}
}

// download → interpolated URL → decode → edit → emit, end to end through Apply.
func TestApplyDownloadEmit(t *testing.T) {
	tmp := t.TempDir()
	fetched := "kind: Deployment\nmetadata:\n  name: d\nspec:\n  replicas: 1\n"

	directives := `
download "https://example.com/\(version)/install.yaml"
select .kind Deployment
set .spec.replicas "\(replicas)"
emit "nested/out.yaml"
`
	var gotURL string
	opts := Options{
		Vars:   Vars{"version": "v1.2.3", "replicas": 2}, // replicas is a typed int var
		OutDir: tmp,
		Fetch: func(url string) ([]byte, error) {
			gotURL = url
			return []byte(fetched), nil
		},
	}

	docs, err := Apply(directives, opts)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if gotURL != "https://example.com/v1.2.3/install.yaml" {
		t.Fatalf("fetched URL = %q, want interpolated", gotURL)
	}
	if v, _ := Get(docs[0], mustPath(t, ".spec.replicas")); v != 2 {
		t.Fatalf("in-memory replicas = %#v, want 2", v)
	}

	out, err := os.ReadFile(filepath.Join(tmp, "nested", "out.yaml"))
	if err != nil {
		t.Fatalf("read emitted: %v", err)
	}
	written := decodeDocs(t, string(out))
	if v, _ := Get(written[0], mustPath(t, ".spec.replicas")); v != 2 {
		t.Fatalf("emitted replicas = %#v, want 2", v)
	}
}

func TestEmitReplaceAndEscape(t *testing.T) {
	tmp := t.TempDir()
	docs := decodeDocs(t, "kind: First\n")

	if err := emit(tmp, "x.yaml", docs); err != nil {
		t.Fatal(err)
	}
	// last write wins — re-emit replaces, never appends
	if err := emit(tmp, "x.yaml", decodeDocs(t, "kind: Second\n")); err != nil {
		t.Fatal(err)
	}
	got := decodeDocs(t, readFile(t, filepath.Join(tmp, "x.yaml")))
	if len(got) != 1 {
		t.Fatalf("emitted %d docs, want 1 (replace, not append)", len(got))
	}
	if v, _ := Get(got[0], mustPath(t, ".kind")); v != "Second" {
		t.Fatalf("kind = %v, want Second", v)
	}

	for _, bad := range []string{"/etc/passwd", "../escape.yaml", "a/../../b.yaml"} {
		if err := emit(tmp, bad, docs); err == nil {
			t.Errorf("emit(%q) expected path-escape error, got nil", bad)
		}
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestApplyDecodeRoundTrip(t *testing.T) {
	// download then emit with no edits still round-trips through decode/encode
	tmp := t.TempDir()
	opts := Options{
		OutDir: tmp,
		Fetch:  func(string) ([]byte, error) { return []byte("kind: Plain\n"), nil },
	}
	if _, err := Apply("download \"u\"\nemit \"p.yaml\"", opts); err != nil {
		t.Fatal(err)
	}
	got := decodeDocs(t, readFile(t, filepath.Join(tmp, "p.yaml")))
	if v, _ := Get(got[0], mustPath(t, ".kind")); v != "Plain" {
		t.Fatalf("kind = %v, want Plain", v)
	}
}
