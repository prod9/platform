package rocks

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// untarGzip writes the archive's directories and regular files under dir. An entry of any
// other type, or with a path that is not local to dir (absolute, or climbing out through
// ".."), fails the unpack.
func untarGzip(archive []byte, dir string) error {
	unzipped, gzipErr := gzip.NewReader(bytes.NewReader(archive))
	if gzipErr != nil {
		return gzipErr
	}

	entries := tar.NewReader(unzipped)
	for {
		header, err := entries.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		if !filepath.IsLocal(header.Name) {
			return fmt.Errorf("archive entry %q escapes the rock directory", header.Name)
		}
		err = writeEntry(entries, header, filepath.Join(dir, header.Name))
		if err != nil {
			return err
		}
	}
}

func writeEntry(entries *tar.Reader, header *tar.Header, target string) error {
	switch header.Typeflag {
	case tar.TypeDir:
		return os.MkdirAll(target, 0o755)
	case tar.TypeReg:
		err := os.MkdirAll(filepath.Dir(target), 0o755)
		if err != nil {
			return err
		}
		content, err := io.ReadAll(entries)
		if err != nil {
			return err
		}
		return os.WriteFile(target, content, 0o644)
	default:
		return fmt.Errorf("archive entry %q has unsupported type %q", header.Name, header.Typeflag)
	}
}
