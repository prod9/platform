<script>
	// The engine fleet, one card per resolved instance: address, reachability, engine
	// version, and what it is running right now. The seed line states where the roster
	// comes from; the roster itself is DNS, read per load.
	const engines = [
		{
			addr: "tcp://10.2.1.14:1234",
			ok: true,
			version: "dagger v0.18.5",
			uptime: "up 6d",
			cache: "41 GB cache",
			work: { repo: "prod9/infra", build: 125, tag: "v0.3.12" },
		},
		{
			addr: "tcp://10.2.3.87:1234",
			ok: true,
			version: "dagger v0.18.5",
			uptime: "up 2d",
			cache: "17 GB cache",
			work: null,
		},
		{
			addr: "tcp://10.2.4.2:1234",
			ok: false,
			version: "",
			uptime: "",
			cache: "",
			work: null,
		},
	];
</script>

<section>
	<div class="head">
		<h2>Engines</h2>
		<p class="label">{engines.length} resolved</p>
		<span class="spacer"></span>
		<span class="mono muted seed">DAGGER_ENGINE = dagger-engine.platform.svc : 1234</span>
	</div>

	<ul class="fleet">
		{#each engines as engine (engine.addr)}
			<li class="card" class:down={!engine.ok}>
				<div class="top">
					<span class="mono dot" class:ok={engine.ok}>{engine.ok ? "●" : "○"}</span>
					<span class="mono addr">{engine.addr}</span>
					<span class="chip label" class:bad={!engine.ok}>
						{engine.ok ? "reachable" : "unreachable"}
					</span>
				</div>

				{#if engine.ok}
					<div class="mono muted facts">
						{engine.version} · {engine.uptime} · {engine.cache}
					</div>
					{#if engine.work}
						<a class="mono work" href="/preview/build/">
							◌ building {engine.work.repo} #{engine.work.build} · {engine.work.tag}
						</a>
					{:else}
						<span class="mono muted">idle</span>
					{/if}
				{:else}
					<span class="mono warn">did not answer the dial · last seen 41m ago</span>
				{/if}
			</li>
		{/each}
	</ul>
</section>

<style>
	section {
		max-width: 100ch;
	}

	.head {
		display: flex;
		align-items: baseline;
		gap: var(--lead);
		margin-bottom: var(--lead);
	}

	.spacer {
		margin-left: auto;
	}

	.fleet {
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(38ch, 1fr));
		gap: var(--lead);
		list-style: none;
		margin: 0;
		padding: 0;
	}

	.card {
		padding: var(--lead-half) var(--lead) var(--lead);
		border: 1px solid var(--border);
		border-radius: var(--radius-md);
		background: var(--surface-raised);
		box-shadow: var(--raised-shadow);
	}

	.card.down {
		background: none;
	}

	.top {
		display: flex;
		align-items: baseline;
		gap: var(--lead-half);
		margin-bottom: var(--lead-half);
	}

	.dot {
		color: var(--text-muted);
	}

	.dot.ok {
		color: var(--accent-ok);
	}

	.addr {
		font-weight: 600;
	}

	.chip {
		margin-left: auto;
	}

	.chip.bad {
		color: var(--accent-signal);
	}

	.facts {
		margin-bottom: var(--lead-half);
	}

	.work {
		color: var(--accent);
		text-decoration: none;
	}

	.work:hover {
		color: var(--accent-signal);
	}

	.warn {
		color: var(--accent-signal);
	}
</style>
