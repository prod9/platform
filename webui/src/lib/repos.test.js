import { describe, expect, test } from "vitest";
import {
	filterCandidates,
	latestStatus,
	moduleLabel,
	publishPolicyDetails,
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

describe("moduleLabel", () => {
	test("pairs one module name with its framework", () => {
		expect(moduleLabel({ name: "api", framework: "go/basic" })).toBe(
			"api (go/basic)",
		);
	});

	test("a module with no framework is its name alone", () => {
		expect(moduleLabel({ name: "api", framework: "" })).toBe("api");
	});
});

describe("publishPolicyDetails", () => {
	test.each([
		[
			"always",
			{
				policy: "Always publish",
				cadence: "Every successful build",
				imageTag: "latest",
			},
		],
		[
			"tags",
			{
				policy: "Publish tags",
				cadence: "Successful tag builds",
				imageTag: "The exact Git tag",
			},
		],
		[
			"never",
			{
				policy: "Never publish",
				cadence: "Builds validate without publishing",
				imageTag: "None",
			},
		],
	])("explains %s as policy, cadence, and image tag", (policy, expected) => {
		expect(publishPolicyDetails(policy)).toEqual(expected);
	});

	test("rejects an unknown server value", () => {
		expect(() => publishPolicyDetails("sometimes")).toThrow(
			"unknown server publish policy",
		);
	});
});
