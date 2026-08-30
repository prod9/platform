<script>
	const signals = {
		positive: { glyph: "✓", className: "signal--positive" },
		negative: { glyph: "✗", className: "signal--negative" },
		active: { glyph: "◌", className: "signal--active" },
		pending: { glyph: "·", className: "signal--pending" },
		navigation: { glyph: "›", className: "signal--navigation" },
	};

	let { signal, label = "" } = $props();

	function definitionOf(value) {
		if (!Object.hasOwn(signals, value)) {
			throw new Error(`unknown signal: ${value}`);
		}
		return signals[value];
	}

	let definition = $derived(definitionOf(signal));
</script>

<span
	class="mono signal {definition.className}"
	role={label ? "img" : undefined}
	aria-label={label || undefined}
	aria-hidden={label ? undefined : "true"}
>
	<span aria-hidden="true">{definition.glyph}</span>
</span>

<style>
	.signal {
		display: inline-block;
		width: var(--lead);
		line-height: var(--lead);
		text-align: center;
	}

	.signal--positive {
		color: var(--accent-ok);
	}

	.signal--negative {
		color: var(--accent-signal);
	}

	.signal--active {
		color: var(--accent);
	}

	.signal--pending {
		color: var(--text-muted);
	}

	.signal--navigation {
		color: var(--navigation-mark, var(--text-muted));
	}
</style>
