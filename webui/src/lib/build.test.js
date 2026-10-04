import { describe, expect, test } from "vitest";
import { groupBuildSteps, lastActivity, ranFor, selectBuildStep, stepStatus } from "./build.js";

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

// Recorded build detail must distinguish incomplete observations from successful
// steps, and step selection must survive refresh without crossing module boundaries.
// See docs/spec/webui.md §Recorded build-detail slice.
describe("recorded build steps", () => {
	test("a zero finish time remains running even when an error has been recorded", () => {
		const open = {
			started_at: "2026-10-04T10:00:00Z",
			finished_at: never,
			error: "reported before completion",
		};

		expect(stepStatus(open, "running")).toBe("running");
	});

	test("closed observations determine success or failure", () => {
		const closed = { finished_at: "2026-10-04T10:01:00Z", error: "" };
		expect(stepStatus(closed, "running")).toBe("succeeded");
		expect(stepStatus({ ...closed, error: "tests failed" }, "running")).toBe("failed");
	});

	test("a terminal module cannot make an unclosed step look successful or still running", () => {
		for (const status of ["succeeded", "failed"]) {
			expect(stepStatus({ finished_at: never, error: "" }, status)).toBe("incomplete");
		}
	});

	test("identical step names stay with their module, including a queued module without steps", () => {
		const build = {
			modules: [
				{ build_module_id: 21, name: "api", status: "running" },
				{ build_module_id: 22, name: "web", status: "succeeded" },
				{ build_module_id: 23, name: "worker", status: "queued" },
			],
		};
		const steps = [
			{
				build_module_id: 22,
				step: "test",
				finished_at: "2026-10-04T10:01:00Z",
				error: "",
				stdout: "web output",
			},
			{ build_module_id: 21, step: "deps", finished_at: never, error: "", stdout: "" },
			{ build_module_id: 21, step: "test", finished_at: never, error: "", stdout: "api output" },
		];
		const groups = groupBuildSteps(build, steps);

		expect(groups.map(({ name }) => name)).toEqual(["api", "web", "worker"]);
		expect(groups[0].steps.map(({ step }) => step)).toEqual(["deps", "test"]);
		expect(groups[0].steps[1].stdout).toBe("api output");
		expect(groups[1].steps[0].stdout).toBe("web output");
		expect(groups[1].steps[0].status).toBe("succeeded");
		expect(groups[2].steps).toEqual([]);
		expect(selectBuildStep(groups, null)).toBe(groups[1].steps[0]);
	});

	test("refresh keeps the selected observation as new and repeated steps arrive", () => {
		const selected = { build_module_id: 22, step: "test", started_at: "2026-10-04T10:00:00Z" };
		const refreshed = { ...selected, stdout: "latest output" };
		const groups = [
			{ steps: [{ ...selected, build_module_id: 21, stdout: "other module", recordedOrder: 1 }] },
			{
				steps: [
					{ ...selected, started_at: "2026-10-04T10:02:00Z", stdout: "later observation", recordedOrder: 2 },
					{ ...refreshed, recordedOrder: 0 },
				],
			},
		];

		expect(selectBuildStep(groups, selected)).toBe(groups[1].steps[1]);
		expect(selectBuildStep(groups, null)).toBe(groups[1].steps[1]);
		expect(selectBuildStep(groups, { ...selected, step: "removed" })).toBe(groups[1].steps[1]);
		expect(selectBuildStep([{ steps: [] }], selected)).toBeNull();
	});
});
