<script>
	// The outcome mark: one glyph per build outcome, colored by the same scale
	// everywhere it appears (docs/spec/webui.md §Shared components). "none" is the
	// no-builds-yet placeholder the nested feeds use.
	import SignalMark from "$lib/components/SignalMark.svelte";

	let { status } = $props();

	const signals = {
		succeeded: "positive",
		failed: "negative",
		running: "active",
		queued: "pending",
		none: "pending",
	};

	function signalOf(value) {
		if (!Object.hasOwn(signals, value)) {
			throw new Error(`unknown outcome status: ${value}`);
		}
		return signals[value];
	}
</script>

<SignalMark signal={signalOf(status)} label={status} />
