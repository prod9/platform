<script>
	// A build's folded outcome, in CI semantics: green passed, red failed, indigo live,
	// muted waiting. Red stays failure's alone, so a failed row is what the eye lands on.
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

	/* Running is the only live state, so it gets the system's authority color. */
	.chip--running {
		color: var(--accent);
	}

	.chip--succeeded {
		color: var(--accent-ok);
	}

	.chip--failed {
		color: var(--accent-signal);
	}
</style>
