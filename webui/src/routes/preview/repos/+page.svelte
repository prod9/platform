<script>
	// The landing page: onboarded repos, each row folding its latest builds. A row opens
	// the repo's build list; add-repository closes the list and is the whole page on an
	// empty install.
	import StatusChip from "$lib/components/StatusChip.svelte";
	import Button from "$lib/components/Button.svelte";
	import Panel from "$lib/components/Panel.svelte";

	const repos = [
		{
			full: "prod9/platform",
			modules: "platform",
			last: "2h ago",
			recent: [
				{ id: 128, status: "succeeded" },
				{ id: 127, status: "failed" },
				{ id: 126, status: "succeeded" },
			],
		},
		{
			full: "prod9/infra",
			modules: "infra",
			last: "4m ago",
			recent: [
				{ id: 125, status: "running" },
				{ id: 121, status: "succeeded" },
			],
		},
		{ full: "prod9/fx", modules: "docs · site", last: "—", recent: [] },
	];
</script>

<section>
	<div class="head">
		<h2>Repositories</h2>
		<p class="label">{repos.length} onboarded</p>
	</div>

	<table>
		<thead>
			<tr>
				<th>Repository</th>
				<th>Modules</th>
				<th>Recent builds</th>
				<th>Last activity</th>
				<th></th>
			</tr>
		</thead>
		<tbody>
			{#each repos as repo (repo.full)}
				<tr>
					<td class="mono"><a href="/preview/builds/">{repo.full}</a></td>
					<td class="mono muted">{repo.modules}</td>
					<td>
						{#if repo.recent.length === 0}
							<span class="mono muted">no builds yet</span>
						{:else}
							<span class="recent">
								{#each repo.recent as build (build.id)}
									<a class="mono" href="/preview/build/">
										#{build.id} <StatusChip status={build.status} />
									</a>
								{/each}
							</span>
						{/if}
					</td>
					<td class="mono muted">{repo.last}</td>
					<td><a class="chev mono" href="/preview/builds/">›</a></td>
				</tr>
			{/each}
		</tbody>
	</table>

	<div class="add">
		<Panel label="Add a repository">
			<p class="muted">
				Onboard a repo the App doesn't build yet — pick it, preview its
				<span class="mono">platform.toml</span>, confirm.
			</p>
			<Button variant="primary" href="/preview/add-repo/">Add repository</Button>
		</Panel>
	</div>
</section>

<style>
	.head {
		display: flex;
		align-items: baseline;
		gap: var(--lead);
		margin-bottom: var(--lead);
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

	.recent {
		display: inline-flex;
		gap: var(--lead-half);
	}

	.recent a {
		text-decoration: none;
	}

	.chev {
		color: var(--text-muted);
		text-decoration: none;
	}

	.chev:hover {
		color: var(--accent);
	}

	.add {
		max-width: 62ch;
		margin-top: var(--lead-2);
	}
</style>
