import { describe, expect, test } from "vitest";
import {
	errorText,
	installSignal,
	systemSettings,
	systemMigrations,
	runSystemMigrations,
	classifyMigrationPlan,
	Answered,
	Refused,
	Offline,
	Installing,
	Installed,
	Unknown,
} from "./server.js";

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
