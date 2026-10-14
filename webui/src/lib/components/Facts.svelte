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
		gap: var(--space-1) var(--space-6);
		min-width: 0;
		margin: 0;
	}

	.facts--compact {
		grid-template-columns: minmax(0, 12ch) minmax(0, 1fr);
	}

	.facts--standard {
		grid-template-columns: 18ch minmax(0, 1fr);
	}

	.facts--wide {
		grid-template-columns: 22ch minmax(0, 1fr);
	}

	.facts :global(dt),
	.facts :global(dd) {
		min-width: 0;
		margin: 0;
		line-height: var(--step-0-lh);
		overflow-wrap: anywhere;
	}

	.facts :global(dt) {
		color: var(--muted);
	}

	@media (max-width: 40rem) {
		.facts--standard,
		.facts--wide {
			grid-template-columns: minmax(0, 1fr);
			gap: 0;
		}

		.facts--standard :global(dd),
		.facts--wide :global(dd) {
			margin-bottom: var(--space-3);
		}

		.facts :global(dd:last-child) {
			margin-bottom: 0;
		}
	}

	@media (width < 24rem) {
		.facts--compact {
			grid-template-columns: minmax(0, 1fr);
			gap: 0;
		}

		.facts--compact :global(dd) {
			margin-bottom: var(--space-3);
		}

		.facts--compact :global(dd:last-child) {
			margin-bottom: 0;
		}
	}
</style>
