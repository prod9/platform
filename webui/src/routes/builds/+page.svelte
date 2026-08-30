<script>
	// ⚠ MOCK — canned data promoted from /preview; before the real implementation
	// locks in: graduate the design into docs/spec, wire the real reads, extract shared
	// components (outcome mark, feed row, kv list, terminal pane), delete canned data.
	// One repo's builds as a CI feed: newest first, each row led by its outcome, carrying
	// the exact ref and commit, per-module status, and the trigger's provenance.
	import Button from "$lib/components/Button.svelte";
	import OutcomeMark from "$lib/components/OutcomeMark.svelte";

	const builds = [
		{
			id: 128,
			ref: "refs/tags/v0.9.36",
			sha: "8c0db6e000000000000000000000000000000000",
			subject: "tests: Re-record the golden for the v0.9.36 launcher pin",
			status: "succeeded",
			modules: [{ name: "platform", status: "succeeded" }],
			trigger: "github-push",
			duration: "4m 12s",
			activity: { field: "finished_at", value: "2026-08-12T07:16:12Z" },
		},
		{
			id: 127,
			ref: "refs/tags/v0.9.35",
			sha: "e996f69000000000000000000000000000000000",
			subject: "webui: Install page classifies its state read by the install signal",
			status: "failed",
			modules: [{ name: "platform", status: "failed" }],
			trigger: "webui · chakrit",
			duration: "2m 40s",
			activity: { field: "finished_at", value: "2026-08-11T09:17:40Z" },
		},
		{
			id: 126,
			ref: "refs/heads/main",
			sha: "2f4c1d9000000000000000000000000000000000",
			subject: "engine: Roster picks uniformly at random per dial",
			status: "succeeded",
			modules: [{ name: "platform", status: "succeeded" }],
			trigger: "webui · chakrit · refs/heads/main",
			duration: "3m 45s",
			activity: { field: "finished_at", value: "2026-08-12T03:18:45Z" },
		},
		{
			id: 125,
			ref: "refs/tags/v0.9.34",
			sha: "43a6928000000000000000000000000000000000",
			subject: "srv: Installer replicas converge on the claim restart by re-probing",
			status: "running",
			modules: [{ name: "platform", status: "running" }],
			trigger: "github-push",
			duration: "1m 03s",
			activity: { field: "started_at", value: "2026-08-12T09:14:03Z" },
		},
		{
			id: 121,
			ref: "refs/tags/v0.9.33",
			sha: "7f31c37000000000000000000000000000000000",
			subject: "docs: Ban manual Dagger-engine touches — the SDK spawns its own",
			status: "queued",
			modules: [{ name: "platform", status: "queued" }],
			trigger: "retry · chakrit",
			duration: "",
			activity: { field: "created_at", value: "2026-08-09T09:14:03Z" },
		},
	];
</script>

<section>
	<div class="head">
		<h2><a href="/">Repositories</a> / platform</h2>
		<p class="label">prod9/platform</p>
		<span class="spacer"></span>
		<Button variant="primary" href="/builds/new/">New build</Button>
	</div>

	<ul class="rows">
		{#each builds as build (build.id)}
			<li>
				<a class="row" href={`/builds/${build.id}/`}>
					<OutcomeMark status={build.status} />

					<span class="what">
						<span class="line">
							<span class="mono tag">{build.ref}</span>
							<span class="subject">{build.subject}</span>
						</span>
						<span class="mono muted meta">
							#{build.id} · {build.sha} · {build.trigger}
						</span>
					</span>

					<span class="mods">
						{#each build.modules as unit (unit.name)}
							<span class="mod"><OutcomeMark status={unit.status} /> {unit.name}</span>
						{/each}
					</span>

					<span class="timing">
						<span class="mono">{build.activity.field}</span>
						<span class="mono muted">{build.activity.value}</span>
						{#if build.duration}<span class="mono muted">duration: {build.duration}</span>{/if}
					</span>
				</a>
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

	.head h2 a {
		text-decoration: none;
	}

	.spacer {
		margin-left: auto;
	}

	.rows {
		list-style: none;
		margin: 0;
		padding: 0;
	}

	.row {
		display: grid;
		grid-template-columns: minmax(var(--lead), max-content) minmax(32ch, 1fr) auto 18ch;
		align-items: center;
		gap: var(--lead);
		padding: var(--lead-half) 0;
		box-shadow: 0 -1px 0 var(--border) inset;
		text-decoration: none;
		color: var(--text);
	}

	.row:hover {
		background: var(--surface-quiet);
	}

	.what {
		display: flex;
		flex-direction: column;
		min-width: 0;
	}

	.line {
		display: flex;
		align-items: baseline;
		gap: var(--lead-half);
		min-width: 0;
	}

	.tag {
		font-size: var(--size-prose);
		font-weight: 600;
		color: var(--accent);
	}

	.subject {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		color: var(--text-muted);
	}

	.mods {
		display: flex;
		gap: var(--lead-half);
	}

	.mod {
		display: flex;
		align-items: baseline;
		gap: var(--lead-half);
		white-space: nowrap;
	}

	.timing {
		display: flex;
		flex-direction: column;
		text-align: right;
	}
</style>
