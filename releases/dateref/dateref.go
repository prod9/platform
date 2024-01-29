package dateref

import (
	"errors"
	"regexp"
	"strconv"
	"time"
)

var (
	ErrInvalidDateRef = errors.New("invalid dateref version")

	refRegexp = regexp.MustCompile("^v([0-9]{8})(-[0-9]+)?$")
)

// Jan 2 15:04:05 2006 MST
const dateFormat = "20060102"

type DateRef struct {
	date    time.Time
	counter int
}

func New(date time.Time, counter int) DateRef {
	return DateRef{date, counter}
}

func Now(counter int) DateRef {
	if counter < 0 {
		return DateRef{time.Now(), 0}
	} else {
		return DateRef{time.Now(), counter}
	}
}

func Parse(str string) (DateRef, error) {
	matches := refRegexp.FindAllStringSubmatch(str, -1)
	if len(matches) <= 0 || len(matches[0]) < 2 {
		return DateRef{}, ErrInvalidDateRef
	}

	date, err := time.Parse(dateFormat, matches[0][1])
	if err != nil {
		return DateRef{}, ErrInvalidDateRef
	}

	if len(matches[0]) < 2 || matches[0][2] == "" {
		return DateRef{date, 0}, nil
	}

	counter, err := strconv.Atoi(matches[0][2][1:]) // dash prefix
	if err != nil {
		return DateRef{}, ErrInvalidDateRef
	}
	return DateRef{date, counter}, nil
}

func (d DateRef) IsToday() bool {
	now := time.Now()
	return d.date.Year() == now.Year() &&
		d.date.Month() == now.Month() &&
		d.date.Day() == now.Day()
}

func (d DateRef) NextCounter() DateRef {
	if d.counter < 0 {
		return DateRef{d.date, 1}
	} else {
		return DateRef{d.date, d.counter + 1}
	}
}

func (d DateRef) String() string {
	if d.counter > 0 {
		return "v" + d.date.Format(dateFormat) + "-" + strconv.Itoa(d.counter)
	} else {
		return "v" + d.date.Format(dateFormat)
	}
}

func IsValid(str string) bool {
	return refRegexp.MatchString(str)
}
