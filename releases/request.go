package releases

import (
	"fmt"

	"golang.org/x/mod/semver"
)

// Request chooses one naming operation. Generate validates it before reading Git.
type Request interface {
	validate(Strategy) error
	nextName(Strategy, string) (string, error)
}

type Name string

func (n Name) validate(strategy Strategy) error {
	if _, ok := strategy.(Semver); !ok {
		return fmt.Errorf("%w: explicit names require semver", ErrBadStrategy)
	}
	name := string(n)
	if !semver.IsValid(name) || semver.Canonical(name) != name {
		return fmt.Errorf("%w: expected full vMAJOR.MINOR.PATCH with optional prerelease, without build metadata: %q", ErrBadVersion, name)
	}
	return nil
}

func (n Name) nextName(Strategy, string) (string, error) {
	return string(n), nil
}

func (b Bump) validate(Strategy) error {
	switch b {
	case "", BumpAny, BumpPatch, BumpMinor, BumpMajor:
		return nil
	default:
		return fmt.Errorf("%w: %q", ErrBadVersionBump, b)
	}
}

func (b Bump) nextName(strategy Strategy, previous string) (string, error) {
	return strategy.NextName(previous, b)
}
