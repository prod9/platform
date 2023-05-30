package config

import (
	"context"
	"time"
)

// because, well, time.Duration has no TextUnmarshaler implementation :yuno:
type Timeout time.Duration

func (t Timeout) NewContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, (time.Duration)(t))
}

func (t Timeout) MarshalText() ([]byte, error) {
	str := (time.Duration)(t).String()
	return []byte(str), nil
}

func (t *Timeout) UnmarshalText(text []byte) error {
	if dur, err := time.ParseDuration(string(text)); err != nil {
		return err
	} else {
		*t = Timeout(dur)
		return nil
	}
}
