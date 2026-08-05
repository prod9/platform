// The platform server's JSON surface, one function per operation
// (docs/spec/platform-server.md §Operations). Wire shapes are hand-written on both sides
// — there is deliberately no generated contract layer — so this file is where drift
// against the handlers shows up.

// Every request lands in exactly one of three outcomes, and callers branch on the name. A
// null return would collapse the two that matter most: a server that answered "no" and a
// server that never answered at all are different facts about the world, and the install
// gate reads one of them as its signal.
export const Answered = "answered";
export const Refused = "refused";
export const Offline = "offline";

async function send(path, options) {
	try {
		return await fetch(path, options);
	} catch {
		return null;
	}
}

// A refusal's body is the handler's plain-text reason; a successful body is JSON. Reading
// the wrong one throws, so the outcome decides which reader runs.
async function call(path, options) {
	const resp = await send(path, options);
	if (resp === null) {
		return { outcome: Offline, body: "" };
	}

	if (!resp.ok) {
		return { outcome: Refused, body: await resp.text(), status: resp.status };
	}
	return { outcome: Answered, body: await resp.json() };
}

// errorText renders a non-Answered result for display. A refusal carries the handler's
// reason; Offline has no body at all, so the message is ours — every mutation handler
// funnels through here instead of inventing its own strings.
export function errorText(result) {
	if (result.outcome === Offline) {
		return "No answer from the platform server.";
	}
	if (result.body === "") {
		return `The server refused without a reason (status ${result.status}).`;
	}
	return result.body;
}

// The install gate's three-way read of installState. The installer fragment is unmounted
// once the server is installed, so specifically a 404 is the installed signal
// (docs/spec/installation.md §The SvelteKit SPA drives the installer-vs-app view) — any
// other refusal is a server in trouble, and Unknown keeps the gate from routing on it.
export const Installing = "installing";
export const Installed = "installed";
export const Unknown = "unknown";

export function installSignal(result) {
	if (result.outcome === Answered) {
		return Installing;
	}
	if (result.outcome === Refused && result.status === 404) {
		return Installed;
	}
	return Unknown;
}

// installState reads the ordered checklist; installSignal above interprets the result.
export function installState() {
	return call("/api/install");
}

export function runMigrations() {
	return call("/api/install/migrations", { method: "POST" });
}

// saveApp is the wizard's create-the-App step: the trio GitHub's creation form
// yields, entered on the install page. The response is a fresh install-state read.
export function saveApp(payload) {
	return post("/api/install/app", payload);
}

// saveCredentials is the wizard's generated-keys step: the pair GitHub generates on
// the created App's settings page. The response is a fresh install-state read.
export function saveCredentials(payload) {
	return post("/api/install/credentials", payload);
}

// saveRegistryToken is the wizard's registry step: the ghcr push PAT the operator
// creates by hand. The response is a fresh install-state read.
export function saveRegistryToken(payload) {
	return post("/api/install/registry", payload);
}

// claimInstall is the org-owner claim: the App's Setup URL lands the browser on the
// install page with an installation_id, and this posts it (docs/spec/installation.md).
export function claimInstall(installationID) {
	return post("/api/install/claim", { installation_id: installationID });
}

export function currentUser() {
	return call("/api/users/me");
}

export function listBuilds() {
	return call("/api/builds");
}

export function getBuild(id) {
	return call(`/api/builds/${id}`);
}

export function listSteps(id) {
	return call(`/api/builds/${id}/steps`);
}

export function listRepos() {
	return call("/api/repos");
}

// createBuild records a manual trigger — and a retry is just this again with the same
// repo and ref (docs/spec/platform-server.md §Build lifecycle).
export function createBuild(owner, repo, ref) {
	return post("/api/builds", { owner, repo, ref });
}

function post(path, body) {
	return call(path, {
		method: "POST",
		headers: { "Content-Type": "application/json" },
		body: JSON.stringify(body),
	});
}

export function endSession() {
	return call("/api/session", { method: "DELETE" });
}
