package releases

import (
	"platform.prodigy9.co/releases/timeref"
)

type Timestamp struct{}

var _ Strategy = Timestamp{}

func (d Timestamp) IsValid(name string) bool {
	return timeref.IsValid(name)
}

func (d Timestamp) NextName(prevName string, comp NameComponent) (string, error) {
	return timeref.Now(), nil
}
