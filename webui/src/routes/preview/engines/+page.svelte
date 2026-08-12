<script>
	// The engine fleet as the same nested feed the repo list uses: one block per resolved
	// instance — reachability leading the header, its facts and current work as sub-rows.
	// The seed line states where the roster comes from; the roster itself is DNS, read
	// per load.
	import Button from "$lib/components/Button.svelte";
	import Panel from "$lib/components/Panel.svelte";

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
		<Button>Refresh</Button>
	</div>

	<div class="seed">
		<Panel label="Roster source">
			<dl class="kv">
				<dt class="mono key">DAGGER_ENGINE</dt>
				<dd class="mono">dagger-engine.platform.svc</dd>
				<dt class="mono key">DAGGER_ENGINE_PORT</dt>
				<dd class="mono">1234</dd>
			</dl>
		</Panel>
	</div>

	<ul class="fleet">
		{#each engines as engine (engine.addr)}
			<li class="engine">
				<a class="engine-head" href="/preview/engine/">
					<span class="mono state" class:ok={engine.ok} class:bad={!engine.ok}>
						{engine.ok ? "●" : "○"}
					</span>
					<span class="mono addr">{engine.addr}</span>
					<span class="label reach" class:ok={engine.ok} class:warn={!engine.ok}>
						{engine.ok ? "reachable" : "unreachable"}
					</span>
					<span class="mono chev">›</span>
				</a>

				<span class="subs">
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
				</span>
			</li>
		{/each}
	</ul>
</section>

<style>
	.head {
		display: flex;
		align-items: baseline;
		gap: var(--lead);
		margin-bottom: var(--lead);
	}

	.spacer {
		margin-left: auto;
	}

	.seed {
		margin-bottom: var(--lead);
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
		grid-template-columns: var(--lead) auto 1fr var(--lead);
		align-items: baseline;
		gap: var(--lead-half);
		line-height: var(--lead);
		text-decoration: none;
		color: var(--text);
	}

	.engine-head:hover .addr,
	.engine-head:hover .chev {
		color: var(--accent-signal);
	}

	.chev {
		color: var(--text-muted);
		text-align: center;
	}

	.addr {
		font-size: var(--size-prose);
		font-weight: 600;
		color: var(--accent);
	}

	.reach.ok {
		color: var(--accent-ok);
	}

	.subs {
		display: block;
		margin-left: var(--lead-half);
		padding-left: var(--lead-half);
		box-shadow: 1px 0 0 var(--border) inset;
	}

	.sub {
		display: grid;
		grid-template-columns: var(--lead) 1fr auto;
		align-items: baseline;
		gap: var(--lead-half);
		line-height: var(--lead);
		text-decoration: none;
		color: var(--text);
	}

	a.sub:hover {
		background: var(--surface-quiet);
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
