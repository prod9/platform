import { Answered, Refused, createBuild, errorText, getBuild, getManifest } from "./server.js";

// A source observation and its selected modules travel together; changing the ref
// discards that whole review. Only this owner can turn a review into a queue request.
export class BuildRequest {
	#owner;
	#repo;
	#active = true;
	#generation = 0;
	#pending = $state(false);
	#state = $state({ kind: "entry", ref: "", message: "" });
	#source = $state({ kind: "manual" });

	constructor(owner, repo, search = "") {
		this.#owner = owner;
		this.#repo = repo;
		this.#source = retrySource(search);
		if (this.#source.kind === "invalid") {
			this.#state = { kind: "predecessor", ref: "", message: "Invalid retry predecessor id." };
		} else if (this.#source.kind === "retry") {
			this.#state = { kind: "predecessor", ref: "", message: "" };
		}
	}

	get state() { return this.#state; }
	get source() { return this.#source; }
	get busy() { return this.#pending; }
	get canLoad() {
		const resolved = this.#source.kind === "manual" || this.#source.kind === "linked";
		return resolved && this.#active && !this.#pending && this.#state.kind !== "accepted" &&
			this.#state.ref.trim() !== "";
	}
	get canQueue() {
		return this.#active && !this.#pending && this.#state.kind === "reviewed" &&
			this.#state.selected.length > 0;
	}

	async initialize() {
		if (!this.#active || this.#pending || this.#source.kind !== "retry") return;
		const generation = ++this.#generation;
		this.#pending = true;
		this.#state = { kind: "predecessor-loading", ref: "", message: "" };

		try {
			const result = await getBuild(this.#source.id);
			if (!this.#current(generation)) return;
			if (result.outcome !== Answered) {
				const message = result.status === 401 ? "" : `Predecessor unavailable: ${errorText(result)}`;
				this.#state = { kind: "predecessor", ref: "", message };
				return;
			}
			const predecessor = result.body;
			const matches = predecessor.owner.toLowerCase() === this.#owner.toLowerCase() &&
				predecessor.repo.toLowerCase() === this.#repo.toLowerCase();
			if (!matches) {
				this.#state = { kind: "predecessor", ref: "", message: "The predecessor belongs to a different repository." };
				return;
			}

			this.#source = { kind: "linked", predecessor };
			this.#state = { kind: "entry", ref: predecessor.ref, message: "" };
		} catch (error) {
			const message = responseFailure(error);
			if (this.#current(generation)) this.#state = { kind: "predecessor", ref: "", message };
		} finally {
			this.#pending = false;
		}
	}

	setRef(ref) {
		if (this.#source.kind !== "manual" || !this.#active || this.#state.kind === "queueing" || this.#state.ref === ref) {
			return;
		}

		this.#generation++;
		this.#state = { kind: "entry", ref, message: "" };
	}

	setSelected(name, selected) {
		if (!this.#active || this.#pending || this.#state.kind !== "reviewed") {
			return;
		}
		if (!this.#state.manifest.modules.some((module) => module.name === name)) {
			throw new Error(`unknown reviewed module: ${name}`);
		}

		const names = new Set(this.#state.selected);
		if (selected) names.add(name);
		else names.delete(name);
		const selection = this.#state.manifest.modules
			.filter((module) => names.has(module.name))
			.map((module) => module.name);
		this.#state = { ...this.#state, selected: selection, message: "" };
	}

	async load() {
		if (!this.canLoad) {
			return;
		}
		const ref = this.#source.kind === "linked" ? this.#source.predecessor.ref : this.#state.ref.trim();
		const generation = ++this.#generation;
		this.#pending = true;
		this.#state = { kind: "loading", ref, message: "" };

		try {
			const result = await getManifest(this.#owner, this.#repo, ref);
			if (!this.#current(generation)) return;
			if (result.outcome !== Answered) {
				this.#state = { kind: "entry", ref, message: reviewFailure(result) };
				return;
			}

			const available = result.body.modules.map((module) => module.name);
			const original = this.#source.kind === "linked"
				? this.#source.predecessor.modules.map((module) => module.name) : available;
			const selected = available.filter((name) => original.includes(name));
			const removed = original.filter((name) => !available.includes(name));
			this.#state = {
				kind: "reviewed", ref, manifest: result.body,
				selected, removed, message: "",
			};
		} catch (error) {
			const message = responseFailure(error);
			if (this.#current(generation)) this.#state = { kind: "entry", ref, message };
		} finally {
			this.#pending = false;
		}
	}

	async queue() {
		if (!this.canQueue) {
			return 0;
		}
		const review = this.#state;
		const generation = ++this.#generation;
		this.#pending = true;
		this.#state = { ...review, kind: "queueing", message: "" };

		try {
			const lineage = this.#source.kind === "linked" ? { retry_of: this.#source.predecessor.id } : {};
			const result = await createBuild({
				owner: this.#owner, repo: this.#repo, ref: review.ref,
				reviewed_sha: review.manifest.sha, modules: [...review.selected],
				...lineage,
			});
			if (!this.#current(generation)) return 0;
			if (result.outcome === Answered) {
				this.#state = { kind: "accepted", ref: review.ref, build: result.body, message: "" };
				return result.body.id;
			}
			if (result.outcome === Refused && result.status === 409) {
				this.#state = {
					kind: "entry", ref: review.ref,
					message: `Source changed or is no longer available. Load and review it again. ${errorText(result)}`,
				};
				return 0;
			}

			const message = result.status === 401 ? "" : `Build not confirmed: ${errorText(result)}`;
			this.#state = { ...review, message };
		} catch (error) {
			const message = responseFailure(error);
			if (this.#current(generation)) this.#state = { ...review, message: `Build not confirmed: ${message}` };
		} finally {
			this.#pending = false;
		}
		return 0;
	}

	dispose() {
		this.#active = false;
		this.#generation++;
	}

	#current(generation) {
		return this.#active && this.#generation === generation;
	}
}

function retrySource(search) {
	const values = new URLSearchParams(search).getAll("retry_of");
	if (values.length === 0) return { kind: "manual" };
	if (values.length !== 1 || !/^[1-9][0-9]*$/.test(values[0])) return { kind: "invalid" };
	const id = Number(values[0]);
	return Number.isSafeInteger(id) ? { kind: "retry", id } : { kind: "invalid" };
}

function reviewFailure(result) {
	if (result.status === 401) return "";
	const reason = errorText(result);
	if (result.status === 409) return `Manifest absent: ${reason}`;
	if (result.status === 422) return `Manifest invalid: ${reason}`;
	if (result.status === 404) return `Source unavailable: ${reason}`;
	return reason;
}

function responseFailure(error) {
	if (!(error instanceof TypeError || error instanceof SyntaxError)) throw error;
	return `The server response could not be read: ${error.message}`;
}
