<script>
	// A build's folded outcome. Failure is the only state that earns the red — the rest
	// carry system colors, so a failed row is the one thing the eye lands on.
	let { status } = $props();
</script>

<span class="chip chip--{status}">{status}</span>

<style>
	.chip {
		font-family: var(--p9-support);
		font-size: var(--size-label);
		font-weight: 600;
		line-height: var(--lead);
		letter-spacing: 0.12em;
		text-transform: uppercase;
	}

	.chip--queued {
		color: var(--text-muted);
	}

	/* Running is the only live state, so it gets the system's authority color; success is
	   the norm and stays quiet. */
	.chip--running {
		color: var(--accent);
	}

	.chip--succeeded {
		color: var(--text);
	}

	.chip--failed {
		color: var(--accent-signal);
	}
</style>
