<script>
	let { observation } = $props();
</script>

{#if observation?.state === "quiet"}
	<span class="quiet mono">No recent observations · {observation.last_observed_at}</span>
{/if}

<style>
	.quiet {
		display: block;
		color: var(--error-ink);
		overflow-wrap: anywhere;
	}
</style>
