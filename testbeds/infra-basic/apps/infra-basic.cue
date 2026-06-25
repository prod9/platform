package apps

import "prodigy9.co/defs/packs"

// Each top-level field is one app (component); its keys are output filenames.
// A #WebApp instance is list-valued (its objects), so "objects.yaml" holds a
// document list — the renderer emits each element as one YAML document.
"infra-basic": {
	"objects.yaml": packs.#WebApp & {
		#name: "infra-basic"
		#ns:   "infra-basic"

		// Committed image literal — desired state lives in git, never injected at
		// render time; a release / gated-deploy commit updates this ref.
		#image:        "ghcr.io/prod9/infra-basic:v1.0.0"
		#port:         8080
		#host:         "infra-basic.example.com"
		#gateway_name: "infra-basic-gateway"
		#env: {}

		// The open-list embed makes this input list-kind so unification with
		// the pack succeeds.
		[...]
	}
}
