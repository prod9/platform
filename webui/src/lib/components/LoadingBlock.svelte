<script>
	const measures = {
		compact: "loading-block--compact",
		standard: "loading-block--standard",
		full: "loading-block--full",
	};

	let { measure = "full" } = $props();

	function measureClass(value) {
		if (!Object.hasOwn(measures, value)) {
			throw new Error(`unknown loading block measure: ${value}`);
		}
		return measures[value];
	}
</script>

<span class="loading-block {measureClass(measure)}" aria-hidden="true">&nbsp;</span>

<style>
	.loading-block {
		display: block;
		max-width: 100%;
		height: var(--lead);
		background: var(--surface-quiet);
		border-radius: var(--radius-sm);
		line-height: var(--lead);
		color: transparent;
		overflow: hidden;
	}

	.loading-block--compact {
		width: 12ch;
	}

	.loading-block--standard {
		width: 24ch;
	}

	.loading-block--full {
		width: 100%;
	}
</style>
