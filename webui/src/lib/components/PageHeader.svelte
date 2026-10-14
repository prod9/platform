<script>
	let { title, metadata, navigation, actions } = $props();
</script>

<header class="page-header">
	<div class="identity">
		<div class="title">{@render title()}</div>
		{#if metadata}
			{@render metadata()}
		{/if}
		{#if navigation}
			{@render navigation()}
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
		gap: var(--space-6);
		min-height: var(--space-18);
		margin-bottom: var(--space-6);
		padding-block: var(--space-3);
		box-shadow: 0 -1px 0 var(--hairline) inset;
	}

	.identity {
		display: flex;
		align-items: baseline;
		gap: var(--space-6);
		min-width: 0;
	}

	.title {
		min-width: 0;
		overflow-wrap: anywhere;
	}

	.page-header :global(h2) {
		font-size: var(--step-2);
		line-height: var(--step-2-lh);
	}

	.page-header :global(h2 a) {
		text-decoration: none;
	}

	.identity > :global(p) {
		margin: 0;
	}

	.actions {
		display: flex;
		flex-wrap: wrap;
		gap: var(--space-3);
		margin-left: auto;
		flex-shrink: 0;
	}

	@media (max-width: 64rem) {
		.page-header {
			align-items: flex-start;
			flex-wrap: wrap;
		}

		.identity {
			flex-wrap: wrap;
			row-gap: var(--space-3);
		}
	}

	@media (max-width: 40rem) {
		.page-header {
			gap: var(--space-3);
		}

		.identity {
			width: 100%;
		}

		.actions {
			margin-left: 0;
		}
	}
</style>
