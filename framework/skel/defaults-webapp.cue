package defaults

import (
	"prodigy9.co/defs"
	"prodigy9.co/defs/packs"
	"prodigy9.co/defs/parts"
)

// The operator gateway's cluster-wide coordinates — declared once; apps/gateway.cue
// instantiates the Gateway from these and every wrapper below attaches to them.
#gateway: {name: "nginx", ns: "gateway"}

// #WebApp is the repo-defaulted app pack: packs.#WebApp pre-wired to the operator
// gateway with cert-manager issuance on its ListenerSet. An app declares only its own
// facts (#name, #ns, #image, #port, #host).
#WebApp: packs.#WebApp & parts.#UseCertManager & {
	#gateway_name: #gateway.name
	#gateway_ns:   #gateway.ns

	[...]
}

// #ListenerSet is the repo-defaulted ListenerSet for hand-rolled routes (e.g. flux-sync):
// attached to the operator gateway, certs issued via cert-manager.
#ListenerSet: defs.#ListenerSet & parts.#UseCertManager & {
	#gateway_name: #gateway.name
	#gateway_ns:   #gateway.ns
}
