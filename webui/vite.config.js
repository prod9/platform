import { sveltekit } from "@sveltejs/kit/vite";

export default {
	plugins: [sveltekit()],
	server: {
		host: "0.0.0.0",
		allowedHosts: [".meerkat-banded.ts.net"],
		// the platform server (fx LISTEN_ADDR default) owns the API in dev.
		proxy: {
			"/api": "http://localhost:3000",
			"/setup": "http://localhost:3000",
		},
	},
};
