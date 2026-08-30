<script>
	const measures = {
		compact: "facts--compact",
		standard: "facts--standard",
		wide: "facts--wide",
	};

	let { measure = "wide", children } = $props();

	function measureClass(value) {
		if (!Object.hasOwn(measures, value)) {
			throw new Error(`unknown facts measure: ${value}`);
		}
		return measures[value];
	}
</script>

<dl class="facts {measureClass(measure)}">
	{@render children()}
</dl>

<style>
	.facts {
		display: grid;
		gap: 0 var(--lead);
		margin: 0;
	}

	.facts--compact {
		grid-template-columns: 12ch minmax(0, 1fr);
	}

	.facts--standard {
		grid-template-columns: 18ch minmax(0, 1fr);
	}

	.facts--wide {
		grid-template-columns: 22ch minmax(0, 1fr);
	}

	.facts :global(dt),
	.facts :global(dd) {
		margin: 0;
		line-height: var(--lead);
	}

	.facts :global(dt) {
		color: var(--text-muted);
	}
</style>
