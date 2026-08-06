import { describe, expect, test } from "vitest";
import {
	nextStep,
	appPayload,
	credentialsPayload,
	registryPayload,
	orgPayload,
	orgSlug,
	stepValues,
	generateWebhookSecret,
	orgSettingsURL,
	appSettingsURL,
} from "./install.js";

describe("nextStep", () => {
	test("picks the first entry that is not fully ready", () => {
		const entries = [
			{ name: "db-reachable", state: "fully_ready" },
			{ name: "migrations", state: "partially_ready" },
			{ name: "app-credentials", state: "not_started" },
		];

		expect(nextStep(entries)).toEqual({ name: "migrations", state: "partially_ready" });
	});

	test("treats an intervention entry as the step", () => {
		const entries = [
			{ name: "db-reachable", state: "intervention_required", message: "connection refused" },
			{ name: "migrations", state: "not_started" },
		];

		expect(nextStep(entries)).toEqual({
			name: "db-reachable",
			state: "intervention_required",
			message: "connection refused",
		});
	});

	test("treats an unknown (empty) state as the step", () => {
		const entries = [
			{ name: "db-reachable", state: "fully_ready" },
			{ name: "migrations", state: "", message: "check failed" },
		];

		expect(nextStep(entries)).toEqual({
			name: "migrations",
			state: "",
			message: "check failed",
		});
	});

	test("is null once every entry is fully ready", () => {
		const entries = [
			{ name: "db-reachable", state: "fully_ready" },
			{ name: "migrations", state: "fully_ready" },
		];

		expect(nextStep(entries)).toBe(null);
	});

	test("is null for no entries", () => {
		expect(nextStep([])).toBe(null);
	});
});

describe("appPayload", () => {
	test("trims every field and numbers the app id", () => {
		const payload = appPayload({
			app_id: " 12345 ",
			app_slug: " prodigy9-platform ",
			client_id: " Iv1.abc ",
			webhook_secret: " hooksec ",
		});

		expect(payload).toEqual({
			app_id: 12345,
			app_slug: "prodigy9-platform",
			client_id: "Iv1.abc",
			webhook_secret: "hooksec",
		});
	});

	test("a non-numeric app id becomes zero for the server to refuse", () => {
		expect(appPayload({ app_id: "not-a-number" }).app_id).toBe(0);
	});

	test("missing fields land as empty strings for the server to refuse", () => {
		const payload = appPayload({ app_id: "7" });

		expect(payload).toEqual({
			app_id: 7,
			app_slug: "",
			client_id: "",
			webhook_secret: "",
		});
	});
});

describe("appSettingsURL", () => {
	const entries = [
		{ name: "org", state: "fully_ready", values: { org: "prod9" } },
		{
			name: "app-created",
			state: "fully_ready",
			values: { app_id: "42", app_slug: "prodigy9-platform", client_id: "Iv1.abc" },
		},
	];

	test("builds the App's direct settings path from the saved org and slug", () => {
		expect(appSettingsURL(entries)).toBe(
			"https://github.com/organizations/prod9/settings/apps/prodigy9-platform",
		);
		expect(appSettingsURL(entries, "/installations")).toBe(
			"https://github.com/organizations/prod9/settings/apps/prodigy9-platform/installations",
		);
	});

	test("is null until both the org and the App slug are saved", () => {
		expect(appSettingsURL([])).toBe(null);
		expect(appSettingsURL([entries[0]])).toBe(null);
		expect(appSettingsURL([entries[1]])).toBe(null);
	});
});

describe("credentialsPayload", () => {
	test("trims both fields", () => {
		const payload = credentialsPayload({
			private_key: "-----BEGIN RSA PRIVATE KEY-----\nabc\n-----END RSA PRIVATE KEY-----\n",
			client_secret: " shhh ",
		});

		expect(payload).toEqual({
			private_key: "-----BEGIN RSA PRIVATE KEY-----\nabc\n-----END RSA PRIVATE KEY-----",
			client_secret: "shhh",
		});
	});

	test("missing fields land as empty strings for the server to refuse", () => {
		expect(credentialsPayload({})).toEqual({ private_key: "", client_secret: "" });
	});
});

describe("generateWebhookSecret", () => {
	test("mints 64 hex characters", () => {
		expect(generateWebhookSecret()).toMatch(/^[0-9a-f]{64}$/);
	});

	test("mints a different secret every call", () => {
		expect(generateWebhookSecret()).not.toBe(generateWebhookSecret());
	});
});

describe("orgSettingsURL", () => {
	test("builds the org's developer-settings path", () => {
		expect(orgSettingsURL("prod9", "apps/new")).toBe(
			"https://github.com/organizations/prod9/settings/apps/new",
		);
		expect(orgSettingsURL(" prod9 ", "apps")).toBe(
			"https://github.com/organizations/prod9/settings/apps",
		);
	});

	test("is null without a slug", () => {
		expect(orgSettingsURL("", "apps/new")).toBe(null);
		expect(orgSettingsURL("   ", "apps")).toBe(null);
		expect(orgSettingsURL(undefined, "apps")).toBe(null);
	});
});

describe("orgPayload", () => {
	test("trims the slug", () => {
		expect(orgPayload({ org: "  prod9  " })).toEqual({ org: "prod9" });
	});

	test("leaves emptiness for the server to refuse", () => {
		expect(orgPayload({})).toEqual({ org: "" });
	});
});

describe("stepValues", () => {
	const entries = [
		{ name: "org", state: "fully_ready", values: { org: "prod9" } },
		{ name: "app-created", state: "not_started" },
	];

	test("returns the named entry's saved values for pre-fill", () => {
		expect(stepValues(entries, "org")).toEqual({ org: "prod9" });
	});

	test("an entry without values pre-fills nothing", () => {
		expect(stepValues(entries, "app-created")).toEqual({});
	});

	test("a missing entry pre-fills nothing", () => {
		expect(stepValues([], "org")).toEqual({});
	});
});

describe("orgSlug", () => {
	test("reads the slug the org step surfaced — the server-side source every link builds from", () => {
		const entries = [{ name: "org", state: "fully_ready", values: { org: "prod9" } }];

		expect(orgSlug(entries)).toBe("prod9");
	});

	test("is empty before the org step is saved", () => {
		expect(orgSlug([{ name: "org", state: "not_started" }])).toBe("");
		expect(orgSlug([])).toBe("");
	});
});

describe("registryPayload", () => {
	test("trims the token", () => {
		expect(registryPayload({ token: "  ghp_abc  " })).toEqual({ token: "ghp_abc" });
	});

	test("leaves emptiness for the server to refuse", () => {
		expect(registryPayload({ token: "   " })).toEqual({ token: "" });
	});
});
