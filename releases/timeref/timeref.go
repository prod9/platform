package timeref

import (
	"errors"
	"regexp"
	"time"
)

var (
	ErrInvalidTimeRef = errors.New("invalid timeref version")

	refRegexp = regexp.MustCompile("^v([0-9]{12})$")
)

// Jan 2 15:04:05 2006 MST
const refFormat = "v200601021504"

func Now() string {
	return time.Now().Format(refFormat)
}

// Parse returns the moment a timeref names.
func Parse(str string) (time.Time, error) {
	if !refRegexp.MatchString(str) {
		return time.Time{}, ErrInvalidTimeRef
	}
	return time.Parse(refFormat, str)
}

func IsValid(str string) bool {
	return refRegexp.MatchString(str)
}
