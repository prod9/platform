package dsl

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// httpGet is the default download fetcher: a plain GET, body read whole. Tests
// inject a fixture fetcher through Options.Fetch instead of hitting the network.
func httpGet(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

// extract decompresses and unarchives the buffer, detecting the container by
// magic bytes (never the URL extension) in two layers: gzip compression first,
// then a tar/zip archive. member selects an entry inside an archive and is
// required for one — a bare compressed stream (e.g. a plain .gz) takes no member.
func extract(buf []byte, member string) ([]byte, error) {
	data := buf
	if isGzip(data) {
		d, err := gunzip(data)
		if err != nil {
			return nil, err
		}
		data = d
	}

	switch {
	case isZip(data):
		return zipMember(data, member)
	case isTar(data):
		return tarMember(data, member)
	default:
		if member != "" {
			return nil, fmt.Errorf("extract: %q given but buffer is not an archive", member)
		}
		return data, nil
	}
}

func isGzip(b []byte) bool { return len(b) >= 2 && b[0] == 0x1f && b[1] == 0x8b }
func isZip(b []byte) bool  { return len(b) >= 2 && b[0] == 'P' && b[1] == 'K' }
func isTar(b []byte) bool  { return len(b) >= 262 && string(b[257:262]) == "ustar" }

func gunzip(b []byte) ([]byte, error) {
	r, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}

func tarMember(b []byte, member string) ([]byte, error) {
	if member == "" {
		return nil, fmt.Errorf("extract: tar archive requires a member path")
	}

	r := tar.NewReader(bytes.NewReader(b))
	for {
		hdr, err := r.Next()
		if errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("extract: member %q not found in tar", member)
		}
		if err != nil {
			return nil, err
		}
		if hdr.Name == member {
			return io.ReadAll(r)
		}
	}
}

func zipMember(b []byte, member string) ([]byte, error) {
	if member == "" {
		return nil, fmt.Errorf("extract: zip archive requires a member path")
	}

	zr, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		return nil, err
	}
	for _, f := range zr.File {
		if f.Name != member {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		defer rc.Close()
		return io.ReadAll(rc)
	}
	return nil, fmt.Errorf("extract: member %q not found in zip", member)
}

// emit writes the working buffer to name under outDir, replacing any existing
// file (truncate, never append). name is relative and may not escape outDir;
// intermediate directories are created.
func emit(outDir, name string, docs []map[string]any) error {
	if err := checkRelPath(name); err != nil {
		return err
	}

	data, err := encodeStream(docs)
	if err != nil {
		return err
	}

	path := filepath.Join(outDir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// checkRelPath rejects absolute paths and any name that escapes the output dir.
func checkRelPath(name string) error {
	if name == "" {
		return fmt.Errorf("emit: empty filename")
	}
	if filepath.IsAbs(name) {
		return fmt.Errorf("emit: absolute path %q not allowed", name)
	}

	clean := filepath.Clean(name)
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return fmt.Errorf("emit: path %q escapes output dir", name)
	}
	return nil
}

// decodeStream splits a multi-document YAML buffer into one map per document,
// skipping empty documents (a bare `---` or trailing separator).
func decodeStream(buf []byte) ([]map[string]any, error) {
	dec := yaml.NewDecoder(bytes.NewReader(buf))

	var docs []map[string]any
	for {
		var d map[string]any
		err := dec.Decode(&d)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		if d != nil {
			docs = append(docs, d)
		}
	}
	return docs, nil
}

// encodeStream renders docs as a multi-document YAML stream, the inverse of
// decodeStream.
func encodeStream(docs []map[string]any) ([]byte, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)

	for _, d := range docs {
		if err := enc.Encode(d); err != nil {
			return nil, err
		}
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
