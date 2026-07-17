// Package migrate composes fragment migration sources for the srv layer.
package migrate

import (
	"slices"
	"strings"

	"fx.prodigy9.co/data/migrator"
)

// Merged combines migration sources into one, re-sorted by name so timestamps
// interleave across fragments exactly as they would in a single directory.
func Merged(sources ...migrator.Source) migrator.Source {
	return func() ([]migrator.Migration, error) {
		all := []migrator.Migration{}
		for _, source := range sources {
			migrations, err := source()
			if err != nil {
				return nil, err
			}
			all = append(all, migrations...)
		}

		slices.SortFunc(all, func(a, b migrator.Migration) int {
			return strings.Compare(a.Name, b.Name)
		})
		return all, nil
	}
}
