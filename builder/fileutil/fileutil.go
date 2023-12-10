package fileutil

import (
	"os"
	"path/filepath"
	"strings"
)

func DetectFile(wd, filename string) (bool, error) {
	_, err := os.Stat(filepath.Join(wd, filename))
	switch {
	case os.IsNotExist(err):
		return false, nil
	case err != nil:
		return false, err
	default:
		return true, nil
	}
}

func WalkSubdirs(wd string, fn func(os.DirEntry) error) error {
	return filepath.WalkDir(wd, func(path string, dir os.DirEntry, err error) error {
		switch {
		case path == wd:
			return nil
		case !dir.IsDir():
			return nil
		case err != nil:
			return err
		case strings.HasPrefix(dir.Name(), "."):
			return filepath.SkipDir
		}

		if err = fn(dir); err != nil {
			return err
		} else {
			return filepath.SkipDir // only walk 1 lvl
		}
	})
}
