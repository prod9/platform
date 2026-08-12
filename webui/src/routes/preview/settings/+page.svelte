<script>
	// The install-time facts, read-only, secrets shown as presence only. Section heads
	// carry the structure in indigo; keys are the machine's voice, muted; values are the
	// data, in ink.
	const sections = [
		{
			name: "Server",
			facts: [
				{ key: "public_url", value: "https://platform.prodigy9.co" },
				{ key: "org", value: "prod9" },
			],
		},
		{
			name: "GitHub App",
			facts: [
				{ key: "app", value: "prod9-platform (id 412887)" },
				{ key: "client_id", value: "Iv1.8a61f9b3a7aba766" },
				{ key: "private_key", value: "✓ saved", ok: true },
				{ key: "webhook_secret", value: "✓ saved", ok: true },
				{ key: "client_secret", value: "✓ saved", ok: true },
			],
		},
		{
			name: "Registry",
			facts: [{ key: "ghcr.io token", value: "✓ saved · pushes as chakrit", ok: true }],
		},
	];
</script>

<section>
	<div class="head">
		<h2>Settings</h2>
	</div>

	{#each sections as group (group.name)}
		<div class="group">
			<h3 class="crosshead">{group.name}</h3>
			<dl class="kv">
				{#each group.facts as fact (fact.key)}
					<dt class="mono key">{fact.key}</dt>
					<dd class="mono" class:ok={fact.ok}>{fact.value}</dd>
				{/each}
			</dl>
		</div>
	{/each}
</section>

<style>
	section {
		max-width: 90ch;
	}

	.head {
		display: flex;
		align-items: baseline;
		gap: var(--lead);
		margin-bottom: var(--lead);
	}

	.group {
		margin-bottom: var(--lead-2);
	}

	.crosshead {
		color: var(--accent);
		margin-bottom: var(--lead-half);
		box-shadow: 0 -1px 0 var(--border) inset;
	}

	.kv {
		display: grid;
		grid-template-columns: 22ch minmax(0, 1fr);
		gap: 0 var(--lead);
		margin: 0;
	}

	.kv dt,
	.kv dd {
		margin: 0;
		line-height: var(--lead);
	}

	.key {
		color: var(--text-muted);
	}

	.ok {
		color: var(--accent-ok);
	}
</style>
