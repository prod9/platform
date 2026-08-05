<script>
	// Renders as a link when given href, a button otherwise — the same face either way, so
	// a navigation and an action never look like different species.
	let { variant = "ghost", href = "", disabled = false, onclick, children } = $props();
</script>

{#if href}
	<a class="btn btn--{variant}" {href}>{@render children()}</a>
{:else}
	<button class="btn btn--{variant}" {disabled} {onclick}>{@render children()}</button>
{/if}

<style>
	.btn {
		display: inline-block;
		padding: 0 var(--lead-half);
		border: 1px solid var(--border);
		border-radius: var(--radius-sm);
		background: none;
		font-family: var(--p9-support);
		font-size: var(--size-label);
		font-weight: 600;
		line-height: var(--lead);
		letter-spacing: 0.12em;
		text-transform: uppercase;
		text-decoration: none;
		color: var(--text);
		cursor: pointer;
	}

	.btn:hover {
		border-color: var(--accent);
		color: var(--accent);
	}

	/* Primary is the one red note on a view — the key action, never a second one.
	   Filled, not outlined: the confirm has to read as the page's action. */
	.btn--primary {
		border-color: var(--accent-signal);
		background: var(--accent-signal);
		color: var(--surface-raised);
	}

	.btn--primary:hover {
		border-color: var(--accent-signal-strong);
		background: var(--accent-signal-strong);
		color: var(--surface-raised);
	}

	.btn:disabled {
		color: var(--text-muted);
		border-color: var(--border);
		background: none;
		cursor: default;
	}
</style>
