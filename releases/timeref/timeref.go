package timeref

import (
	"regexp"
	"time"
)

var refRegexp = regexp.MustCompile("^v([0-9]{12})$")

// Jan 2 15:04:05 2006 MST
const refFormat = "v200601021504"

func Now() string {
	return time.Now().Format(refFormat)
}

func IsValid(str string) bool {
	return refRegexp.MatchString(str)
}
