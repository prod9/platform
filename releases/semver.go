package releases

import (
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/mod/semver"
)

type Semver struct{}

var _ Strategy = Semver{}

func (Semver) IsVersioned() bool { return true }

func (s Semver) IsValid(name string) bool {
	return semver.IsValid(name)
}

func (s Semver) NextName(prevName string, bump Bump) (string, error) {
	if prevName == "" {
		return "v0.1.0", nil
	}
	if bump == "" || bump == BumpAny {
		bump = BumpPatch
	}

	v := semver.Canonical(prevName)
	parts := strings.Split(v, ".")
	switch bump {
	case BumpPatch:
		n, err := strconv.Atoi(parts[2])
		if err != nil {
			return "", fmt.Errorf("%w: bad patch part: %q: %w", ErrBadVersion, parts[2], err)
		}
		parts[2] = strconv.Itoa(n + 1)

	case BumpMinor:
		n, err := strconv.Atoi(parts[1])
		if err != nil {
			return "", fmt.Errorf("%w: bad minor part: %q: %w", ErrBadVersion, parts[1], err)
		}
		parts[1] = strconv.Itoa(n + 1)
		parts[2] = "0"

	case BumpMajor:
		n, err := strconv.Atoi(parts[0][1:])
		if err != nil {
			return "", fmt.Errorf("%w: bad major part: %q: %w", ErrBadVersion, parts[0], err)
		}
		parts[0] = "v" + strconv.Itoa(n+1)
		parts[1] = "0"
		parts[2] = "0"

	default:
		return "", fmt.Errorf("%w: %q", ErrBadVersionBump, bump)
	}

	return strings.Join(parts, "."), nil
}
