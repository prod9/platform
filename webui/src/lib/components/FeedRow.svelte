<script>
	let { href, mark, subject, facts, timing } = $props();
</script>

{#snippet contents()}
	<span class="mark">{@render mark()}</span>
	<span class="subject mono">{@render subject()}</span>
	{#if facts}<span class="facts mono muted">{@render facts()}</span>{/if}
	{#if timing}<span class="timing mono muted">{@render timing()}</span>{/if}
{/snippet}

{#if href}
	<a class="row" {href}>{@render contents()}</a>
{:else}
	<div class="row">{@render contents()}</div>
{/if}

<style>
	.row {
		display: grid;
		grid-template-columns: var(--space-6) minmax(0, 1fr) minmax(0, 1.5fr) minmax(0, 1fr);
		align-items: baseline;
		gap: var(--space-2) var(--space-3);
		padding-block: var(--space-3);
		color: var(--ink);
		text-decoration: none;
		min-width: 0;
	}

	.row > span {
		min-width: 0;
		overflow-wrap: anywhere;
	}

	a.row:hover {
		background: var(--soft);
	}

	.timing {
		text-align: right;
	}

	@media (max-width: 48rem) {
		.row {
			grid-template-columns: var(--space-6) minmax(0, 1fr);
		}

		.facts,
		.timing {
			grid-column: 2;
			text-align: left;
		}
	}
</style>
