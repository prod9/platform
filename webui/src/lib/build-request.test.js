import { beforeEach, describe, expect, test, vi } from "vitest";
import { BuildRequest } from "./build-request.svelte.js";
import { createBuild, getBuild, getManifest, Answered, Refused, Offline } from "./server.js";

vi.mock("./server.js", async (original) => ({
	...await original(),
	getManifest: vi.fn(),
	getBuild: vi.fn(),
	createBuild: vi.fn(),
}));

const manifest = {
	sha: "abcdef123456", raw: "[modules.api]\nframework = 'go/basic'",
	publish_policy: "never",
	modules: [{ name: "api", framework: "go/basic" }, { name: "web", framework: "pnpm/static" }],
};

function deferred() {
	let resolve;
	const promise = new Promise((finish) => { resolve = finish; });
	return { promise, resolve };
}

beforeEach(() => {
	vi.clearAllMocks();
	getManifest.mockResolvedValue({ outcome: Answered, body: manifest });
	createBuild.mockResolvedValue({ outcome: Answered, body: { id: 92 } });
	getBuild.mockResolvedValue({ outcome: Answered, body: {
		id: 81, owner: "prod9", repo: "platform", ref: "refs/heads/main",
		sha: "old-sha", status: "running", modules: [{ name: "api" }, { name: "removed" }],
	} });
});

describe("linked retry review", () => {
	test("predecessor authorization must finish before source review and late responses cannot revive a disposed request", async () => {
		const reading = deferred();
		getBuild.mockReturnValue(reading.promise);
		const request = new BuildRequest("prod9", "platform", "?retry_of=81");
		await request.load();
		const initialize = request.initialize();
		await request.initialize();
		await request.load();
		expect(getBuild).toHaveBeenCalledTimes(1);
		expect(getManifest).not.toHaveBeenCalled();
		request.dispose();
		reading.resolve({ outcome: Answered, body: {
			id: 81, owner: "prod9", repo: "platform", ref: "refs/heads/main", modules: [],
		} });
		await initialize;
		await request.load();
		expect(await request.queue()).toBe(0);
		expect(getManifest).not.toHaveBeenCalled();
	});

	test("resolves the exact predecessor ref again and starts with surviving original modules", async () => {
		const request = new BuildRequest("prod9", "platform", "?retry_of=81");
		await request.initialize();
		request.setRef("refs/heads/other");
		await request.load();

		expect(request.state.selected).toEqual(["api"]);
		expect(request.state.removed).toEqual(["removed"]);
		expect(await request.queue()).toBe(92);
		expect(getBuild).toHaveBeenCalledExactlyOnceWith(81);
		expect(createBuild).toHaveBeenCalledExactlyOnceWith({
			owner: "prod9", repo: "platform", ref: "refs/heads/main",
			reviewed_sha: manifest.sha, modules: ["api"], retry_of: 81,
		});
	});

	test.each(["", "0", "-2", "1x", "01", "9007199254740992", "1&retry_of=2"])(
		"invalid retry id %s never falls back to a manual request", async (value) => {
			const request = new BuildRequest("prod9", "platform", `?retry_of=${value}`);
			await request.initialize();
			request.setRef("refs/heads/main");
			await request.load();
			expect(await request.queue()).toBe(0);
			expect(getBuild).not.toHaveBeenCalled();
			expect(getManifest).not.toHaveBeenCalled();
			expect(createBuild).not.toHaveBeenCalled();
			expect(request.state.message).toContain("Invalid retry");
		},
	);

	test.each([
		{ outcome: Refused, status: 404, body: "not found" },
		{ outcome: Offline, body: "" },
		{ outcome: Answered, body: { id: 81, owner: "prod9", repo: "other" } },
	])("a refused or mismatched predecessor cannot reach source review", async (result) => {
		getBuild.mockResolvedValue(result);
		const request = new BuildRequest("prod9", "platform", "?retry_of=81");
		await request.initialize();
		await request.load();
		expect(await request.queue()).toBe(0);
		expect(getManifest).not.toHaveBeenCalled();
		expect(createBuild).not.toHaveBeenCalled();
		expect(request.state.message).not.toBe("");
	});

	test("cannot queue when all original modules vanished until a new module is explicitly selected", async () => {
		getManifest.mockResolvedValue({ outcome: Answered, body: { ...manifest, modules: [{ name: "new" }] } });
		const request = new BuildRequest("prod9", "platform", "?retry_of=81");
		await request.initialize();
		await request.load();
		expect(await request.queue()).toBe(0);
		request.setSelected("new", true);
		expect(await request.queue()).toBe(92);
		expect(createBuild.mock.calls[0][0].modules).toEqual(["new"]);
	});

	test("a moved retry source keeps its predecessor and requires a fresh review", async () => {
		createBuild.mockResolvedValue({ outcome: Refused, status: 409, body: "source moved" });
		const request = new BuildRequest("prod9", "platform", "?retry_of=81");
		await request.initialize();
		await request.load();
		await request.queue();
		await request.load();
		expect(request.state.selected).toEqual(["api"]);
		expect(getBuild).toHaveBeenCalledTimes(1);
		expect(getManifest).toHaveBeenCalledTimes(2);
		expect(createBuild).toHaveBeenCalledTimes(1);
	});
});

