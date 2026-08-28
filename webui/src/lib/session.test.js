import { beforeEach, describe, expect, test, vi } from "vitest";
import { beginSignIn, session } from "./session.svelte.js";

describe("sign in", () => {
	beforeEach(() => {
		session.signingIn = false;
	});

	test("the first activation enters the pending state and refuses repeats", () => {
		const first = { preventDefault: vi.fn() };
		const repeat = { preventDefault: vi.fn() };

		beginSignIn(first);

		expect(session.signingIn).toBe(true);
		expect(first.preventDefault).not.toHaveBeenCalled();

		beginSignIn(repeat);

		expect(repeat.preventDefault).toHaveBeenCalledOnce();
	});
});
