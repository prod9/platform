package defaults

import "prodigy9.co/defs/packs"

#Basics: packs.#Basics & {
	// #registry defaults to ghcr.io; uncomment to override.
	// #registry: "ghcr.io"

	// Cluster image-pull credentials — hand-edit before delivering private images.
	#registry_username: ""
	#registry_password: ""

	[...]
}
