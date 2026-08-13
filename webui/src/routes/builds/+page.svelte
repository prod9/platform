<script>
	// ⚠ MOCK — canned data promoted from /preview; before the real implementation
	// locks in: graduate the design into docs/spec, wire the real reads, extract shared
	// components (outcome mark, feed row, kv list, terminal pane), delete canned data.
	// One repo's builds as a CI feed: newest first, each row led by its outcome, carrying
	// the tag, the commit it resolved to, per-module marks, and the trigger's provenance.
	import Button from "$lib/components/Button.svelte";

	const marks = { succeeded: "✓", failed: "✗", running: "◌", queued: "·" };

	const builds = [
		{
			id: 128,
			tag: "v0.9.36",
			sha: "8c0db6e",
			subject: "tests: Re-record the golden for the v0.9.36 launcher pin",
			status: "succeeded",
			modules: [{ name: "platform", status: "succeeded" }],
			trigger: "github-push",
			took: "4m 12s",
			when: "2h ago",
		},
		{
			id: 127,
			tag: "v0.9.35",
			sha: "e996f69",
			subject: "webui: Install page classifies its state read by the install signal",
			status: "failed",
			modules: [{ name: "platform", status: "failed" }],
			trigger: "webui · chakrit",
			took: "2m 40s",
			when: "1d ago",
		},
		{
			id: 126,
			tag: "",
			sha: "2f4c1d9",
			subject: "engine: Roster picks uniformly at random per dial",
			status: "succeeded",
			modules: [{ name: "platform", status: "succeeded" }],
			trigger: "webui · chakrit · refs/heads/main",
			took: "3m 45s",
			when: "6h ago",
		},
		{
			id: 125,
			tag: "v0.9.34",
			sha: "43a6928",
			subject: "srv: Installer replicas converge on the claim restart by re-probing",
			status: "running",
			modules: [{ name: "platform", status: "running" }],
			trigger: "github-push",
			took: "1m 03s",
			when: "4m ago",
		},
		{
			id: 121,
			tag: "v0.9.33",
			sha: "7f31c37",
			subject: "docs: Ban manual Dagger-engine touches — the SDK spawns its own",
			status: "queued",
			modules: [{ name: "platform", status: "queued" }],
			trigger: "retry · chakrit",
			took: "",
			when: "3d ago",
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
				<a class="row" href="/builds/127/">
					<span class="mono state state--{build.status}">{marks[build.status]}</span>

					<span class="what">
						<span class="line">
							<span class="mono tag">{build.tag || build.sha}</span>
							<span class="subject">{build.subject}</span>
						</span>
						<span class="mono muted meta">
							#{build.id}{build.tag ? ` · ${build.sha}` : ""} · {build.trigger}
						</span>
					</span>

					<span class="mods">
						{#each build.modules as unit (unit.name)}
							<span class="mono mod mod--{unit.status}">{marks[unit.status]} {unit.name}</span>
						{/each}
					</span>

					<span class="timing">
						<span class="mono">{build.took}</span>
						<span class="mono muted">{build.when}</span>
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
		grid-template-columns: var(--lead) minmax(32ch, 1fr) auto 10ch;
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

	.state {
		text-align: center;
	}

	.state--succeeded,
	.mod--succeeded {
		color: var(--accent-ok);
	}

	.state--failed,
	.mod--failed {
		color: var(--accent-signal);
	}

	.state--running,
	.mod--running {
		color: var(--accent);
	}

	.state--queued,
	.mod--queued {
		color: var(--text-muted);
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
		white-space: nowrap;
	}

	.timing {
		display: flex;
		flex-direction: column;
		text-align: right;
	}
</style>
