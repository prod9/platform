<script>
	// The outcome mark: one glyph per build outcome, colored by the same scale
	// everywhere it appears (docs/spec/webui.md §Shared components). "none" is the
	// no-builds-yet placeholder the nested feeds use.
	let { status } = $props();

	const marks = { succeeded: "✓", failed: "✗", running: "◌", queued: "·", none: "·" };
</script>

<span class="mono state state--{status}">{marks[status] ?? "·"}</span>

<style>
	.state {
		text-align: center;
	}

	.state--succeeded {
		color: var(--accent-ok);
	}

	.state--failed {
		color: var(--accent-signal);
	}

	.state--running {
		color: var(--accent);
	}

	.state--none,
	.state--queued {
		color: var(--text-muted);
	}
</style>
