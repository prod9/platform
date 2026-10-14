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
		flex-wrap: wrap;
		gap: var(--space-3) var(--space-6);
		min-height: var(--space-12);
		margin-bottom: var(--space-6);
		padding-block: var(--space-3);
		box-shadow: 0 -1px 0 var(--hairline) inset;
	}

	h3 {
		min-width: 0;
		color: var(--indigo);
		overflow-wrap: anywhere;
	}

	.actions {
		display: flex;
		flex-wrap: wrap;
		gap: var(--space-3);
		margin-left: auto;
	}
</style>
