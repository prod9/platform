import { describe, expect, test } from "vitest";
import { lastActivity, ranFor, byAttempt } from "./build.js";

// A Go zero time — what the server sends for a moment that has not happened.
const never = "0001-01-01T00:00:00Z";

describe("lastActivity", () => {
	test("reports the finish as a concise clock time", () => {
		const build = {
			created_at: "2026-08-01T10:00:00Z",
			started_at: "2026-08-01T10:01:00Z",
			finished_at: "2026-08-01T10:02:00Z",
		};

		expect(lastActivity(build)).toBe("finished 10:02:00");
	});

	test("falls back to the start when nothing has finished", () => {
		const build = {
			created_at: "2026-08-01T10:00:00Z",
			started_at: "2026-08-01T10:01:00Z",
			finished_at: never,
		};

		expect(lastActivity(build)).toBe("started 10:01:00");
	});

	test("falls back to creation for a build nothing has reported on", () => {
		const build = {
			created_at: "2026-08-01T10:00:00Z",
			started_at: never,
			finished_at: never,
		};

		expect(lastActivity(build)).toBe("created 10:00:00");
	});

	test("is blank when every moment is absent", () => {
		expect(lastActivity({ created_at: never, started_at: never, finished_at: never })).toBe("");
	});

	test("treats a missing field as absent rather than as a date", () => {
		expect(lastActivity({})).toBe("");
	});
});

describe("byAttempt", () => {
	test("groups steps under their attempt ordinal, in order", () => {
		const steps = [
			{ attempt: 0, unit: "web", step: "base" },
			{ attempt: 0, unit: "web", step: "deps" },
			{ attempt: 1, unit: "web", step: "base" },
		];

		expect(byAttempt(steps)).toEqual([
			[
				{ attempt: 0, unit: "web", step: "base" },
				{ attempt: 0, unit: "web", step: "deps" },
			],
			[{ attempt: 1, unit: "web", step: "base" }],
		]);
	});

	test("keeps an attempt no step reported for as an empty group", () => {
		const steps = [{ attempt: 1, unit: "web", step: "base" }];

		expect(byAttempt(steps)).toEqual([[], [{ attempt: 1, unit: "web", step: "base" }]]);
	});

	test("is empty for no steps", () => {
		expect(byAttempt([])).toEqual([]);
	});
});

describe("ranFor", () => {
	test("reports seconds under a minute", () => {
		expect(
			ranFor({ started_at: "2026-08-01T10:00:00Z", finished_at: "2026-08-01T10:00:38Z" }),
		).toBe("38s");
	});

	test("reports minutes and seconds past a minute", () => {
		expect(
			ranFor({ started_at: "2026-08-01T10:00:00Z", finished_at: "2026-08-01T10:02:07Z" }),
		).toBe("2m 7s");
	});

	test("is blank while the build is still running", () => {
		expect(ranFor({ started_at: "2026-08-01T10:00:00Z", finished_at: never })).toBe("");
	});

	test("is blank for a build that never started", () => {
		expect(ranFor({ started_at: never, finished_at: never })).toBe("");
	});
});
