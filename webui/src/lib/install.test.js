import { describe, expect, test } from "vitest";
import { nextStep, credentialsPayload } from "./install.js";

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

describe("credentialsPayload", () => {
	test("trims every field and numbers the app id", () => {
		const payload = credentialsPayload({
			app_id: " 12345 ",
			private_key: "-----BEGIN RSA PRIVATE KEY-----\nabc\n-----END RSA PRIVATE KEY-----\n",
			webhook_secret: " hooksec ",
			client_id: " Iv1.abc ",
			client_secret: " shhh ",
		});

		expect(payload).toEqual({
			app_id: 12345,
			private_key: "-----BEGIN RSA PRIVATE KEY-----\nabc\n-----END RSA PRIVATE KEY-----",
			webhook_secret: "hooksec",
			client_id: "Iv1.abc",
			client_secret: "shhh",
		});
	});

	test("a non-numeric app id becomes zero for the server to refuse", () => {
		expect(credentialsPayload({ app_id: "not-a-number" }).app_id).toBe(0);
	});

	test("missing fields land as empty strings for the server to refuse", () => {
		const payload = credentialsPayload({ app_id: "7" });

		expect(payload).toEqual({
			app_id: 7,
			private_key: "",
			webhook_secret: "",
			client_id: "",
			client_secret: "",
		});
	});
});
