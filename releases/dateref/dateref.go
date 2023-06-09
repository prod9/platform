package dateref

import (
	"regexp"
	"time"
)

var refRegexp = regexp.MustCompile("^v([0-9]{4})([0-9]{2})([0-9]{2})([0-9]{2})([0-9]{2})$")

// Jan 2 15:04:05 2006 MST
const refFormat = "v200601021504"

func Now() string {
	return time.Now().Format(refFormat)
}

func IsValid(str string) bool {
	return refRegexp.MatchString(str)
}
