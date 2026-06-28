package engine

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestHostsResolvesAndSorts(t *testing.T) {
	t.Setenv("KUBERNETES_SERVICE_HOST", "10.96.0.1")

	var asked string
	d := &discovery{lookup: func(_ context.Context, host string) ([]string, error) {
		asked = host
		return []string{"10.0.0.2", "10.0.0.1"}, nil
	}}

	got := d.Hosts(context.Background())
	want := []string{"tcp://10.0.0.1:1234", "tcp://10.0.0.2:1234"}

	if asked != engineDNS() {
		t.Errorf("looked up %q, want %q", asked, engineDNS())
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Hosts = %v, want %v", got, want)
	}
}

func TestHostsLocalOutOfCluster(t *testing.T) {
	t.Setenv("KUBERNETES_SERVICE_HOST", "")

	called := false
	d := &discovery{lookup: func(_ context.Context, _ string) ([]string, error) {
		called = true
		return nil, nil
	}}

	if got := d.Hosts(context.Background()); !reflect.DeepEqual(got, []string{localHost}) {
		t.Errorf("Hosts out of cluster = %v, want [%q]", got, localHost)
	}
	if called {
		t.Error("resolved DNS out of cluster, want no lookup")
	}
}

func TestHostsLocalOnError(t *testing.T) {
	t.Setenv("KUBERNETES_SERVICE_HOST", "10.96.0.1")
	d := &discovery{lookup: func(_ context.Context, _ string) ([]string, error) {
		return nil, errors.New("nxdomain")
	}}

	if got := d.Hosts(context.Background()); !reflect.DeepEqual(got, []string{localHost}) {
		t.Errorf("Hosts on error = %v, want [%q]", got, localHost)
	}
}

func TestHostsLocalOnEmpty(t *testing.T) {
	t.Setenv("KUBERNETES_SERVICE_HOST", "10.96.0.1")
	d := &discovery{lookup: func(_ context.Context, _ string) ([]string, error) {
		return nil, nil
	}}

	if got := d.Hosts(context.Background()); !reflect.DeepEqual(got, []string{localHost}) {
		t.Errorf("Hosts on empty = %v, want [%q]", got, localHost)
	}
}

func TestInCluster(t *testing.T) {
	t.Setenv("KUBERNETES_SERVICE_HOST", "10.96.0.1")
	if !inCluster() {
		t.Error("inCluster = false with KUBERNETES_SERVICE_HOST set")
	}

	t.Setenv("KUBERNETES_SERVICE_HOST", "")
	if inCluster() {
		t.Error("inCluster = true with KUBERNETES_SERVICE_HOST empty")
	}
}
