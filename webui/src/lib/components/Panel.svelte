<script>
	// A plate: a real border, because the frame is the point and its width is absorbed
	// into the padding. Carries a caps label and its content, nothing else.
	let { label, children } = $props();
</script>

<div class="panel">
	<p class="label">{label}</p>
	{@render children()}
</div>

<style>
	.panel {
		padding: var(--lead-half) var(--lead);
		border: 1px solid var(--border);
		border-radius: var(--radius-md);
		background: var(--surface-raised);
		box-shadow: var(--raised-shadow);
	}

	.label {
		margin: 0;
	}
</style>
