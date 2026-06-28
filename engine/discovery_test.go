package engine

import (
	"context"
	"errors"
	"reflect"
	"testing"
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
	if err != nil {
		t.Fatal(err)
	}

	want := []string{"tcp://10.0.0.1:1234", "tcp://10.0.0.2:1234"}
	if asked != d.dns {
		t.Errorf("looked up %q, want %q", asked, d.dns)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Hosts = %v, want %v", got, want)
	}
}

func TestHostsEmptyWhenUnconfigured(t *testing.T) {
	called := false
	d := &discovery{dns: "", lookup: func(context.Context, string) ([]string, error) {
		called = true
		return nil, nil
	}}

	got, err := d.Hosts(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("Hosts unconfigured = %v, want empty", got)
	}
	if called {
		t.Error("resolved DNS while unconfigured, want no lookup")
	}
}

func TestHostsErrorsOnLookupFailure(t *testing.T) {
	d := &discovery{dns: "x", port: 1234, lookup: func(context.Context, string) ([]string, error) {
		return nil, errors.New("nxdomain")
	}}

	if _, err := d.Hosts(context.Background()); err == nil {
		t.Error("Hosts on lookup failure = nil error, want error")
	}
}
