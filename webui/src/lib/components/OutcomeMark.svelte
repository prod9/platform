<script>
	// The outcome mark: one glyph per build outcome, colored by the same scale
	// everywhere it appears (docs/spec/webui.md §Shared components). "none" is the
	// no-builds-yet placeholder the nested feeds use.
	let { status } = $props();

	const marks = { succeeded: "✓", failed: "✗", running: "◌", queued: "·", none: "·" };

	function markOf(value) {
		if (!Object.hasOwn(marks, value)) {
			throw new Error(`unknown outcome status: ${value}`);
		}
		return marks[value];
	}
</script>

<span class="mono outcome state--{status}">
	<span class="mark" aria-hidden="true">{markOf(status)}</span>
	<span class="value">{status}</span>
</span>

<style>
	.outcome {
		display: inline-grid;
		grid-template-columns: var(--lead) auto;
		align-items: baseline;
		line-height: var(--lead);
	}

	.mark {
		text-align: center;
	}

	.state--succeeded .mark {
		color: var(--accent-ok);
	}

	.state--failed .mark {
		color: var(--accent-signal);
	}

	.state--running .mark {
		color: var(--accent);
	}

	.state--none .mark,
	.state--queued .mark,
	.value {
		color: var(--text-muted);
	}
</style>
