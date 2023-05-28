package config

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

var ErrPlatformFileNotFound = errors.New("could not find platform.toml")

func ResolvePath(wd string) (string, error) {
	info, err := os.Stat(wd)
	if err != nil {
		return "", err
	}

	if !info.IsDir() { // found a file
		return wd, nil
	}

	filename := filepath.Join(wd, "platform.toml")
	info, err = os.Stat(filename)
	if err == nil && !info.IsDir() {
		// we found the file
		return filename, err
	}
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	// if err == nil && info.IsDir(), keep looking
	// if err != nil && errors.Is(err, fs.ErrNotExist), keep looking

	parentWD := filepath.Dir(wd)
	if parentWD == wd {
		// no more parents :(
		return "", ErrPlatformFileNotFound
	}

	return ResolvePath(parentWD)
}
