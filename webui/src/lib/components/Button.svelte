<script>
	// Renders as a link when given href, a button otherwise — the same face either way, so
	// a navigation and an action never look like different species.
	// variant names only the departures from the default face; the default emits no
	// modifier class at all.
	let {
		variant = "",
		href = "",
		target = "",
		disabled = false,
		type,
		title,
		onclick,
		children,
	} = $props();
</script>

<!-- An <a> cannot be disabled, so a disabled link renders as the disabled button
     face — one disabling story for both faces. -->
{#if href && !disabled}
	<a
		class={variant ? `btn key chamfer btn--${variant}` : "btn key chamfer"}
		{href}
		{title}
		target={target || undefined}
		{onclick}
	>
		{@render children()}
	</a>
{:else}
	<button class={variant ? `btn key chamfer btn--${variant}` : "btn key chamfer"} {disabled} {type} {title} {onclick}>
		{@render children()}
	</button>
{/if}

<style>
	.btn {
		--face: var(--soft);
		--on-face: var(--ink);
		justify-content: center;
		min-height: var(--space-12);
		max-width: 100%;
		padding: var(--space-3) var(--space-6);
		clip-path: none;
		font-family: var(--face-wide);
		font-size: var(--step--1);
		font-weight: 700;
		line-height: var(--step--1-lh);
		letter-spacing: 0.03em;
		text-transform: uppercase;
		text-align: center;
		text-decoration: none;
		overflow-wrap: anywhere;
	}

	.btn:not(:disabled):hover {
		--face: var(--hairline);
		--on-face: var(--ink);
	}

	.btn--primary,
	.btn--primary:not(:disabled):hover {
		--face: var(--red);
		--on-face: var(--white);
	}

	.btn.key:focus-visible {
		outline: 2px solid var(--indigo);
		outline-offset: var(--space-1);
	}

	.btn.key:focus-visible::after {
		box-shadow: inset 0 0 0 2px var(--on-face);
	}

	.btn.key:disabled {
		--face: var(--soft);
		--on-face: var(--muted);
		--key-y: 0px;
		--key-lift: 0px;
		--key-edge: 0px;
		cursor: default;
	}
</style>
