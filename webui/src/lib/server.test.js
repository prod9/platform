import { describe, expect, test } from "vitest";
import {
	errorText,
	installSignal,
	systemSettings,
	systemMigrations,
	runSystemMigrations,
	registerRepo,
	classifyMigrationPlan,
	Answered,
	Refused,
	Offline,
	Installing,
	Installed,
	Unknown,
	prepareReauthentication,
	restoreFragment,
	signInRetryURL,
	shouldRecoverSession,
	authenticationURL,
} from "./server.js";

function browserAt(href) {
	const url = new URL(href);
	const stored = new Map();
	const assigned = [];
	globalThis.window = {
		location: {
			pathname: url.pathname,
			search: url.search,
			hash: url.hash,
			assign: (destination) => assigned.push(destination),
		},
		sessionStorage: {
			getItem: (key) => stored.get(key) ?? null,
			setItem: (key, value) => stored.set(key, value),
			removeItem: (key) => stored.delete(key),
		},
	};
	return { assigned, stored };
}

describe("session recovery", () => {
	test("authentication binds the return location and pre-claim installation", () => {
		expect(authenticationURL("/install/?installation_id=84", "84")).toBe(
			"/auth/github?return=%2Finstall%2F%3Finstallation_id%3D84&installation_id=84",
		);
	});

	test("a product 401 starts one OAuth recovery at the exact browser location", async () => {
		const browser = browserAt(
			"https://platform.example/repos/new/?step=manifest#instructions",
		);
		globalThis.fetch = async () => ({
			ok: false,
			status: 401,
			text: async () => "expired",
		});

		await Promise.all([systemSettings(), systemMigrations()]);

		expect(browser.assigned).toEqual([
			"/auth/github?return=%2Frepos%2Fnew%2F%3Fstep%3Dmanifest",
		]);
		expect(browser.stored.get("auth.fragment:/repos/new/?step=manifest")).toBe(
			"#instructions",
		);
	});

	test("non-401 refusals stay page-level outcomes", async () => {
		const browser = browserAt("https://platform.example/repos/new/");
		globalThis.fetch = async () => ({
			ok: false,
			status: 403,
			text: async () => "forbidden",
		});

		const result = await systemSettings();

		expect(result).toEqual({ outcome: Refused, body: "forbidden", status: 403 });
		expect(browser.assigned).toEqual([]);
	});

	test("the session failure page cannot start a recovery loop", () => {
		expect(shouldRecoverSession(401, "/session/")).toBe(false);
		expect(shouldRecoverSession(401, "/repos/new/")).toBe(true);
	});

	test("the install probe never starts session recovery", async () => {
		const browser = browserAt("https://platform.example/install/");
		globalThis.fetch = async () => ({
			ok: false,
			status: 401,
			text: async () => "not claimed",
		});

		const result = await import("./server.js").then(({ installState }) => installState());

		expect(result.status).toBe(401);
		expect(browser.assigned).toEqual([]);
	});

	test("pre-claim recovery binds the installation from the setup landing", () => {
		const browser = browserAt(
			"https://platform.example/install/?installation_id=84#claim",
		);

		expect(prepareReauthentication()).toBe(
			"/auth/github?return=%2Finstall%2F%3Finstallation_id%3D84&installation_id=84",
		);
		expect(browser.stored.get("auth.fragment:/install/?installation_id=84")).toBe(
			"#claim",
		);
	});

	test("a successful matching return restores its fragment once", () => {
		const browser = browserAt("https://platform.example/repos/new/?step=manifest");
		browser.stored.set(
			"auth.fragment:/repos/new/?step=manifest",
			"#instructions",
		);

		restoreFragment();
		restoreFragment();

		expect(window.location.hash).toBe("#instructions");
		expect(browser.stored.size).toBe(0);
	});

	test("the failure state offers an explicit retry with its bound inputs", () => {
		expect(
			signInRetryURL(
				"?return=%2Frepos%2Fnew%2F%3Fstep%3Dmanifest&installation_id=84",
			),
		).toBe(
			"/auth/github?return=%2Frepos%2Fnew%2F%3Fstep%3Dmanifest&installation_id=84",
		);
	});

	test("the failure state cannot turn retry into an open redirect", () => {
		expect(signInRetryURL("?return=https%3A%2F%2Fevil.example%2Fsteal")).toBe(
			"/auth/github?return=%2F",
		);
		expect(signInRetryURL("?return=%2F%2Fevil.example%2Fsteal")).toBe(
			"/auth/github?return=%2F",
		);
	});
});

