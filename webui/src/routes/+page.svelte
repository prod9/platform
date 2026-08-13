<script>
	// The repos landing: one block per registered repo the session user can still
	// reach — the nested-feed shape, each block heading its last three builds and
	// linking into the repo's feed (docs/spec/webui.md). Signed out, the page is the
	// sign-in door and nothing else.
	import { session } from "$lib/session.svelte.js";
	import { listRepos, listRepoBuilds, errorText, Answered } from "$lib/server.js";
	import { latestStatus } from "$lib/repos.js";
	import { tagOf, shortSHA, lastActivity } from "$lib/build.js";
	import Button from "$lib/components/Button.svelte";
	import OutcomeMark from "$lib/components/OutcomeMark.svelte";

	const recentLimit = 3;

	let repos = $state([]);
	let loaded = $state(false);
	let loadError = $state("");

	async function load() {
		const result = await listRepos();
		if (result.outcome !== Answered) {
			loadError = errorText(result);
			loaded = true;
			return;
		}
		loadError = "";

		// The fan-out is per visible repo by design (docs/spec/webui.md §Route map);
		// a repo whose feed read fails renders with an empty feed rather than
		// blanking the page.
		repos = await Promise.all(
			result.body.map(async (repo) => {
				const feed = await listRepoBuilds(repo.owner, repo.repo, recentLimit);
				return {
					...repo,
					recent: feed.outcome === Answered ? feed.body : [],
				};
			}),
		);
		loaded = true;
	}

	$effect(() => {
		if (session.user === null) {
			return;
		}
		load();
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
	<p class="mono muted">Loading…</p>
{:else}
	<section>
		<div class="head">
			<h2>Repositories</h2>
			<p class="label">{repos.length} onboarded</p>
			<span class="spacer"></span>
			<Button variant="primary" href="/repos/new/">Add repository</Button>
		</div>

		{#if loadError}
			<p class="mono error">Repositories unavailable: {loadError}</p>
		{:else if repos.length === 0}
			<p class="mono muted">
				Nothing onboarded yet — add the first repository to start building.
			</p>
		{:else}
			<ul class="repos">
				{#each repos as repo (repo.full_name)}
					<li class="repo">
						<a class="repo-head" href={`/repos/${repo.owner}/${repo.repo}/`}>
							<OutcomeMark status={latestStatus(repo.recent)} />
							<span class="mono name">{repo.full_name}</span>
							<span class="mono chev">›</span>
						</a>

						<span class="subs">
							{#each repo.recent as build (build.id)}
								<a class="build" href={`/builds/${build.id}/`}>
									<OutcomeMark status={build.status} />
									<span class="mono tag">{tagOf(build.ref)}</span>
									<span class="mono muted">#{build.id} · {shortSHA(build.sha)}</span>
									<span class="mono muted timing">{lastActivity(build)}</span>
								</a>
							{:else}
								<span class="build none">
									<OutcomeMark status="none" />
									<span class="mono muted">no builds yet</span>
								</span>
							{/each}
						</span>
					</li>
				{/each}
			</ul>
		{/if}
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

	.spacer {
		margin-left: auto;
	}

	.error {
		color: var(--accent-signal);
	}

	.repos {
		list-style: none;
		margin: 0;
		padding: 0;
	}

	.repo {
		padding: var(--lead-half) 0;
		box-shadow: 0 -1px 0 var(--border) inset;
	}

	.repo-head {
		display: grid;
		grid-template-columns: var(--lead) 1fr var(--lead);
		align-items: baseline;
		gap: var(--lead-half);
		text-decoration: none;
		color: var(--text);
		line-height: var(--lead);
	}

	.repo-head:hover .name,
	.repo-head:hover .chev {
		color: var(--accent-signal);
	}

	/* The repo's name is the block's headline: prose-size, indigo, unmistakably not one
	   of the builds beneath it. */
	.name {
		font-size: var(--size-prose);
		font-weight: 600;
		color: var(--accent);
	}

	.chev {
		color: var(--text-muted);
		text-align: center;
	}

	/* Sub-rows hang off a vertical hairline, so the nesting reads before any row does. */
	.subs {
		display: block;
		margin-left: var(--lead-half);
		padding-left: var(--lead-half);
		box-shadow: 1px 0 0 var(--border) inset;
	}

	.build {
		display: grid;
		grid-template-columns: var(--lead) 10ch 1fr auto;
		align-items: baseline;
		gap: var(--lead-half);
		text-decoration: none;
		color: var(--text);
		line-height: var(--lead);
	}

	a.build:hover {
		background: var(--surface-quiet);
	}

	.timing {
		text-align: right;
	}
</style>
