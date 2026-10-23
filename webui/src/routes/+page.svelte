<script>
	// The repos landing: one block per registered repo the session user can still
	// reach — the nested-feed shape, each block heading its last three builds and
	// linking into the repo's feed (docs/spec/webui.md). Signed out, the page is the
	// sign-in door and nothing else.
	import { beginSignIn, session } from "$lib/session.svelte.js";
	import { listRepos, listRepoBuilds, errorText, Answered } from "$lib/server.js";
	import { latestStatus } from "$lib/repos.js";
	import { lastActivity } from "$lib/build.js";
	import Button from "$lib/components/Button.svelte";
	import FeedRow from "$lib/components/FeedRow.svelte";
	import LoadingBlock from "$lib/components/LoadingBlock.svelte";
	import OutcomeMark from "$lib/components/OutcomeMark.svelte";
	import PageHeader from "$lib/components/PageHeader.svelte";
	import SignalMark from "$lib/components/SignalMark.svelte";

	const recentLimit = 3;
	const pendingRepos = Array.from({ length: 2 });
	const pendingBuilds = Array.from({ length: recentLimit });

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
		if (!session.ready || session.user === null) {
			return;
		}
		load();
	});
</script>

{#if session.ready && session.user === null}
	<section class="door">
		<h1>platform</h1>
		<p>
			Tag-triggered builds for the repos on this cluster. Sign in with the GitHub
			account that pushes them.
		</p>
		<Button
			variant="primary"
			href="/auth/github"
			onclick={beginSignIn}
			disabled={session.signingIn}
		>
			{session.signingIn ? "Signing in…" : "Sign in with GitHub"}
		</Button>
	</section>
{:else}
	<section aria-busy={!loaded}>
		<PageHeader>
			{#snippet title()}<h2>Repositories</h2>{/snippet}
			{#snippet metadata()}
				<p class="label">
					{#if loaded}
						{repos.length} registered
					{:else}
						<LoadingBlock measure="compact" />
					{/if}
				</p>
			{/snippet}
			{#snippet actions()}
				<Button variant="primary" href="/repos/new/">Register repository</Button>
			{/snippet}
		</PageHeader>

		{#if !loaded}
			<ul class="repos" aria-hidden="true">
				{#each pendingRepos as _}
					<li class="repo">
						<button class="repo-head" disabled aria-label="Loading repository">
							<span class="pending-mark"><LoadingBlock /></span>
							<span class="mono name"><LoadingBlock measure="standard" /></span>
							<SignalMark signal="navigation" />
						</button>
						<div class="subs">
							{#each pendingBuilds as _}
								<FeedRow>
									{#snippet mark()}<LoadingBlock />{/snippet}
									{#snippet subject()}<LoadingBlock measure="compact" />{/snippet}
									{#snippet facts()}<LoadingBlock measure="standard" />{/snippet}
									{#snippet timing()}<LoadingBlock measure="compact" />{/snippet}
								</FeedRow>
							{/each}
						</div>
					</li>
				{/each}
			</ul>
		{:else if loadError}
			<p class="mono error">Repositories unavailable: {loadError}</p>
		{:else if repos.length === 0}
			<p class="mono muted">
				Nothing registered yet — register the first repository to start building.
			</p>
		{:else}
			<ul class="repos">
				{#each repos as repo (repo.full_name)}
					<li class="repo">
						<a class="repo-head" href={`/repos/${repo.owner}/${repo.repo}/`}>
							<OutcomeMark status={latestStatus(repo.recent)} />
							<span class="mono name">{repo.full_name}</span>
							<SignalMark signal="navigation" />
						</a>

						<div class="subs">
							{#each repo.recent as build (build.id)}
								{@const activity = lastActivity(build)}
								<FeedRow href={`/builds/${build.id}/`}>
									{#snippet mark()}<OutcomeMark status={build.status} />{/snippet}
									{#snippet subject()}{build.ref}{/snippet}
									{#snippet facts()}#{build.id} · {build.sha}{/snippet}
									{#snippet timing()}{activity || "no timestamp"}{/snippet}
								</FeedRow>
							{:else}
								<FeedRow>
									{#snippet mark()}<OutcomeMark status="none" />{/snippet}
									{#snippet subject()}<span class="muted">no builds yet</span>{/snippet}
								</FeedRow>
							{/each}
						</div>
					</li>
				{/each}
			</ul>
		{/if}
	</section>
{/if}

<style>
	.door {
		max-width: 62ch;
		padding-block: var(--space-12);
	}

	.door h1 {
		margin-bottom: var(--space-6);
	}

	.error {
		color: var(--error-ink);
		overflow-wrap: anywhere;
	}

	.repos {
		list-style: none;
		margin: 0;
		padding: 0;
	}

	.repo {
		padding-block: var(--space-6);
		box-shadow: inset 0 -1px 0 var(--hairline);
	}

	.repo-head {
		display: grid;
		grid-template-columns: var(--space-6) minmax(0, 1fr) var(--space-6);
		align-items: baseline;
		gap: var(--space-3);
		padding-block: var(--space-3);
		color: var(--ink);
		text-decoration: none;
	}

	a.repo-head:hover {
		--navigation-mark: var(--red);
	}

	a.repo-head:hover .name {
		color: var(--error-ink);
	}

	.name {
		font-family: var(--face-display);
		font-size: var(--step-1);
		font-weight: 800;
		line-height: var(--step-1-lh);
		color: var(--indigo);
		overflow-wrap: anywhere;
	}

	.subs {
		display: block;
		margin-left: var(--space-3);
		padding-left: var(--space-3);
		box-shadow: inset 1px 0 0 var(--hairline);
	}

	button.repo-head {
		width: 100%;
		padding-inline: 0;
		border: 0;
		background: none;
		font: inherit;
		text-align: left;
		cursor: default;
	}

	.pending-mark {
		width: var(--space-6);
	}

</style>
