<script>
	// The build list: the server's whole point made visible. A row's outcome is folded from
	// its events on every read — no status is stored anywhere.
	import { listBuilds, Answered } from "$lib/server.js";
	import { tagOf, shortSHA, lastActivity, ranFor } from "$lib/build.js";
	import { session } from "$lib/session.svelte.js";
	import StatusChip from "$lib/components/StatusChip.svelte";
	import Button from "$lib/components/Button.svelte";
	import Panel from "$lib/components/Panel.svelte";

	let builds = $state([]);
	let loaded = $state(false);

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
{:else if builds.length === 0}
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
