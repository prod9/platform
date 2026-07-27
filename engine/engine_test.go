package engine

import (
	"context"
	"errors"
	"testing"

	fxconfig "fx.prodigy9.co/config"
	r "github.com/stretchr/testify/require"
	"platform.prodigy9.co/conf"
)

func TestHostsResolvesAndSorts(t *testing.T) {
	t.Setenv("DAGGER_ENGINE", "dagger-engine.platform.svc.cluster.local")
	t.Setenv("DAGGER_ENGINE_PORT", "1234")

	asked := ""
	stubLookup(t, func(_ context.Context, host string) ([]string, error) {
		asked = host
		return []string{"10.0.0.2", "10.0.0.1"}, nil
	})

	got, err := Hosts(rosterCtx())
	r.NoError(t, err)
	r.Equal(t, "dagger-engine.platform.svc.cluster.local", asked)
	r.Equal(t, []string{"tcp://10.0.0.1:1234", "tcp://10.0.0.2:1234"}, got)
}

func TestHostsEmptyWhenUnconfigured(t *testing.T) {
	t.Setenv("DAGGER_ENGINE", "")

	called := false
	stubLookup(t, func(context.Context, string) ([]string, error) {
		called = true
		return nil, nil
	})

	got, err := Hosts(rosterCtx())
	r.NoError(t, err)
	r.Empty(t, got)
	r.False(t, called, "resolved DNS while unconfigured")
}

func TestHostsErrorsOnLookupFailure(t *testing.T) {
	t.Setenv("DAGGER_ENGINE", "x")

	stubLookup(t, func(context.Context, string) ([]string, error) {
		return nil, errors.New("nxdomain")
	})

	_, err := Hosts(rosterCtx())
	r.Error(t, err)
}

// TestBuildArch pins the arch rule: the question is not where you stand but whether the
// image outlives the box that built it. A CI build's output is pushed, so it takes the
// publish arch even though the verb is a plain build.
func TestBuildArch(t *testing.T) {
	cfg := &conf.Model{LocalArch: "auto", PublishArch: "amd64"}
	sess := NewSession(rosterCtx())

	t.Setenv("CI", "")
	r.Equal(t, "auto", sess.buildArch(cfg))

	t.Setenv("CI", "true")
	r.Equal(t, "amd64", sess.buildArch(cfg))
}

// TestPickIsLocalWithNoHosts pins the fallback: an empty roster is not an error but the
// operator's explicit choice of a local auto-provisioned engine, which dial spells "".
func TestPickIsLocalWithNoHosts(t *testing.T) {
	r.Equal(t, "", pick(nil))
}

// TestPickReachesEveryHost is what replaces the round-robin cursor's ordering test: the
// guarantee is coverage of the fleet, not a sequence.
func TestPickReachesEveryHost(t *testing.T) {
	hosts := []string{"tcp://a:1234", "tcp://b:1234", "tcp://c:1234"}

	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		seen[pick(hosts)] = true
	}

	r.Len(t, seen, len(hosts))
}

// rosterCtx seeds a context with a freshly-read config, which is where the roster takes
// DAGGER_ENGINE from — the same shape a command hands to NewSession.
func rosterCtx() context.Context {
	return fxconfig.NewContext(context.Background(), fxconfig.Configure())
}

// stubLookup swaps the resolver seam for one test, so no roster test touches DNS.
func stubLookup(t *testing.T, fn func(ctx context.Context, host string) ([]string, error)) {
	t.Helper()

	previous := lookupHost
	lookupHost = fn
	t.Cleanup(func() { lookupHost = previous })
}
