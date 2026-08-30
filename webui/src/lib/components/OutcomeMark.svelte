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

<span class="mono outcome state--{status}" role="img" aria-label={status}>
	<span class="mark" aria-hidden="true">{markOf(status)}</span>
</span>

<style>
	.outcome {
		display: inline-block;
		width: var(--lead);
		line-height: var(--lead);
		text-align: center;
	}

	.mark {
		display: block;
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
	.state--queued .mark {
		color: var(--text-muted);
	}
</style>
