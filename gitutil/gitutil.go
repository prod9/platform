package gitutil

import (
	"github.com/go-git/go-git/v5"
	"path/filepath"
)

// TODO: Probably belongs in fx
func GitCommit(wd string) (string, error) {
	if wd, err := filepath.Abs(wd); err != nil {
		return "", err
	} else if repo, err := git.PlainOpen(wd); err != nil {
		return "", err
	} else if ref, err := repo.Head(); err != nil {
		return "", err
	} else {
		return ref.Hash().String(), nil
	}
}
