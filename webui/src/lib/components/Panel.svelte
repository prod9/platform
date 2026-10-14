<script>
	let { label, children } = $props();
</script>

<div class="panel">
	<p class="label">{label}</p>
	{@render children()}
</div>

<style>
	.panel {
		--field-ground: var(--white);
		display: grid;
		gap: var(--space-6);
		min-width: 0;
		padding: var(--space-6);
		background: var(--white);
		box-shadow: 0 3px 0 var(--slab-edge);
	}

	.label {
		--cap-path: polygon(0 0, 100% 0, 100% calc(100% - var(--space-2)), 0 100%);
		position: relative;
		isolation: isolate;
		max-width: none;
		margin: calc(-1 * var(--space-6)) calc(-1 * var(--space-6)) 0;
		padding: var(--space-3) var(--space-6) calc(var(--space-3) + var(--space-2));
		color: var(--paper);
	}

	.label::before,
	.label::after {
		position: absolute;
		inset: 0;
		clip-path: var(--cap-path);
		content: '';
		pointer-events: none;
	}

	.label::after {
		z-index: -2;
		background: var(--night);
	}

	.label::before {
		z-index: -1;
		background: currentColor;
		mask: url('/art/akira-cap-full-1.webp') center / cover no-repeat;
		opacity: 0.075;
	}

	.panel > :global(*) {
		margin-block: 0;
	}

	.panel > .label {
		margin-top: calc(-1 * var(--space-6));
	}
</style>
