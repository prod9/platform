<script>
	let { title, actions } = $props();
</script>

<header class="section-header">
	<h3>{title}</h3>
	{#if actions}
		<div class="actions">
			{@render actions()}
		</div>
	{/if}
</header>

<style>
	.section-header {
		display: flex;
		align-items: center;
		gap: var(--lead);
		min-height: var(--lead-2);
		margin-bottom: var(--lead);
		box-shadow: 0 -1px 0 var(--border) inset;
	}

	h3 {
		color: var(--accent);
	}

	.actions {
		margin-left: auto;
	}
</style>
