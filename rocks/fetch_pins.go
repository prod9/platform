package rocks

import (
	"context"
	"sort"
)

// FetchPins makes sure every rock a [rocks] table pins is in the cache. Pins maps a rock
// name to its version; Username and Password are as on Fetch.
type FetchPins struct {
	Pins               map[string]string
	Username, Password string
}

// Run returns the rocks' cache directories in rock-name order.
func (f FetchPins) Run(ctx context.Context) ([]string, error) {
	names := make([]string, 0, len(f.Pins))
	for name := range f.Pins {
		names = append(names, name)
	}
	sort.Strings(names)

	dirs := make([]string, 0, len(names))
	for _, name := range names {
		fetch := Fetch{Name: name, Version: f.Pins[name], Username: f.Username, Password: f.Password}
		dir, err := fetch.Run(ctx)
		if err != nil {
			return nil, err
		}
		dirs = append(dirs, dir)
	}
	return dirs, nil
}
