<script>
	// The engine fleet as the same nested feed the repo list uses: one block per resolved
	// instance — reachability leading the header, its facts and current work as sub-rows.
	// The seed line states where the roster comes from; the roster itself is DNS, read
	// per load.
	const engines = [
		{
			addr: "tcp://10.2.1.14:1234",
			ok: true,
			version: "dagger v0.18.5",
			uptime: "up 6d",
			cache: "41 GB cache",
			work: { repo: "prod9/infra", build: 125, tag: "v0.3.12", took: "1m 03s" },
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
		<span class="mono muted">DAGGER_ENGINE = dagger-engine.platform.svc : 1234</span>
	</div>

	<ul class="fleet">
		{#each engines as engine (engine.addr)}
			<li class="engine">
				<span class="engine-head">
					<span class="mono state" class:ok={engine.ok} class:bad={!engine.ok}>
						{engine.ok ? "●" : "○"}
					</span>
					<span class="mono addr">{engine.addr}</span>
					<span class="label" class:warn={!engine.ok}>
						{engine.ok ? "reachable" : "unreachable"}
					</span>
				</span>

				{#if !engine.ok}
					<span class="sub">
						<span class="mono state bad">✗</span>
						<span class="mono warn">did not answer the dial</span>
						<span class="mono muted timing">last seen 41m ago</span>
					</span>
				{:else}
					<span class="sub">
						<span class="mono state"></span>
						<span class="mono muted">{engine.version} · {engine.uptime} · {engine.cache}</span>
					</span>
					{#if engine.work}
						<a class="sub" href="/preview/build/">
							<span class="mono state live">◌</span>
							<span class="mono live">building {engine.work.repo} #{engine.work.build} · {engine.work.tag}</span>
							<span class="mono muted timing">{engine.work.took}</span>
						</a>
					{:else}
						<span class="sub">
							<span class="mono state"></span>
							<span class="mono muted">idle</span>
						</span>
					{/if}
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
		list-style: none;
		margin: 0;
		padding: 0;
	}

	.engine {
		padding: var(--lead-half) 0;
		box-shadow: 0 -1px 0 var(--border) inset;
	}

	.engine-head {
		display: grid;
		grid-template-columns: var(--lead) auto 1fr;
		align-items: baseline;
		gap: var(--lead-half);
		line-height: var(--lead);
	}

	.addr {
		font-weight: 600;
	}

	.sub {
		display: grid;
		grid-template-columns: var(--lead) 1fr auto;
		align-items: baseline;
		gap: var(--lead-half);
		padding-left: var(--lead);
		line-height: var(--lead);
		text-decoration: none;
		color: var(--text);
	}

	a.sub:hover {
		background: var(--surface-raised);
	}

	.state {
		text-align: center;
	}

	.state.ok {
		color: var(--accent-ok);
	}

	.state.bad,
	.bad {
		color: var(--accent-signal);
	}

	.live {
		color: var(--accent);
	}

	.warn {
		color: var(--accent-signal);
	}

	.timing {
		text-align: right;
	}
</style>
