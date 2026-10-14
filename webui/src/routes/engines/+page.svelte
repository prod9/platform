<script>
	// ⚠ MOCK — canned data promoted from /preview; before the real implementation
	// locks in: graduate the design into docs/spec, wire the real reads, extract shared
	// components (outcome mark, feed row, kv list, terminal pane), delete canned data.
	// The engine fleet as the same nested feed the repo list uses: one block per resolved
	// instance — reachability leading the header, its facts and current work as sub-rows.
	// The seed line states where the roster comes from; the roster itself is DNS, read
	// per load.
	import Button from "$lib/components/Button.svelte";
	import Facts from "$lib/components/Facts.svelte";
	import OutcomeMark from "$lib/components/OutcomeMark.svelte";
	import PageHeader from "$lib/components/PageHeader.svelte";
	import Panel from "$lib/components/Panel.svelte";
	import SignalMark from "$lib/components/SignalMark.svelte";

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
	<PageHeader>
		{#snippet title()}
			<h2>Engines</h2>
		{/snippet}
		{#snippet metadata()}
			<p class="label">{engines.length} resolved</p>
		{/snippet}
		{#snippet actions()}
			<Button>Refresh</Button>
		{/snippet}
	</PageHeader>

	<div class="seed">
		<Panel label="Roster source">
			<Facts>
				<dt class="mono">DAGGER_ENGINE</dt>
				<dd class="mono">dagger-engine.platform.svc</dd>
				<dt class="mono">DAGGER_ENGINE_PORT</dt>
				<dd class="mono">1234</dd>
			</Facts>
		</Panel>
	</div>

	<ul class="fleet">
		{#each engines as engine (engine.addr)}
			<li class="engine">
				<a class="engine-head" href="/engines/instance/">
					<SignalMark signal={engine.ok ? "positive" : "negative"} />
					<span class="mono addr">{engine.addr}</span>
					<span class="label reach" class:ok={engine.ok} class:warn={!engine.ok}>
						{engine.ok ? "reachable" : "unreachable"}
					</span>
					<SignalMark signal="navigation" />
				</a>

				<span class="subs">
					{#if !engine.ok}
						<span class="sub">
							<OutcomeMark status="failed" />
							<span class="mono warn">did not answer the dial</span>
							<span class="mono muted timing">
								last_seen_at 2026-08-12T08:34:00Z
							</span>
						</span>
					{:else}
						<span class="sub">
							<span class="mono state"></span>
							<span class="mono muted">
								{engine.version} · {engine.uptime} · {engine.cache}
							</span>
						</span>
						{#if engine.work}
							<a class="sub" href={`/builds/${engine.work.build}/`}>
								<OutcomeMark status="running" />
								<span class="mono live">
									building {engine.work.repo} #{engine.work.build} · {engine.work.tag}
								</span>
								<span class="mono muted timing">duration {engine.work.took}</span>
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
	.seed {
		margin-bottom: var(--space-8);
	}

	.fleet {
		list-style: none;
		margin: 0;
		padding: 0;
	}

	.engine {
		padding-block: var(--space-6);
		box-shadow: inset 0 -1px 0 var(--hairline);
	}

	.engine-head {
		display: grid;
		grid-template-columns: var(--space-6) minmax(0, 1fr) auto var(--space-6);
		align-items: baseline;
		gap: var(--space-3);
		padding-block: var(--space-3);
		color: var(--ink);
		text-decoration: none;
	}

	.engine-head:hover {
		--navigation-mark: var(--red);
	}

	.engine-head:hover .addr {
		color: var(--error-ink);
	}

	.addr {
		font-size: var(--step-0);
		font-weight: 700;
		color: var(--indigo);
		overflow-wrap: anywhere;
	}

	.reach.ok {
		color: var(--ink);
	}

	.subs {
		display: block;
		margin-left: var(--space-3);
		padding-left: var(--space-3);
		box-shadow: inset 1px 0 0 var(--hairline);
	}

	.sub {
		display: grid;
		grid-template-columns: var(--space-6) minmax(0, 1fr) minmax(0, auto);
		align-items: baseline;
		gap: var(--space-2) var(--space-3);
		padding-block: var(--space-2);
		color: var(--ink);
		text-decoration: none;
	}

	.sub > span {
		min-width: 0;
		overflow-wrap: anywhere;
	}

	a.sub:hover {
		background: var(--soft);
	}

	.state {
		text-align: center;
	}

	.live {
		color: var(--indigo);
	}

	.warn {
		color: var(--error-ink);
	}

	.timing {
		text-align: right;
	}

	@media (max-width: 48rem) {
		.engine-head {
			grid-template-columns: var(--space-6) minmax(0, 1fr) var(--space-6);
		}

		.reach {
			grid-column: 2;
			grid-row: 2;
		}

		.sub {
			grid-template-columns: var(--space-6) minmax(0, 1fr);
		}

		.timing {
			grid-column: 2;
			text-align: left;
		}
	}
</style>
