import { sveltekit } from "@sveltejs/kit/vite";

// Ports come from the prod9 block-10 assignment (8200-8219): dev 8200, preview 8201,
// the platform server 8210. 0.0.0.0 so the dev server is reachable over the tailnet,
// where only 8000-9000 is open.
export default {
	plugins: [sveltekit()],
	server: {
		host: "0.0.0.0",
		port: 8200,
		strictPort: true,
		allowedHosts: [".meerkat-banded.ts.net"],
		// the platform server owns every reserved backend prefix in dev
		// (docs/spec/platform-server.md §Operations).
		proxy: {
			"/api": "http://localhost:8210",
			"/auth": "http://localhost:8210",
			"/hooks": "http://localhost:8210",
			"/health": "http://localhost:8210",
		},
	},
	preview: {
		host: "0.0.0.0",
		port: 8201,
		strictPort: true,
		allowedHosts: [".meerkat-banded.ts.net"],
	},
};
