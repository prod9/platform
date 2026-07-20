package timeouts

import (
	"encoding"
	"time"
)

type Timeout time.Duration

var (
	_ encoding.TextMarshaler   = Timeout(0)
	_ encoding.TextUnmarshaler = (*Timeout)(nil)
)

func From(d time.Duration) Timeout {
	return Timeout(d)
}

func (t Timeout) Duration() time.Duration {
	return time.Duration(t)
}

func (t Timeout) MarshalText() ([]byte, error) {
	return []byte(t.Duration().String()), nil
}

func (t *Timeout) UnmarshalText(buf []byte) error {
	txt := string(buf)
	if len(txt) == 0 {
		*t = 0
		return nil
	}

	d, err := time.ParseDuration(txt)
	if err != nil {
		return err
	}
	*t = Timeout(d)
	return nil
}
