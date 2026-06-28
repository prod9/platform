package engine

import (
	"context"
	"fmt"
	"net"
	"os"
	"sort"
)

// In-cluster engine discovery. Platform finds the engine pods by resolving the headless
// Service's A-records (no k8s API / RBAC) and round-robins build units across them.
// These constants mirror apps/dagger-engine.cue verbatim — keep them in sync.
// See docs/decisions/2026-06-21-dagger-engine-statefulset-tcp.md.
const (
	engineService   = "dagger-engine"
	engineNamespace = "platform"
	enginePort      = 1234
)

func engineDNS() string {
	return fmt.Sprintf("%s.%s.svc.cluster.local", engineService, engineNamespace)
}

// inCluster reports whether platform is running inside Kubernetes, via the ambient
// KUBERNETES_SERVICE_HOST that the kubelet injects into every pod.
func inCluster() bool {
	return os.Getenv("KUBERNETES_SERVICE_HOST") != ""
}

// localHost is the sentinel endpoint for the local auto-provisioned engine — an empty
// runner host, which dialEngine translates into a plain dagger.Connect.
const localHost = ""

// discovery resolves the current set of Dagger engine endpoints. It is deliberately
// stateless: each call hits the resolver, which already caches per the record TTL (CoreDNS
// in cluster), so a new engine pod becomes selectable as soon as DNS reflects it — no
// restart, no app-level TTL. It knows nothing about connections.
type discovery struct {
	lookup func(ctx context.Context, host string) ([]string, error)
}

func newDiscovery() *discovery {
	return &discovery{lookup: net.DefaultResolver.LookupHost}
}

// Hosts returns one runner-host URL per ready engine pod, sorted for stable round-robin.
// Out of cluster, or when resolution fails or is empty, it returns the local sentinel so
// there is always at least one endpoint to build on.
func (d *discovery) Hosts(ctx context.Context) []string {
	if !inCluster() {
		return []string{localHost}
	}

	addrs, err := d.lookup(ctx, engineDNS())
	if err != nil || len(addrs) == 0 {
		return []string{localHost}
	}

	hosts := make([]string, 0, len(addrs))
	for _, addr := range addrs {
		hosts = append(hosts, fmt.Sprintf("tcp://%s:%d", addr, enginePort))
	}
	sort.Strings(hosts)
	return hosts
}
