package apps

import (
	"prodigy9.co/defs"
	"prodigy9.co/defs/parts"
)

// dagger-engine — the in-cluster Dagger build engine: a StatefulSet of independent BuildKit
// daemons listening over TCP. Platform round-robins build sessions across the pods, which it
// discovers via the headless Service's A-records. Topology + rationale:
// docs/decisions/2026-06-21-dagger-engine-statefulset-tcp.md.
"dagger-engine": {
	let _ns = "platform"
	let _port = 1234 // BuildKit-conventional TCP port; shared verbatim with the dispatcher

	let engine = defs.#StatefulSet & {
		#name:     "dagger-engine"
		#ns:       _ns
		#image:    "registry.dagger.io/engine:v0.20.8" // pinned to platform's dagger SDK
		#replicas: 2

		#privileged: true // full host caps (subsumes CAP_SYS_ADMIN), required by the engine
		#port: _port
		#args: ["--addr", "tcp://0.0.0.0:\(_port)"]

		#service_name: "dagger-engine" // governing headless Service (svc, below)

		parts.#PodSpread // even pod spread across nodes

		// per-ordinal build cache — each engine keeps its own warm cache (round-robin
		// fragments cache across ordinals; the accepted dumb-RR tradeoff per the ADR).
		spec: {
			volumeClaimTemplates: [{
				metadata: name: "cache"
				spec: {
					accessModes: ["ReadWriteOnce"]
					resources: requests: storage: "50Gi"
				}
			}]
			template: spec: containers: [{
				volumeMounts: [{name: "cache", mountPath: "/var/lib/dagger"}]
			}]
		}
	}

	let svc = defs.#Service & {
		#name:     "dagger-engine"
		#ns:       _ns
		#port:     _port
		#headless: true // publish pod A-records for the dispatcher's DNS discovery
		#selector: engine.#out.selector
	}

	"namespace.yaml":   [defs.#Namespace & {#name: _ns}]
	"statefulset.yaml": [engine]
	"service.yaml":     [svc]
}
