package apps

import "prodigy9.co/defs/packs"

// Image is injected by the renderer via `cue export --inject image=...`,
// surfaced on the CLI as `platform ops render --image`. Named distinctly from
// the pack's `#image` input so the reference below resolves to package scope,
// not to a self-cycle.
#injected_image: string @tag(image)

// Each top-level field is one app (component); its keys are output filenames.
// A #WebApp instance is list-valued (its objects), so "objects.yaml" holds a
// document list — the renderer emits each element as one YAML document.
"infra-basic": {
	"objects.yaml": packs.#WebApp & {
		#name:         "infra-basic"
		#ns:           "infra-basic"
		#image:        #injected_image
		#port:         8080
		#host:         "infra-basic.example.com"
		#gateway_name: "infra-basic-gateway"
		#env: {}

		// The open-list embed makes this input list-kind so unification with
		// the pack succeeds.
		[...]
	}
}
