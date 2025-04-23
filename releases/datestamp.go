package releases

import (
	"fmt"
	"platform.prodigy9.co/releases/dateref"
)

type Datestamp struct{}

var _ Strategy = Datestamp{}

func (d Datestamp) IsValid(name string) bool {
	_, err := dateref.Parse(name)
	return err == nil
}

func (d Datestamp) NextName(prevName string, comp NameComponent) (string, error) {
	if prevName == "" {
		return dateref.Now(0).String(), nil
	}

	ref, err := dateref.Parse(prevName)
	if err != nil {
		return "", fmt.Errorf("%w: could not parse %q: %w", ErrBadVersion, prevName, err)
	} else if ref.IsToday() {
		return ref.NextCounter().String(), nil
	} else {
		return dateref.Now(0).String(), nil
	}
}
