import { describe, expect, test } from "vitest";
import {
	filterCandidates,
	latestStatus,
	moduleLine,
	publishPolicyLine,
} from "./repos.js";

describe("filterCandidates", () => {
	const candidates = [
		{ owner: "prod9", repo: "platform", full_name: "prod9/platform" },
		{ owner: "prod9", repo: "infra", full_name: "prod9/infra" },
		{ owner: "naxon", repo: "api", full_name: "naxon/api" },
	];

	test("matches on the full name, case-insensitively", () => {
		expect(filterCandidates(candidates, "PLAT")).toEqual([candidates[0]]);
		expect(filterCandidates(candidates, "prod9/")).toEqual([
			candidates[0],
			candidates[1],
		]);
	});

	test("ignores surrounding whitespace and passes everything on empty", () => {
		expect(filterCandidates(candidates, "  ")).toEqual(candidates);
		expect(filterCandidates(candidates, "")).toEqual(candidates);
	});

	test("no match is an empty list", () => {
		expect(filterCandidates(candidates, "ghost")).toEqual([]);
	});
});

describe("latestStatus", () => {
	test("is the newest build's status", () => {
		expect(latestStatus([{ status: "failed" }, { status: "succeeded" }])).toBe(
			"failed",
		);
	});

	test("is none when the repo has no builds", () => {
		expect(latestStatus([])).toBe("none");
	});
});

describe("moduleLine", () => {
	test("joins name (framework) pairs with middots", () => {
		expect(
			moduleLine([
				{ name: "api", framework: "go/basic" },
				{ name: "web", framework: "pnpm/static" },
			]),
		).toBe("api (go/basic) · web (pnpm/static)");
	});

	test("a module with no framework is its name alone", () => {
		expect(moduleLine([{ name: "api", framework: "" }])).toBe("api");
	});

	test("no modules is an empty string", () => {
		expect(moduleLine([])).toBe("");
	});
});

describe("publishPolicyLine", () => {
	test.each([
		["always", "Every successful build · latest"],
		["tags", "Successful tags · exact tag"],
		["never", "Build only · never publish"],
	])("renders %s without exposing config vocabulary alone", (policy, expected) => {
		expect(publishPolicyLine(policy)).toBe(expected);
	});

	test("rejects an unknown server value", () => {
		expect(() => publishPolicyLine("sometimes")).toThrow("unknown server publish policy");
	});
});
