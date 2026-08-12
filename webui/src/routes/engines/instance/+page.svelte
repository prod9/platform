<script>
	// One engine instance: its facts, the builds it has carried, and its live log — the
	// same night-ground terminal the build detail uses.
	const recent = [
		{ id: 125, repo: "prod9/infra", tag: "v0.3.12", status: "running", took: "1m 03s", when: "4m ago" },
		{ id: 128, repo: "prod9/platform", tag: "v0.9.36", status: "succeeded", took: "4m 12s", when: "2h ago" },
		{ id: 126, repo: "prod9/platform", tag: "2f4c1d9", status: "succeeded", took: "3m 45s", when: "6h ago" },
	];

	const marks = { succeeded: "✓", failed: "✗", running: "◌", queued: "·" };

	const log =
		"time=2026-08-12T09:14:03Z level=INFO msg=\"session opened\" client=platform-worker\n" +
		"time=2026-08-12T09:14:04Z level=INFO msg=\"solve start\" build=125 unit=infra\n" +
		"time=2026-08-12T09:14:11Z level=INFO msg=\"cache hit\" layers=41 ratio=0.87\n" +
		"time=2026-08-12T09:15:02Z level=INFO msg=\"exporting image\" ref=ghcr.io/prod9/infra:v0.3.12\n" +
		"time=2026-08-12T09:15:06Z level=WARN msg=\"gc pressure\" cache=41GB budget=48GB";
</script>

<section>
	<div class="head">
		<h2><a href="/engines/">Engines</a> / 10.2.1.14</h2>
		<span class="label ok">reachable</span>
	</div>

	<dl class="kv">
		<dt class="mono key">address</dt>
		<dd class="mono">tcp://10.2.1.14:1234</dd>
		<dt class="mono key">version</dt>
		<dd class="mono">dagger v0.18.5</dd>
		<dt class="mono key">uptime</dt>
		<dd class="mono">6d 4h</dd>
		<dt class="mono key">cache</dt>
		<dd class="mono">41 GB · 87% hit ratio</dd>
		<dt class="mono key">now</dt>
		<dd class="mono live">◌ building prod9/infra #125 · v0.3.12</dd>
	</dl>

	<div class="cols">
		<div>
			<h3 class="crosshead">Recent builds on this engine</h3>
			<ul class="rows">
				{#each recent as build (build.id)}
					<li>
						<a class="row" href="/builds/127/">
							<span class="mono state state--{build.status}">{marks[build.status]}</span>
							<span class="mono tag">{build.tag}</span>
							<span class="mono muted">{build.repo} · #{build.id}</span>
							<span class="mono muted timing">{build.took} · {build.when}</span>
						</a>
					</li>
				{/each}
			</ul>
		</div>

		<div>
			<h3 class="crosshead">Engine log</h3>
			<pre class="mono term">{log}</pre>
		</div>
	</div>
</section>

<style>
	.head {
		display: flex;
		align-items: baseline;
		gap: var(--lead);
		margin-bottom: var(--lead);
	}

	.head h2 a {
		text-decoration: none;
	}

	.ok {
		color: var(--accent-ok);
	}

	.kv {
		display: grid;
		grid-template-columns: 12ch minmax(0, 1fr);
		gap: 0 var(--lead);
		margin: 0 0 var(--lead-2);
	}

	.kv dt,
	.kv dd {
		margin: 0;
		line-height: var(--lead);
	}

	.key {
		color: var(--text-muted);
	}

	.live {
		color: var(--accent);
	}

	.cols {
		display: grid;
		grid-template-columns: minmax(40ch, 1fr) minmax(0, 1.5fr);
		gap: var(--lead-2);
		align-items: start;
	}

	.crosshead {
		color: var(--accent);
		margin-bottom: var(--lead-half);
		box-shadow: 0 -1px 0 var(--border) inset;
	}

	.rows {
		list-style: none;
		margin: 0;
		padding: 0;
	}

	.row {
		display: grid;
		grid-template-columns: var(--lead) 10ch 1fr auto;
		align-items: baseline;
		gap: var(--lead-half);
		line-height: var(--lead);
		padding: var(--lead-half) 0;
		box-shadow: 0 -1px 0 var(--border) inset;
		text-decoration: none;
		color: var(--text);
	}

	.row:hover {
		background: var(--surface-quiet);
	}

	.state {
		text-align: center;
	}

	.state--succeeded {
		color: var(--accent-ok);
	}

	.state--failed {
		color: var(--accent-signal);
	}

	.state--running {
		color: var(--accent);
	}

	.timing {
		text-align: right;
	}

	/* The engine's terminal holds fixed night pigments across modes, like the build log. */
	.term {
		margin: 0;
		padding: var(--lead-half) var(--lead) var(--lead);
		border-radius: var(--radius-md);
		overflow-x: auto;
		line-height: var(--lead);
		background: var(--p9-night);
		color: var(--p9-line);
	}
</style>
