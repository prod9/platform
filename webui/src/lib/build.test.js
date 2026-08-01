import { describe, expect, test } from "vitest";
import { tagOf, shortSHA, lastActivity, ranFor } from "./build.js";

// A Go zero time — what the server sends for a moment that has not happened.
const never = "0001-01-01T00:00:00Z";

describe("tagOf", () => {
	test("takes the last segment of a tag ref", () => {
		expect(tagOf("refs/tags/v1.2.3")).toBe("v1.2.3");
	});

	test("takes the last segment of a branch ref", () => {
		expect(tagOf("refs/heads/topic")).toBe("topic");
	});

	test("returns a ref with no separator unchanged", () => {
		expect(tagOf("main")).toBe("main");
	});

	test("returns empty for a ref ending in a separator", () => {
		expect(tagOf("refs/tags/")).toBe("");
	});
});

describe("shortSHA", () => {
	test("keeps the first seven characters", () => {
		expect(shortSHA("4f2a91c8de3b77aa")).toBe("4f2a91c");
	});

	test("leaves a sha shorter than seven alone", () => {
		expect(shortSHA("abc")).toBe("abc");
	});
});

describe("lastActivity", () => {
	test("prefers the finish over the start and the creation", () => {
		const build = {
			created_at: "2026-08-01T10:00:00Z",
			started_at: "2026-08-01T10:01:00Z",
			finished_at: "2026-08-01T10:02:00Z",
		};

		expect(lastActivity(build)).toBe(new Date(build.finished_at).toLocaleString());
	});

	test("falls back to the start when nothing has finished", () => {
		const build = {
			created_at: "2026-08-01T10:00:00Z",
			started_at: "2026-08-01T10:01:00Z",
			finished_at: never,
		};

		expect(lastActivity(build)).toBe(new Date(build.started_at).toLocaleString());
	});

	test("falls back to creation for a build nothing has reported on", () => {
		const build = {
			created_at: "2026-08-01T10:00:00Z",
			started_at: never,
			finished_at: never,
		};

		expect(lastActivity(build)).toBe(new Date(build.created_at).toLocaleString());
	});

	test("is blank when every moment is absent", () => {
		expect(lastActivity({ created_at: never, started_at: never, finished_at: never })).toBe(
			"",
		);
	});

	test("treats a missing field as absent rather than as a date", () => {
		expect(lastActivity({})).toBe("");
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
