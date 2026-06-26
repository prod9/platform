package apps

import (
	"prodigy9.co/defs"
	"prodigy9.co/defs/parts"
)

platform: {
	let nsp = defs.#Namespace & {#name: "platform"}

	let dagger_version = "v0.20.8"
	let engine_port = 1234
	let server_port = 8000

	let engine = defs.#StatefulSet & {
		parts.#PodSpread
		parts.#PodMounts & {
			#claim_templates: cache: {#storage: "50Gi", #path: "/var/lib/dagger"}
		}

		#name: "dagger-engine"
		#ns:   nsp.#name

		#image:    "registry.dagger.io/engine:\(dagger_version)"
		#replicas: 2

		#privileged: true // BuildKit needs full host caps
		#port:       engine_port
		#args: ["--addr", "tcp://0.0.0.0:\(engine_port)"]
	}

	let engineSvc = defs.#Service & {
		#name: "dagger-engine"
		#ns:   nsp.#name
		#port: engine_port

		#headless: true
		#selector: engine.#out.selector
	}

	let engineNetpol = defs.#NetworkPolicy & {
		#target: {
			#name: "dagger-engine"
			#ns:   nsp.#name
			#ports: [engine_port]
			#out: selector: engine.#out.selector
		}
	}

	let server = defs.#Deployment & {
		#name: "platform"
		#ns:   nsp.#name

		#image:    "ghcr.io/prod9/platform:latest"
		#replicas: 2

		#port:    server_port
		#command: ["./platform", "vanity"]

		#pull_secret: "ghcr.io-pull-secret"
		#pod_labels:  engineNetpol.#out.pod_labels
	}

	let serverSvc = defs.#Service & {
		#name: "platform-service"
		#ns:   nsp.#name
		#port: server_port

		#selector: server.#out.selector
	}

	"namespace.yaml": nsp
	"engine.yaml":    [engine, engineSvc, engineNetpol]
	"platform.yaml":  [server, serverSvc]
}