describe("reviewed build request", () => {
	test("queues only selected modules with the reviewed source and ref", async () => {
		const request = new BuildRequest("prod9", "platform");
		request.setRef(" refs/heads/main ");
		await request.load();
		request.setSelected("web", false);

		expect(await request.queue()).toBe(92);
		expect(getManifest).toHaveBeenCalledWith("prod9", "platform", "refs/heads/main");
		expect(createBuild).toHaveBeenCalledExactlyOnceWith({
			owner: "prod9", repo: "platform", ref: "refs/heads/main",
			reviewed_sha: manifest.sha, modules: ["api"],
		});
	});

	test("ref edits discard review and module selection before another load", async () => {
		const request = new BuildRequest("prod9", "platform");
		request.setRef("refs/heads/main");
		await request.load();
		request.setSelected("api", false);
		request.setRef("refs/heads/next");

		expect(await request.queue()).toBe(0);
		expect(createBuild).not.toHaveBeenCalled();
		expect(request.state.kind).toBe("entry");
		await request.load();
		expect(await request.queue()).toBe(92);
		expect(createBuild.mock.calls[0][0].modules).toEqual(["api", "web"]);
	});

	test("cannot submit an empty selection or a manifest with no modules", async () => {
		const request = new BuildRequest("prod9", "platform");
		request.setRef("refs/heads/main");
		await request.load();
		request.setSelected("api", false);
		request.setSelected("web", false);
		expect(await request.queue()).toBe(0);
		getManifest.mockResolvedValue({ outcome: Answered, body: { ...manifest, modules: [] } });
		await request.load();
		expect(await request.queue()).toBe(0);
		expect(createBuild).not.toHaveBeenCalled();
	});

	test("suppresses competing reads and queues while a request is pending", async () => {
		const reading = deferred();
		getManifest.mockReturnValue(reading.promise);
		const request = new BuildRequest("prod9", "platform");
		request.setRef("refs/heads/main");
		const load = request.load();
		await request.load();
		await request.queue();
		expect(getManifest).toHaveBeenCalledTimes(1);
		expect(createBuild).not.toHaveBeenCalled();
		reading.resolve({ outcome: Answered, body: manifest });
		await load;
		const queued = deferred();
		createBuild.mockReturnValue(queued.promise);
		const queue = request.queue();
		await request.queue();
		await request.load();
		expect(createBuild).toHaveBeenCalledTimes(1);
		expect(getManifest).toHaveBeenCalledTimes(1);
		queued.resolve({ outcome: Answered, body: { id: 93 } });
		expect(await queue).toBe(93);
	});

	test("a moved ref clears review and never resubmits automatically", async () => {
		createBuild.mockResolvedValue({ outcome: Refused, status: 409, body: "source moved" });
		const request = new BuildRequest("prod9", "platform");
		request.setRef("refs/heads/main");
		await request.load();

		expect(await request.queue()).toBe(0);
		expect(request.state.kind).toBe("entry");
		expect(request.state.message).toContain("Source changed");
		await request.queue();
		expect(createBuild).toHaveBeenCalledTimes(1);
		expect(getManifest).toHaveBeenCalledTimes(1);
	});

	test.each(["ref edit", "dispose"])("ignores a late manifest response after %s", async (change) => {
		const reading = deferred();
		getManifest.mockReturnValue(reading.promise);
		const request = new BuildRequest("prod9", "platform");
		request.setRef("refs/heads/main");
		const load = request.load();
		if (change === "dispose") request.dispose();
		else request.setRef("refs/heads/next");
		reading.resolve({ outcome: Answered, body: manifest });
		await load;

		expect(request.state.kind).not.toBe("reviewed");
		expect(await request.queue()).toBe(0);
		expect(createBuild).not.toHaveBeenCalled();
	});

	test("does not navigate from a late accepted queue response after disposal", async () => {
		const queued = deferred();
		createBuild.mockReturnValue(queued.promise);
		const request = new BuildRequest("prod9", "platform");
		request.setRef("refs/heads/main");
		await request.load();
		const queue = request.queue();
		request.dispose();
		queued.resolve({ outcome: Answered, body: { id: 94 } });
		expect(await queue).toBe(0);
	});

	test("a failed queue preserves explicit selection without an automatic second request", async () => {
		createBuild.mockResolvedValue({ outcome: Offline, body: "" });
		const request = new BuildRequest("prod9", "platform");
		request.setRef("refs/heads/main");
		await request.load();
		request.setSelected("web", false);

		expect(await request.queue()).toBe(0);
		expect(request.state.kind).toBe("reviewed");
		expect(request.state.selected).toEqual(["api"]);
		expect(request.state.message).toContain("Build not confirmed");
		expect(createBuild).toHaveBeenCalledTimes(1);
	});

	test("reauthentication responses remain shell-owned instead of page errors", async () => {
		getManifest.mockResolvedValue({ outcome: Refused, status: 401, body: "expired" });
		const request = new BuildRequest("prod9", "platform");
		request.setRef("refs/heads/main");
		await request.load();
		expect(request.state.message).toBe("");
		expect(await request.queue()).toBe(0);
		expect(createBuild).not.toHaveBeenCalled();
	});

	test("invalid response data is surfaced and can be explicitly loaded again", async () => {
		getManifest.mockRejectedValueOnce(new SyntaxError("invalid JSON"));
		const request = new BuildRequest("prod9", "platform");
		request.setRef("refs/heads/main");
		await request.load();
		expect(request.state.message).toContain("invalid JSON");
		await request.load();
		expect(await request.queue()).toBe(92);
	});

	test.each([
		[{ outcome: Refused, status: 409, body: "no manifest" }, "Manifest absent"],
		[{ outcome: Refused, status: 422, body: "bad manifest" }, "Manifest invalid"],
		[{ outcome: Refused, status: 404, body: "unreachable" }, "Source unavailable"],
		[{ outcome: Offline, body: "" }, "No answer"],
	])("keeps preview failure honest without admitting a build", async (failure, message) => {
		getManifest.mockResolvedValue(failure);
		const request = new BuildRequest("prod9", "platform");
		request.setRef("refs/heads/main");
		await request.load();

		expect(request.state.kind).toBe("entry");
		expect(request.state.message).toContain(message);
		expect(createBuild).not.toHaveBeenCalled();
	});
});
