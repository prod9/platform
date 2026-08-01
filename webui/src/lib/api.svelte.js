// The server's JSON surface, one function per operation (docs/spec/platform-server.md
// §Operations). Wire shapes are hand-written on both sides — there is deliberately no
// generated contract layer — so this file is where drift against the handlers shows up.

// An unreachable server is a condition to render, never an exception to throw: a failed
// fetch here would otherwise take the whole shell down with it and leave a blank page.
export const unreachable = $state({ hit: false });

async function json(path, options) {
	let resp;
	try {
		resp = await fetch(path, options);
	} catch {
		unreachable.hit = true;
		return null;
	}

	unreachable.hit = false;
	if (!resp.ok) {
		return null;
	}
	return await resp.json();
}

// installState returns the ordered checklist while the server is incomplete. The
// installer fragment is unmounted once installed, so a 404 here *is* the installed
// signal and resolves to null.
export async function installState() {
	return await json("/api/install");
}

export async function runMigrations() {
	const resp = await fetch("/api/install/migrations", { method: "POST" });
	if (resp.ok) {
		return { entries: await resp.json(), error: "" };
	}
	return { entries: null, error: await resp.text() };
}

export async function currentUser() {
	return await json("/api/users/me");
}

export async function listBuilds() {
	return (await json("/api/builds")) ?? [];
}

export async function logout() {
	await fetch("/api/session", { method: "DELETE" });
}

// tagOf is the last segment of a ref: a build's ref is refs/tags/vX.Y.Z and the image is
// published under vX.Y.Z, so the tag is what a reader recognizes.
export function tagOf(ref) {
	const at = ref.lastIndexOf("/");
	return at === -1 ? ref : ref.slice(at + 1);
}

export function shortSHA(sha) {
	return sha.slice(0, 7);
}

// A Go zero time marshals as year 1, so a timestamp before 2000 means "not yet", not a
// date. Treat it as absent rather than rendering it.
function stamped(value) {
	if (!value) {
		return null;
	}
	const at = new Date(value);
	return at.getFullYear() < 2000 ? null : at;
}

export function moment(value) {
	const at = stamped(value);
	return at === null ? "" : at.toLocaleString();
}

// A build's own clock: when it finished if it has, else when it started, else when it was
// asked for.
export function when(build) {
	const at =
		stamped(build.finished_at) ?? stamped(build.started_at) ?? stamped(build.created_at);
	return at === null ? "" : at.toLocaleString();
}

// elapsed is how long an attempt ran, blank until both ends exist.
export function elapsed(build) {
	const from = stamped(build.started_at);
	const to = stamped(build.finished_at);
	if (from === null || to === null) {
		return "";
	}

	const seconds = Math.round((to - from) / 1000);
	return seconds < 60 ? `${seconds}s` : `${Math.floor(seconds / 60)}m ${seconds % 60}s`;
}
