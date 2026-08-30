<script>
	// ⚠ MOCK — canned data promoted from /preview; before the real implementation
	// locks in: graduate the design into docs/spec, wire the real reads, extract shared
	// components (outcome mark, feed row, kv list, terminal pane), delete canned data.
	// This static path stands in for a dynamic per-engine route — when it graduates,
	// srv's fallback classifier must learn the new shape (it knows only /builds/{id};
	// docs/spec/platform-server.md, "The status of a page is the server's answer").
	// One engine instance: its facts, the builds it has carried, and its live log — the
	// same night-ground terminal the build detail uses.
	import Facts from "$lib/components/Facts.svelte";
	import OutcomeMark from "$lib/components/OutcomeMark.svelte";
	import PageHeader from "$lib/components/PageHeader.svelte";
	import SectionHeader from "$lib/components/SectionHeader.svelte";
	import SignalMark from "$lib/components/SignalMark.svelte";

	const recent = [
		{
			id: 125,
			repo: "prod9/infra",
			ref: "refs/tags/v0.3.12",
			status: "running",
			duration: "1m 03s",
			observed_at: "2026-08-12T09:15:06Z",
		},
		{
			id: 128,
			repo: "prod9/platform",
			ref: "refs/tags/v0.9.36",
			status: "succeeded",
			duration: "4m 12s",
			observed_at: "2026-08-12T07:16:12Z",
		},
		{
			id: 126,
			repo: "prod9/platform",
			ref: "refs/heads/main",
			status: "succeeded",
			duration: "3m 45s",
			observed_at: "2026-08-12T03:18:45Z",
		},
	];

	const log =
		"time=2026-08-12T09:14:03Z level=INFO msg=\"session opened\" client=platform-worker\n" +
		"time=2026-08-12T09:14:04Z level=INFO msg=\"solve start\" build=125 unit=infra\n" +
		"time=2026-08-12T09:14:11Z level=INFO msg=\"cache hit\" layers=41 ratio=0.87\n" +
		"time=2026-08-12T09:15:02Z level=INFO msg=\"exporting image\" ref=ghcr.io/prod9/infra:v0.3.12\n" +
		"time=2026-08-12T09:15:06Z level=WARN msg=\"gc pressure\" cache=41GB budget=48GB";
</script>

<section>
	<PageHeader>
		{#snippet title()}
			<h2><a href="/engines/">Engines</a> / 10.2.1.14</h2>
		{/snippet}
		{#snippet metadata()}
			<span class="label ok">reachable</span>
		{/snippet}
	</PageHeader>

	<div class="facts">
		<Facts measure="compact">
			<dt class="mono">address</dt>
			<dd class="mono">tcp://10.2.1.14:1234</dd>
			<dt class="mono">version</dt>
			<dd class="mono">dagger v0.18.5</dd>
			<dt class="mono">uptime</dt>
			<dd class="mono">6d 4h</dd>
			<dt class="mono">cache</dt>
			<dd class="mono">41 GB · 87% hit ratio</dd>
			<dt class="mono">now</dt>
			<dd class="mono live">
				<SignalMark signal="active" /> building prod9/infra #125 · v0.3.12
			</dd>
		</Facts>
	</div>

	<div class="cols">
		<div>
			<SectionHeader title="Recent builds on this engine" />
			<ul class="rows">
				{#each recent as build (build.id)}
					<li>
						<a class="row" href={`/builds/${build.id}/`}>
							<OutcomeMark status={build.status} />
							<span class="mono tag">{build.ref}</span>
							<span class="mono muted">{build.repo} · #{build.id}</span>
							<span class="mono muted timing">
								observed_at {build.observed_at} · duration {build.duration}
							</span>
						</a>
					</li>
				{/each}
			</ul>
		</div>

		<div>
			<SectionHeader title="Engine log" />
			<pre class="mono term">{log}</pre>
		</div>
	</div>
</section>

<style>
	.ok {
		color: var(--accent-ok);
	}

	.facts {
		margin-bottom: var(--lead-2);
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

	.rows {
		list-style: none;
		margin: 0;
		padding: 0;
	}

	.row {
		display: grid;
		grid-template-columns: minmax(var(--lead), max-content) minmax(16ch, auto) 1fr auto;
		align-items: baseline;
		column-gap: var(--lead-half);
		line-height: var(--lead);
		padding: var(--lead-half) 0;
		box-shadow: 0 -1px 0 var(--border) inset;
		text-decoration: none;
		color: var(--text);
	}

	.row:hover {
		background: var(--surface-quiet);
	}

	.timing {
		text-align: right;
	}

	/* The engine's terminal holds fixed night pigments across modes, like the build log. */
	.term {
		margin: 0;
		padding: var(--lead);
		border-radius: var(--radius-md);
		overflow-x: auto;
		line-height: var(--lead);
		background: var(--p9-night);
		color: var(--p9-line);
	}
</style>
