package infra

import "prodigy9.co/defs/packs"

// Image is injected by the renderer via `cue export --inject image=...`,
// surfaced on the CLI as `platform render --image`. Named distinctly from the
// pack's `#image` input so the reference below resolves to package scope, not
// to a self-cycle.
#injected_image: string @tag(image)

app: packs.#WebApp & {
	#name:         "infra-basic"
	#ns:           "infra-basic"
	#image:        #injected_image
	#port:         8080
	#host:         "infra-basic.example.com"
	#gateway_name: "infra-basic-gateway"
	#env: {}

	// A #WebApp instance is list-valued (its objects); the open-list embed
	// makes this input list-kind so unification with the pack succeeds.
	[...]
}

// The renderer exports this list and emits each element as one YAML document.
objects: app
