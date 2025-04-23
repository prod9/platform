package releases

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/mod/semver"
)

type Semver struct{}

var _ Strategy = Semver{}

func (s Semver) IsValid(name string) bool {
	return semver.IsValid(name)
}

func (s Semver) NextName(prevName string, comp NameComponent) (string, error) {
	if prevName == "" {
		return "v0.1.0", nil
	}
	if comp == "" || comp == NameAny {
		comp = NamePatch
	}

	v := semver.Canonical(prevName)
	parts := strings.Split(v, ".")
	switch comp {
	case NamePatch:
		if n, err := strconv.Atoi(parts[2]); err != nil {
			return "", fmt.Errorf("%w: bad patch part: %q: %w", ErrBadVersion, parts[2], err)
		} else {
			parts[2] = strconv.Itoa(n + 1)
		}

	case NameMinor:
		if n, err := strconv.Atoi(parts[1]); err != nil {
			return "", fmt.Errorf("%w: bad minor part: %q: %w", ErrBadVersion, parts[1], err)
		} else {
			parts[1] = strconv.Itoa(n + 1)
			parts[2] = "0"
		}

	case NameMajor:
		if n, err := strconv.Atoi(parts[0][1:]); err != nil {
			return "", fmt.Errorf("%w: bad major part: %q: %w", ErrBadVersion, parts[0], err)
		} else {
			parts[0] = "v" + strconv.Itoa(n+1)
			parts[1] = "0"
			parts[2] = "0"
		}

	default:
		return "", errors.New("invalid version component: " + string(comp))
	}

	return strings.Join(parts, "."), nil
}
