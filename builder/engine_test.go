package builder

import (
	"errors"
	"reflect"
	"testing"
)

func TestRunnerHostsResolvesAndSorts(t *testing.T) {
	var asked string
	lookup := func(host string) ([]string, error) {
		asked = host
		return []string{"10.0.0.2", "10.0.0.1"}, nil
	}

	got := runnerHosts(lookup)
	want := []string{"tcp://10.0.0.1:1234", "tcp://10.0.0.2:1234"}

	if asked != engineDNS() {
		t.Errorf("looked up %q, want %q", asked, engineDNS())
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("runnerHosts = %v, want %v", got, want)
	}
}

func TestRunnerHostsNilOnError(t *testing.T) {
	got := runnerHosts(func(string) ([]string, error) {
		return nil, errors.New("nxdomain")
	})
	if got != nil {
		t.Errorf("runnerHosts on error = %v, want nil", got)
	}
}

func TestRunnerHostsNilOnEmpty(t *testing.T) {
	got := runnerHosts(func(string) ([]string, error) {
		return nil, nil
	})
	if got != nil {
		t.Errorf("runnerHosts on empty = %v, want nil", got)
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
