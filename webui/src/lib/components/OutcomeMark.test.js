import { render } from "svelte/server";
import { describe, expect, test } from "vitest";
import OutcomeMark from "./OutcomeMark.svelte";

describe("OutcomeMark", () => {
	test("renders an accessible glyph without a visible status label", () => {
		const body = render(OutcomeMark, { props: { status: "succeeded" } }).body;

		expect(body).toContain('aria-label="succeeded"');
		expect(body).toContain("✓");
		expect(body).not.toContain('class="value"');
	});
});
