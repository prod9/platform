import { describe, expect, test, vi } from "vitest";
import { EngineRead } from "./engine-read.svelte.js";
import { Answered, Refused, Offline } from "./server.js";

function deferred() {
	let resolve;
	const promise = new Promise((finish) => { resolve = finish; });
	return { promise, resolve };
}

describe("engine snapshots", () => {
	test.each([
		{ outcome: Refused, status: 503, body: "Discovery unavailable" },
		{ outcome: Offline, body: "" },
	])("failed refresh preserves the complete previous snapshot", async (failed) => {
		const snapshot = { source: "engines.test", engines: [{ address: "10.0.0.1:1234" }] };
		const read = vi.fn().mockResolvedValueOnce({ outcome: Answered, body: snapshot })
			.mockResolvedValueOnce(failed)
			.mockResolvedValueOnce({ outcome: Answered, body: { source: "changed", engines: [] } });
		const model = new EngineRead(read);
		await model.refresh();
		await model.refresh();
		expect(model.state.phase).toBe("unavailable");
		expect(model.state.snapshot).toEqual(snapshot);
		expect(model.state.error).not.toBe("");
		await model.refresh();
		expect(model.state.phase).toBe("ready");
		expect(model.state.snapshot.source).toBe("changed");
		expect(model.state.error).toBe("");
	});

	test("overlapping refreshes cannot race or replace a disposed route", async () => {
		const pending = deferred();
		const read = vi.fn().mockReturnValue(pending.promise);
		const model = new EngineRead(read);
		const first = model.refresh();
		await model.refresh();
		expect(read).toHaveBeenCalledTimes(1);
		expect(model.busy).toBe(true);
		model.dispose();
		pending.resolve({ outcome: Answered, body: { engines: [] } });
		await first;
		expect(model.state.snapshot).toBeNull();
		await model.refresh();
		expect(read).toHaveBeenCalledTimes(1);
	});

	test("malformed responses retain useful data and allow explicit recovery", async () => {
		const model = new EngineRead(vi.fn().mockRejectedValue(new SyntaxError("Invalid JSON")));
		await model.refresh();
		expect(model.state.phase).toBe("unavailable");
		expect(model.state.snapshot).toBeNull();
		expect(model.state.error).toContain("Invalid JSON");
		expect(model.busy).toBe(false);
	});

	test("unexpected programming failures propagate", async () => {
		const model = new EngineRead(vi.fn().mockRejectedValue(new Error("Broken invariant")));
		await expect(model.refresh()).rejects.toThrow("Broken invariant");
		expect(model.busy).toBe(false);
	});
});
