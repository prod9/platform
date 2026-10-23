import { Answered, errorText } from "./server.js";

// One explicit read owns the whole fleet or instance snapshot, including stale recovery.
export class EngineRead {
	#read;
	#active = true;
	#pending = $state(false);
	#state = $state({ phase: "loading", snapshot: null, error: "" });

	constructor(read) { this.#read = read; }

	get state() { return this.#state; }
	get busy() { return this.#pending; }

	async refresh() {
		if (!this.#active || this.#pending) return;
		this.#pending = true;
		this.#state = {
			...this.#state,
			phase: this.#state.snapshot === null ? "loading" : "refreshing",
		};

		try {
			const result = await this.#read();
			if (!this.#active) return;
			if (result.outcome !== Answered) {
				this.#state = { ...this.#state, phase: "unavailable", error: errorText(result) };
				return;
			}
			this.#state = { phase: "ready", snapshot: result.body, error: "" };
		} catch (error) {
			if (!(error instanceof TypeError || error instanceof SyntaxError)) throw error;
			if (this.#active) {
				this.#state = { ...this.#state, phase: "unavailable", error: error.message };
			}
		} finally {
			this.#pending = false;
		}
	}

	dispose() { this.#active = false; }
}
