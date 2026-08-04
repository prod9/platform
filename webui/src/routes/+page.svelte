<script>
	// The build list: the server's whole point made visible. A row's outcome is folded from
	// its events on every read — no status is stored anywhere.
	import { goto } from "$app/navigation";
	import { listBuilds, listRepos, createBuild, errorText, Answered } from "$lib/server.js";
	import { tagOf, shortSHA, lastActivity, ranFor } from "$lib/build.js";
	import { session } from "$lib/session.svelte.js";
	import StatusChip from "$lib/components/StatusChip.svelte";
	import Button from "$lib/components/Button.svelte";
	import Panel from "$lib/components/Panel.svelte";

	let builds = $state([]);
	let loaded = $state(false);

	// The manual trigger: pick a repo the installation reaches, name a ref, and the
	// controller records the same domain fact a webhook would — resolved server-side.
	let repos = $state([]);
	let repoFull = $state("");
	let ref = $state("");
	let repoError = $state("");
	let queueing = $state(false);
	let triggerError = $state("");

	async function loadRepos() {
		const result = await listRepos();
		if (result.outcome === Answered) {
			repos = result.body;
			repoError = "";
		} else {
			repoError = errorText(result);
		}
	}

	async function queueBuild(event) {
		event.preventDefault();
		queueing = true;
		triggerError = "";

		try {
			const repo = repos.find((entry) => entry.full_name === repoFull);
			if (repo === undefined) {
				triggerError = `${repoFull} is no longer in the repo list.`;
				return;
			}

			const result = await createBuild(repo.owner, repo.name, ref);
			if (result.outcome === Answered) {
				await goto(`/builds/${result.body.id}/`);
			} else {
				triggerError = errorText(result);
			}
		} finally {
			queueing = false;
		}
	}

	// A running build advances in the database rather than in this tab, so the list
	// re-reads while anything is live and rests when nothing is.
	const pollMs = 5000;
	const liveStatuses = ["queued", "running"];

	async function load() {
		const result = await listBuilds();
		if (result.outcome === Answered) {
			builds = result.body;
		}
		loaded = true;
	}

	function anyLive() {
		return builds.some((build) => liveStatuses.includes(build.status));
	}

	$effect(() => {
		if (session.user === null) {
			return;
		}

		load();
		loadRepos();
		const timer = setInterval(() => {
			if (anyLive()) {
				load();
			}
		}, pollMs);
		return () => clearInterval(timer);
	});
</script>

{#if session.user === null}
	<section class="door">
		<h1>platform</h1>
		<p>
			Tag-triggered builds for the repos on this cluster. Sign in with the GitHub
			account that pushes them.
		</p>
		<Button variant="primary" href="/auth/github">Sign in with GitHub</Button>
	</section>
{:else if !loaded}
	<p class="muted">Loading…</p>
{:else}
	<form class="trigger" onsubmit={queueBuild}>
		<select class="mono" bind:value={repoFull} required>
			<option value="" disabled>Repository…</option>
			{#each repos as repo (repo.full_name)}
				<option value={repo.full_name}>{repo.full_name}</option>
			{/each}
		</select>
		<input
			class="mono"
			type="text"
			bind:value={ref}
			placeholder="refs/tags/v1.2.3"
			required
		/>
		<Button variant="primary" disabled={queueing}>
			{queueing ? "Queueing…" : "Build"}
		</Button>
		{#if repoError}
			<span class="error mono">Repos unavailable: {repoError}</span>
		{/if}
		{#if triggerError}
			<span class="error mono">{triggerError}</span>
		{/if}
	</form>

	{#if builds.length === 0}
		<Panel label="No builds yet">
			<p class="muted">Push a version tag on an installed repo and it queues one.</p>
		</Panel>
	{:else}
		<section>
			<div class="head">
				<h2>Builds</h2>
				<p class="label">Latest 50, newest first</p>
			</div>

			<table>
				<thead>
					<tr>
						<th>Build</th>
						<th>Repository</th>
						<th>Tag</th>
						<th>Commit</th>
						<th>Status</th>
						<th>Took</th>
						<th>When</th>
					</tr>
				</thead>
				<tbody>
					{#each builds as build (build.id)}
						<tr>
							<td class="mono"><a href="/builds/{build.id}/">#{build.id}</a></td>
							<td class="mono">{build.owner}/{build.repo}</td>
							<td class="mono">{tagOf(build.ref)}</td>
							<td class="mono muted">{shortSHA(build.sha)}</td>
							<td>
								<StatusChip status={build.status} />
								{#if build.error}
									<span class="error mono" title={build.error}>{build.error}</span>
								{/if}
							</td>
							<td class="mono muted">{ranFor(build)}</td>
							<td class="mono muted">{lastActivity(build)}</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</section>
	{/if}
{/if}

<style>
	.door {
		max-width: 62ch;
		padding: var(--lead-3) 0;
	}

	.door h1 {
		margin-bottom: var(--lead);
	}

	.head {
		display: flex;
		align-items: baseline;
		gap: var(--lead);
		margin-bottom: var(--lead);
	}

	.trigger {
		display: flex;
		align-items: center;
		gap: var(--lead-half);
		margin-bottom: var(--lead-2);
	}

	.trigger select,
	.trigger input {
		padding: 0 var(--lead-half);
		border: 1px solid var(--border);
		border-radius: var(--radius-sm);
		background: var(--surface-raised);
		line-height: var(--lead);
		color: var(--text);
	}

	.trigger input {
		width: 24ch;
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

	.error {
		display: block;
		max-width: 48ch;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
		color: var(--text-muted);
	}
</style>
