package engine

import (
	"context"
	"testing"

	"dagger.io/dagger"
	r "github.com/stretchr/testify/require"
)

// The pool outlives every caller that dials into it, so a dial must not inherit the
// caller's cancellation. It used to: Build gives each unit a timeout ctx and cancels it
// when the unit ends, which tore down the session the cached client was still holding —
// the next use of that client (export, exec, preview) died with "file already closed".
func TestGetDialsWithoutCallerCancellation(t *testing.T) {
	var dialed context.Context
	c := &clients{
		pool: map[string]*dagger.Client{},
		dial: func(ctx context.Context, _ string) (*dagger.Client, error) {
			dialed = ctx
			return nil, nil
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	_, err := c.Get(ctx, "tcp://10.0.0.1:1234")
	r.NoError(t, err)
	cancel()

	r.NotNil(t, dialed)
	r.NoError(t, dialed.Err(), "dial context died with the caller's")
}

// Values still cross — only cancellation is severed.
func TestGetDialKeepsContextValues(t *testing.T) {
	type key struct{}
	var dialed context.Context
	c := &clients{
		pool: map[string]*dagger.Client{},
		dial: func(ctx context.Context, _ string) (*dagger.Client, error) {
			dialed = ctx
			return nil, nil
		},
	}

	ctx := context.WithValue(context.Background(), key{}, "carried")
	_, err := c.Get(ctx, "tcp://10.0.0.1:1234")
	r.NoError(t, err)
	r.Equal(t, "carried", dialed.Value(key{}))
}
