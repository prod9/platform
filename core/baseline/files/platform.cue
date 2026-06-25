package apps

import (
	"prodigy9.co/defs"
	"prodigy9.co/defs/parts"
)

// platform — prod9's own deploy under the platformv2 model, as one component: the vanity /
// go-get HTTP server, the in-cluster Dagger build engine it dispatches builds to, and the
// NetworkPolicies that (a) lock the engine's TCP port to the server and (b) fence the engine's
// egress. Wholesale-replaces the legacy Keel-managed Deployment and the standalone
// dagger-engine.cue. Engine topology + rationale:
// docs/decisions/2026-06-21-dagger-engine-statefulset-tcp.md.
"platform": {
	let _ns = "platform"

	let _engine_port = 1234 // BuildKit-conventional TCP port; shared verbatim with the dispatcher
	let _server_port = 8000 // vanity HTTP server

	// --- dagger build engine: a StatefulSet of independent BuildKit daemons over TCP. The
	// server round-robins build sessions across the pods, discovered via the headless Service.
	let engine = defs.#StatefulSet & {
		#name: "dagger-engine"
		#ns:   _ns

		#image:    "registry.dagger.io/engine:v0.20.8" // pinned to platform's dagger SDK
		#replicas: 2

		#privileged: true // full host caps (subsumes CAP_SYS_ADMIN), required by the engine
		#port:       _engine_port
		#args: ["--addr", "tcp://0.0.0.0:\(_engine_port)"]

		#service_name: "dagger-engine" // governing headless Service (engineSvc, below)

		parts.#PodSpread // even pod spread across nodes

		// per-ordinal build cache — each engine keeps its own warm cache (round-robin
		// fragments cache across ordinals; the accepted dumb-RR tradeoff per the ADR).
		parts.#PodMounts & {
			#claim_templates: cache: {#storage: "50Gi", #path: "/var/lib/dagger"}
		}
	}

	let engineSvc = defs.#Service & {
		#name: "dagger-engine"
		#ns:   _ns
		#port: _engine_port

		#headless: true // publish pod A-records for the dispatcher's DNS discovery
		#selector: engine.#out.selector
	}

	// access-grant: lock engine :1234 to in-ns pods bearing prodigy9.co/use-dagger-engine=true.
	// #out.pod_labels carries that grant label; the server claims it via #pod_labels below.
	let engineLock = defs.#NetworkPolicy & {
		#target: {
			#name: "dagger-engine"
			#ns:   _ns
			#ports: [_engine_port]
			#out: selector: engine.#out.selector
		}
	}

	// engine egress fence — the engine runs arbitrary privileged build code, so deny it the
	// internal network (RFC1918) and cloud metadata (link-local/IMDS) while keeping public
	// internet (image/dependency pulls) and cluster DNS. Raw NP: the access-grant def is
	// ingress-only.
	let engineEgress = {
		apiVersion: "networking.k8s.io/v1"
		kind:       "NetworkPolicy"
		metadata: {
			name:      "dagger-engine-egress"
			namespace: _ns
		}
		spec: {
			podSelector: matchLabels: engine.#out.selector
			policyTypes: ["Egress"]
			egress: [
				{
					to: [{
						namespaceSelector: {}
						podSelector: matchLabels: "k8s-app": "kube-dns"
					}]
					ports: [
						{protocol: "UDP", port: 53},
						{protocol: "TCP", port: 53},
					]
				},
				{
					to: [{ipBlock: {
						cidr: "0.0.0.0/0"
						except: ["10.0.0.0/8", "169.254.0.0/16"]
					}}]
				},
			]
		}
	}

	// --- platform vanity / go-get server (the build dispatcher) ---
	let server = defs.#Deployment & {
		#name: "platform"
		#ns:   _ns

		// committed-literal image ref — bumped by a git commit, never a CLI flag. `platform
		// publish` cuts the immutable tag; pin it here.
		#image:    "ghcr.io/prod9/platform:latest"
		#replicas: 2

		#port:    _server_port
		#command: ["./platform", "vanity"]

		#env_from:    "platform-secret"
		#pull_secret: "ghcr.io-pull-secret"

		#pod_labels: engineLock.#out.pod_labels // grant: dispatch builds to the engine
	}

	let serverSvc = defs.#Service & {
		#name: "platform-service"
		#ns:   _ns
		#port: _server_port

		#selector: server.#out.selector
	}

	"namespace.yaml":     [defs.#Namespace & {#name: _ns}]
	"statefulset.yaml":   [engine]
	"deployment.yaml":    [server]
	"service.yaml":       [engineSvc, serverSvc]
	"networkpolicy.yaml": [engineLock, engineEgress]
}
