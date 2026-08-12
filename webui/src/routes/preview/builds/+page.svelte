<script>
	// One repo's builds, newest first, with the per-module outcome folded inline. New
	// build opens the wizard; a row opens the build detail.
	import StatusChip from "$lib/components/StatusChip.svelte";
	import Button from "$lib/components/Button.svelte";

	const marks = { succeeded: "✓", failed: "✗", running: "◌", queued: "·" };

	const builds = [
		{
			id: 128,
			tag: "v0.9.36",
			sha: "8c0db6e",
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
			status: "failed",
			modules: [{ name: "platform", status: "failed" }],
			trigger: "webui · chakrit",
			took: "2m 40s",
			when: "1d ago",
		},
		{
			id: 125,
			tag: "v0.9.34",
			sha: "43a6928",
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
			status: "queued",
			modules: [{ name: "platform", status: "queued" }],
			trigger: "retry · chakrit",
			took: "—",
			when: "3d ago",
		},
	];
</script>

<section>
	<div class="head">
		<h2><a href="/preview/repos/">Repositories</a> / platform</h2>
		<p class="label">prod9/platform</p>
		<span class="spacer"></span>
		<Button variant="primary" href="/preview/new-build/">New build</Button>
	</div>

	<table>
		<thead>
			<tr>
				<th>Build</th>
				<th>Tag</th>
				<th>Commit</th>
				<th>Status</th>
				<th>Modules</th>
				<th>Trigger</th>
				<th>Took</th>
				<th>When</th>
			</tr>
		</thead>
		<tbody>
			{#each builds as build (build.id)}
				<tr>
					<td class="mono"><a href="/preview/build/">#{build.id}</a></td>
					<td class="mono">{build.tag}</td>
					<td class="mono muted">{build.sha}</td>
					<td><StatusChip status={build.status} /></td>
					<td>
						<span class="mods">
							{#each build.modules as unit (unit.name)}
								<span class="mono mod mod--{unit.status}">
									{marks[unit.status]} {unit.name}
								</span>
							{/each}
						</span>
					</td>
					<td class="mono muted">{build.trigger}</td>
					<td class="mono muted">{build.took}</td>
					<td class="mono muted">{build.when}</td>
				</tr>
			{/each}
		</tbody>
	</table>
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

	table {
		width: 100%;
		border-collapse: collapse;
	}

	th {
		font-family: var(--p9-support);
		font-size: var(--size-label);
		font-weight: 600;
		line-height: var(--lead);
		letter-spacing: 0.12em;
		text-transform: uppercase;
		color: var(--text-muted);
		text-align: left;
		padding: 0 var(--lead) 0 0;
		box-shadow: 0 -1px 0 var(--border) inset;
	}

	td {
		padding: 0 var(--lead) 0 0;
		line-height: var(--lead);
		box-shadow: 0 -1px 0 var(--border) inset;
		vertical-align: top;
	}

	.mods {
		display: inline-flex;
		gap: var(--lead-half);
	}

	.mod {
		white-space: nowrap;
	}

	.mod--failed {
		color: var(--accent-signal);
	}

	.mod--running {
		color: var(--accent);
	}

	.mod--queued {
		color: var(--text-muted);
	}
</style>
