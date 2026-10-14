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
		display: inline-grid;
		place-items: center;
		flex-shrink: 0;
		width: var(--space-6);
		font-weight: 700;
		line-height: var(--step-0-lh);
		text-align: center;
	}

	.signal--positive {
		color: var(--signal-positive, var(--ink));
	}

	.signal--negative {
		color: var(--signal-negative, var(--red));
	}

	.signal--active {
		color: var(--signal-active, var(--indigo));
	}

	.signal--pending {
		color: var(--signal-pending, var(--muted));
	}

	.signal--navigation {
		color: var(--signal-navigation, var(--navigation-mark, var(--muted)));
	}
</style>
