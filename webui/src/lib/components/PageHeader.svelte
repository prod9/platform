<script>
	let { title, metadata, actions } = $props();
</script>

<header class="page-header">
	<div class="identity">
		{@render title()}
		{#if metadata}
			{@render metadata()}
		{/if}
	</div>
	{#if actions}
		<div class="actions">
			{@render actions()}
		</div>
	{/if}
</header>

<style>
	.page-header {
		display: flex;
		align-items: center;
		gap: var(--lead);
		height: var(--lead-2);
		margin-bottom: var(--lead);
	}

	.identity {
		display: flex;
		align-items: baseline;
		gap: var(--lead);
	}

	.page-header :global(h2 a) {
		text-decoration: none;
	}

	.actions {
		margin-left: auto;
	}
</style>
