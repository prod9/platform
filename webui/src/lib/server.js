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

// installState reads the ordered checklist. The installer fragment is unmounted once the
// server is installed, so Refused here is the installed signal — which is exactly why it
// must never be confused with Offline.
export function installState() {
	return call("/api/install");
}

export function runMigrations() {
	return call("/api/install/migrations", { method: "POST" });
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
