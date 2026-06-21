package builder

import (
	"fmt"
	"net"
	"os"
	"sort"
)

// In-cluster Dagger engine pool. Platform discovers the engine pods by resolving the
// headless Service's A-records (no k8s API / RBAC) and round-robins build jobs across them.
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

// discoverEngines returns the Dagger runner-host pool when in-cluster, nil otherwise.
// A nil result means "no pool" — the caller falls back to a single local engine.
func discoverEngines() []string {
	if !inCluster() {
		return nil
	}
	return runnerHosts(net.LookupHost)
}

// runnerHosts resolves the engine headless Service into one `tcp://<ip>:<port>` runner host
// per ready pod, sorted for stable round-robin ordering within a run. Returns nil when
// resolution fails or yields nothing — lookup is injected so this stays unit-testable.
func runnerHosts(lookup func(host string) ([]string, error)) []string {
	addrs, err := lookup(engineDNS())
	if err != nil || len(addrs) == 0 {
		return nil
	}

	hosts := make([]string, 0, len(addrs))
	for _, addr := range addrs {
		hosts = append(hosts, fmt.Sprintf("tcp://%s:%d", addr, enginePort))
	}

	sort.Strings(hosts)
	return hosts
}
