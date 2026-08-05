import { describe, expect, test } from "vitest";
import {
	nextStep,
	appPayload,
	credentialsPayload,
	registryPayload,
	generateWebhookSecret,
	orgSettingsURL,
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
			client_id: " Iv1.abc ",
			webhook_secret: " hooksec ",
		});

		expect(payload).toEqual({
			app_id: 12345,
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
			client_id: "",
			webhook_secret: "",
		});
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

describe("registryPayload", () => {
	test("trims the token", () => {
		expect(registryPayload({ token: "  ghp_abc  " })).toEqual({ token: "ghp_abc" });
	});

	test("leaves emptiness for the server to refuse", () => {
		expect(registryPayload({ token: "   " })).toEqual({ token: "" });
	});
});
