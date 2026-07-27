package conf

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

var (
	ErrNoPlatformConfig = errors.New("could not find platform.toml")
	PlatformFilename    = "platform.toml"
)

// ResolvePath locates the platform.toml governing wd: a wd naming a file is taken as the
// config itself, a directory is searched and then each parent up to the filesystem root.
// Walking up is what lets any command run from a module subdirectory of the project.
func ResolvePath(wd string) (string, error) {
	if !filepath.IsAbs(wd) {
		wd_, err := filepath.Abs(wd)
		if err != nil {
			return "", err
		}
		wd = wd_
	}

	info, err := os.Stat(wd)
	if err != nil {
		return "", err
	}

	if !info.IsDir() {
		return wd, nil
	}

	filename := filepath.Join(wd, PlatformFilename)
	info, err = os.Stat(filename)
	if err == nil && !info.IsDir() {
		return filename, err
	}

	// Only an absent platform.toml continues the walk; a stat that failed for any other
	// reason is a real filesystem fault and stops here rather than climbing past it.
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}

	parent := filepath.Dir(wd)
	if parent == wd {
		return "", ErrNoPlatformConfig
	}

	return ResolvePath(parent)
}
