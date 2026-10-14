<script>
	let { status } = $props();
</script>

<span class="chip chip--{status}">{status}</span>

<style>
	.chip {
		font-family: var(--face-figures);
		font-size: var(--step--1);
		font-weight: 700;
		line-height: var(--step--1-lh);
		letter-spacing: 0.06em;
		text-transform: uppercase;
	}

	.chip--queued {
		color: var(--muted);
	}

	.chip--running {
		color: var(--indigo);
	}

	.chip--succeeded {
		color: var(--ink);
	}

	.chip--failed {
		color: var(--error-ink);
	}
</style>
