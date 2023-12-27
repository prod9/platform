package timeouts

import (
	"encoding"
	"strconv"
	"time"
)

type Timeout time.Duration

var (
	t Timeout                  = 1
	_ encoding.TextMarshaler   = t
	_ encoding.TextUnmarshaler = &t
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

	// backward compat with project.toml files before we supported time.Duration text values
	// previously the nanoseconds are stored directly
	lastCh := txt[len(txt)-1]
	if '0' <= lastCh && lastCh <= '9' {
		if n, err := strconv.ParseInt(txt, 10, 64); err != nil {
			return err
		} else {
			*t = Timeout(time.Duration(n))
			return nil
		}
	}

	if d, err := time.ParseDuration(txt); err != nil {
		return err
	} else {
		*t = Timeout(d)
		return nil
	}
}
