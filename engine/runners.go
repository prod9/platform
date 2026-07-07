package engine

import (
	"context"
	"fmt"
	"net"
	"sort"

	fxconfig "fx.prodigy9.co/config"
)

var (
	// DaggerEngineConfig is the headless-Service DNS name of the Dagger engine pool, e.g.
	// dagger-engine.platform.svc.cluster.local. Unset means no remote engines are configured
	// and the engine falls back to a local auto-provisioned one — an explicit operator choice,
	// never inferred from the environment.
	DaggerEngineConfig = fxconfig.Str("DAGGER_ENGINE")
	// DaggerEnginePortConfig is the engine pod port; the default mirrors apps/dagger-engine.cue.
	DaggerEnginePortConfig = fxconfig.IntDef("DAGGER_ENGINE_PORT", 1234)
)

// runners resolves the configured Dagger engine endpoints. It reports only what it finds —
// an empty result when DAGGER_ENGINE is unset or resolves to nothing — and never decides to
// fall back to a local engine; that policy lives in the core. Resolution is via the resolver
// (no k8s API / RBAC), which caches per the record TTL, so a new pod becomes selectable as
// soon as DNS reflects it.
type runners struct {
	dns    string
	port   int
	lookup func(ctx context.Context, host string) ([]string, error)
}

func newRunners(cfg *fxconfig.Source) *runners {
	return &runners{
		dns:    fxconfig.Get(cfg, DaggerEngineConfig),
		port:   fxconfig.Get(cfg, DaggerEnginePortConfig),
		lookup: net.DefaultResolver.LookupHost,
	}
}

// Hosts returns one tcp:// runner-host URL per ready engine pod, sorted for stable
// round-robin. It returns an empty slice when no engine is configured or none have resolved
// yet; a lookup failure is a real error worth surfacing.
func (d *runners) Hosts(ctx context.Context) ([]string, error) {
	if d.dns == "" {
		return nil, nil
	}

	addrs, err := d.lookup(ctx, d.dns)
	if err != nil {
		return nil, fmt.Errorf("resolving dagger engines at %s: %w", d.dns, err)
	}

	hosts := make([]string, 0, len(addrs))
	for _, addr := range addrs {
		hosts = append(hosts, fmt.Sprintf("tcp://%s:%d", addr, d.port))
	}
	sort.Strings(hosts)
	return hosts, nil
}
