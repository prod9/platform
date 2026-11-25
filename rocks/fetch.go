package rocks

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// Fetch makes sure one rock version is unpacked in the cache. Username and Password are
// the registry credentials platform reads for publishing (REGISTRY_USERNAME and
// REGISTRY_PASSWORD); empty, the pull is anonymous.
type Fetch struct {
	Name, Version      string
	Username, Password string
}

// Run returns the rock's cache directory, pulling and unpacking the rock first when the
// version is not there yet. A version directory exists only once it is complete, so its
// presence is the whole cache check: a rock version is immutable once published.
func (f Fetch) Run(ctx context.Context) (string, error) {
	if !namePattern.MatchString(f.Name) {
		return "", fmt.Errorf("rocks: %q is not a rock name", f.Name)
	}
	if !versionPattern.MatchString(f.Version) {
		return "", fmt.Errorf("rocks: %s: %q is not a version tag", f.Name, f.Version)
	}
	if f.Username != "" && f.Password == "" {
		return "", fmt.Errorf("rocks: %s: registry password is required with a username", f.Name)
	}
	root, err := cacheRoot()
	if err != nil {
		return "", err
	}

	parent := filepath.Join(root, "platform", "rocks", f.Name)
	dir := filepath.Join(parent, f.Version)
	_, err = os.Stat(dir)
	if err == nil {
		return dir, nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf("rocks: %w", err)
	}

	layer, err := f.pull(ctx)
	if err != nil {
		return "", fmt.Errorf("rocks: %s:%s: %w", f.Name, f.Version, err)
	}
	err = install(layer, parent, dir)
	if err != nil {
		return "", fmt.Errorf("rocks: %s:%s: %w", f.Name, f.Version, err)
	}
	return dir, nil
}

// cacheRoot is $XDG_CACHE_HOME, or ~/.cache when it is unset. The XDG base directory
// spec makes a relative value invalid, so one is ignored like an unset one.
func cacheRoot() (string, error) {
	fromEnv := os.Getenv("XDG_CACHE_HOME")
	if filepath.IsAbs(fromEnv) {
		return fromEnv, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("rocks: find the cache directory: %w", err)
	}
	return filepath.Join(home, ".cache"), nil
}

// install unpacks layer into a temporary directory beside dir and renames it into place,
// so a failed unpack leaves nothing behind and dir never holds a partial rock.
func install(layer []byte, parent, dir string) (err error) {
	err = os.MkdirAll(parent, 0o755)
	if err != nil {
		return err
	}
	unpacking, err := os.MkdirTemp(parent, "."+filepath.Base(dir)+"-")
	if err != nil {
		return err
	}
	// After the rename there is nothing left to remove, and RemoveAll reports no error.
	defer func() { err = errors.Join(err, os.RemoveAll(unpacking)) }()

	err = untarGzip(layer, unpacking)
	if err != nil {
		return err
	}
	return os.Rename(unpacking, dir)
}