describe("repository onboarding", () => {
	test("confirmation identifies the exact reviewed manifest", async () => {
		let request;
		globalThis.fetch = async (path, options) => {
			request = { path, options };
			return { ok: true, json: async () => ({}) };
		};

		await registerRepo("prodigy9", "platform", "abc123");

		expect(request).toEqual({
			path: "/api/repos",
			options: {
				method: "POST",
				headers: { "Content-Type": "application/json" },
				body: JSON.stringify({
					owner: "prodigy9",
					repo: "platform",
					manifest_sha: "abc123",
				}),
			},
		});
	});
});

describe("system operations", () => {
	test("reads the settings and migration surfaces", async () => {
		const requested = [];
		globalThis.fetch = async (path, options) => {
			requested.push({ path, options });
			return { ok: true, json: async () => [] };
		};

		await systemSettings();
		await systemMigrations();

		expect(requested).toEqual([
			{ path: "/api/system/settings", options: undefined },
			{ path: "/api/system/migrations", options: undefined },
		]);
	});

	test("runs migrations through the system operation", async () => {
		let request;
		globalThis.fetch = async (path, options) => {
			request = { path, options };
			return { ok: true, json: async () => [] };
		};

		await runSystemMigrations();

		expect(request).toEqual({
			path: "/api/system/migrations",
			options: { method: "POST" },
		});
	});
});

describe("classifyMigrationPlan", () => {
	test("an empty plan is current", () => {
		expect(classifyMigrationPlan([])).toBe("current");
	});

	test("migrate-only plans are runnable", () => {
		expect(classifyMigrationPlan([{ action: "migrate", migration: "repos" }])).toBe(
			"runnable",
		);
	});

	test.each(["update sql", "remove"])("%s requires manual recovery", (action) => {
		expect(classifyMigrationPlan([{ action, migration: "repos" }])).toBe(
			"intervention_required",
		);
	});

	test("manual recovery takes precedence over runnable lines", () => {
		expect(
			classifyMigrationPlan([
				{ action: "migrate", migration: "repos" },
				{ action: "update sql", migration: "settings" },
			]),
		).toBe("intervention_required");
	});
});

describe("errorText", () => {
	test("gives Offline a human-readable message", () => {
		expect(errorText({ outcome: Offline, body: "" })).toBe(
			"No answer from the platform server.",
		);
	});

	test("a refusal shows the handler's reason", () => {
		expect(errorText({ outcome: Refused, body: "ref not found", status: 422 })).toBe(
			"ref not found",
		);
	});

	test("a reasonless refusal still says something", () => {
		expect(errorText({ outcome: Refused, body: "", status: 502 })).toBe(
			"The server refused without a reason (status 502).",
		);
	});
});

describe("installSignal", () => {
	test("an answered checklist means the installer is up", () => {
		expect(installSignal({ outcome: Answered, body: [] })).toBe(Installing);
	});

	test("only a real 404 means installed", () => {
		expect(installSignal({ outcome: Refused, body: "", status: 404 })).toBe(Installed);
	});

	test("a server error is not the installed signal", () => {
		expect(installSignal({ outcome: Refused, body: "boom", status: 500 })).toBe(Unknown);
	});

	test("no answer at all is neither side", () => {
		expect(installSignal({ outcome: Offline, body: "" })).toBe(Unknown);
	});
});
