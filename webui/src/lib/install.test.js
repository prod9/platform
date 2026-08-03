import { describe, expect, test } from "vitest";
import { nextStep, credentialsPayload } from "./install.js";

describe("nextStep", () => {
	test("picks the first entry that is not done", () => {
		const entries = [
			{ name: "db-reachable", status: "done" },
			{ name: "migrations", status: "pending" },
			{ name: "app-credentials", status: "pending" },
		];

		expect(nextStep(entries)).toEqual({ name: "migrations", status: "pending" });
	});

	test("treats an error entry as the step", () => {
		const entries = [
			{ name: "db-reachable", status: "error", message: "connection refused" },
			{ name: "migrations", status: "pending" },
		];

		expect(nextStep(entries)).toEqual({
			name: "db-reachable",
			status: "error",
			message: "connection refused",
		});
	});

	test("is null once every entry is done", () => {
		const entries = [
			{ name: "db-reachable", status: "done" },
			{ name: "migrations", status: "done" },
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
