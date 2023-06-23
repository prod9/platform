package config

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

var ErrNoPlatformConfig = errors.New("could not find platform.toml")

func ResolvePath(wd string) (string, error) {
	if !filepath.IsAbs(wd) {
		if wd_, err := filepath.Abs(wd); err != nil {
			return "", err
		} else {
			wd = wd_
		}
	}

	info, err := os.Stat(wd)
	if err != nil {
		return "", err
	}

	if !info.IsDir() { // found a file
		return wd, nil
	}

	// try looking in current folder
	filename := filepath.Join(wd, "platform.toml")
	info, err = os.Stat(filename)
	if err == nil && !info.IsDir() {
		// we found the file
		return filename, err
	}

	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	// keep looking in parent folder if:
	//   * err == nil && info.IsDir()
	//   * err != nil && errors.Is(err, fs.ErrNotExist)

	parent := filepath.Dir(wd)
	if parent == wd {
		// no more parents :(
		return "", ErrNoPlatformConfig
	}

	return ResolvePath(parent)
}
