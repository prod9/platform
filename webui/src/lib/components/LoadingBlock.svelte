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
		height: var(--space-6);
		background: var(--soft);
		box-shadow: 0 -1px 0 var(--hairline) inset;
		line-height: var(--space-6);
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
