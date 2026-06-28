package engine

import (
	"context"
	"errors"
	"testing"

	r "github.com/stretchr/testify/require"
)

func TestHostsResolvesAndSorts(t *testing.T) {
	var asked string
	d := &discovery{
		dns:  "dagger-engine.platform.svc.cluster.local",
		port: 1234,
		lookup: func(_ context.Context, host string) ([]string, error) {
			asked = host
			return []string{"10.0.0.2", "10.0.0.1"}, nil
		},
	}

	got, err := d.Hosts(context.Background())
	r.NoError(t, err)
	r.Equal(t, d.dns, asked)
	r.Equal(t, []string{"tcp://10.0.0.1:1234", "tcp://10.0.0.2:1234"}, got)
}

func TestHostsEmptyWhenUnconfigured(t *testing.T) {
	called := false
	d := &discovery{dns: "", lookup: func(context.Context, string) ([]string, error) {
		called = true
		return nil, nil
	}}

	got, err := d.Hosts(context.Background())
	r.NoError(t, err)
	r.Empty(t, got)
	r.False(t, called, "resolved DNS while unconfigured")
}

func TestHostsErrorsOnLookupFailure(t *testing.T) {
	d := &discovery{dns: "x", port: 1234, lookup: func(context.Context, string) ([]string, error) {
		return nil, errors.New("nxdomain")
	}}

	_, err := d.Hosts(context.Background())
	r.Error(t, err)
}
