<script>
	import { session } from "$lib/session.svelte.js";
	import { Answered, createBuild, errorText, listRepoBuilds } from "$lib/server.js";
	import { recordedTime } from "$lib/build.js";
	import { acceptBuild, HistoryLimit } from "$lib/repos.js";
	import Button from "./Button.svelte";
	import Facts from "./Facts.svelte";
	import FeedRow from "./FeedRow.svelte";
	import LoadingBlock from "./LoadingBlock.svelte";
	import OutcomeMark from "./OutcomeMark.svelte";
	import ObservationFreshness from "./ObservationFreshness.svelte";
	import PageHeader from "./PageHeader.svelte";
	import Panel from "./Panel.svelte";

	let { owner, repo } = $props();

	const Loading = "loading";
	const Ready = "ready";
	const Refreshing = "refreshing";
	const Starting = "starting";
	const Unavailable = "unavailable";

	let phase = $state(Loading);
	let builds = $state([]);
	let ref = $state("");
	let readError = $state("");
	let startError = $state("");
	let acceptedID = $state(0);
	let lastReadAt = $state("");
	let readInFlight = false;

	let canInteract = $derived(
		phase === Ready && session.ready && session.user !== null,
	);
	let canStart = $derived(canInteract && ref.trim() !== "");
	let canRefresh = $derived(
		session.ready && session.user !== null && (phase === Ready || phase === Unavailable),
	);

	async function load() {
		if (readInFlight) {
			return;
		}
		readInFlight = true;

		try {
			const result = await listRepoBuilds(owner, repo, HistoryLimit);
			if (result.outcome !== Answered) {
				readError = errorText(result);
				phase = Unavailable;
				return;
			}

			builds = result.body;
			readError = "";
			lastReadAt = new Date().toISOString();
			phase = Ready;
		} catch (error) {
			if (!(error instanceof TypeError || error instanceof SyntaxError)) {
				throw error;
			}

			readError = error.message;
			phase = Unavailable;
		} finally {
			readInFlight = false;
		}
	}

	function refresh() {
		if (!canRefresh) {
			return;
		}

		phase = Refreshing;
		load();
	}

	async function start(event) {
		event.preventDefault();
		if (!canStart) {
			return;
		}

		phase = Starting;
		startError = "";
		acceptedID = 0;
		try {
			const result = await createBuild({ owner, repo, ref: ref.trim() });
			if (result.outcome !== Answered) {
				startError = errorText(result);
				return;
			}

			acceptedID = result.body.id;
			builds = acceptBuild(builds, result.body);
		} catch (error) {
			if (!(error instanceof TypeError || error instanceof SyntaxError)) {
				throw error;
			}

			startError = error.message;
		} finally {
			phase = Ready;
		}
	}

	$effect(() => {
		if (session.ready && session.user !== null) {
			load();
		}
	});
</script>

<section aria-busy={phase === Loading || phase === Refreshing}>
	<PageHeader>
		{#snippet title()}<h2><a href="/">Repositories</a> / {owner}/{repo}</h2>{/snippet}
		{#snippet actions()}
			<Button href={`/repos/${owner}/${repo}/builds/new/`} disabled={!canInteract}>New build</Button>
			<Button onclick={refresh} disabled={!canRefresh}>
				{phase === Refreshing ? "Refreshing…" : "Refresh"}
			</Button>
		{/snippet}
	</PageHeader>

	<div class="stack">
		<Panel label="Start build">
			<form onsubmit={start}>
				<label for="build-ref" class="mono">Ref</label>
				<input
					id="build-ref"
					class="mono"
					type="text"
					bind:value={ref}
					placeholder="refs/heads/main"
					required
					disabled={!canInteract}
				/>
				<Button variant="primary" disabled={!canStart}>
					{phase === Starting ? "Starting…" : "Start build"}
				</Button>
			</form>
			<p class="muted">Builds every module at the selected ref.</p>
			{#if startError}
				<p class="error mono" role="alert">Build not confirmed: {startError}</p>
			{:else if acceptedID !== 0}
				<p class="mono" role="status">Build <a href={`/builds/${acceptedID}/`}>#{acceptedID}</a> accepted.</p>
			{/if}
		</Panel>

		{#if readError}
			<p class="error mono" role="alert">
				{lastReadAt === "" ? "Builds unavailable" : "Refresh failed; showing earlier states"}:
				{readError}
			</p>
		{/if}

		<p class="label mono">Latest {HistoryLimit} builds · newest first</p>
		{#if lastReadAt !== ""}
			<p class="label mono">Last refreshed {lastReadAt}</p>
		{/if}

		{#if phase === Loading}
			<Panel label="Recent builds">
				<LoadingBlock measure="compact" />
				<Facts measure="compact">
					{#each ["Ref", "SHA", "Trigger", "Created"] as label (label)}
						<dt class="mono">{label}</dt>
						<dd><LoadingBlock measure="standard" /></dd>
					{/each}
				</Facts>
				<LoadingBlock measure="standard" />
			</Panel>
		{:else if builds.length === 0 && lastReadAt !== ""}
			<p class="mono muted">No builds yet.</p>
		{:else}
			<ul class="history">
				{#each builds as build (build.id)}
					<li class="build">
						<FeedRow href={`/builds/${build.id}/`}>
							{#snippet mark()}<OutcomeMark status={build.status} />{/snippet}
							{#snippet subject()}
								<span>#{build.id} · {build.status}</span>
								<span class="value">Ref {build.ref}</span>
							{/snippet}
							{#snippet facts()}
								<span>SHA {build.sha}</span>
								<span class="value">Trigger {build.trigger} · user_id {build.user_id}</span>
							{/snippet}
							{#snippet timing()}
								<span>Created {build.created_at}</span>
								{#if recordedTime(build.started_at) !== null}
									<span class="value">Started {build.started_at}</span>
								{/if}
								{#if recordedTime(build.finished_at) !== null}
									<span class="value">Finished {build.finished_at}</span>
								{/if}
							{/snippet}
						</FeedRow>
						{#if build.error}<p class="error mono">{build.error}</p>{/if}
						<ul class="modules">
							{#each build.modules as module (module.build_module_id)}
								<li>
									<div class="outcome mono">
										<OutcomeMark status={module.status} /> {module.name} · {module.status}
									</div>
									<ObservationFreshness observation={module.observation} />
									{#if module.error}<p class="error mono">{module.error}</p>{/if}
									{#if module.image || module.hash}
										<Facts measure="compact">
											{#if module.image}<dt class="mono">Image</dt><dd class="mono">{module.image}</dd>{/if}
											{#if module.hash}<dt class="mono">Hash</dt><dd class="mono">{module.hash}</dd>{/if}
										</Facts>
									{/if}
								</li>
							{/each}
						</ul>
					</li>
				{/each}
			</ul>
		{/if}
	</div>
</section>

<style>
	.stack {
		display: grid;
		gap: var(--space-6);
		min-width: 0;
		overflow-wrap: anywhere;
	}

	form {
		display: flex;
		align-items: center;
		flex-wrap: wrap;
		row-gap: var(--space-6);
		column-gap: var(--space-3);
	}

	input {
		flex: 1;
		min-width: min(16ch, 100%);
	}

	.outcome {
		display: flex;
		align-items: baseline;
		gap: var(--space-3);
	}

	.modules {
		list-style: none;
		margin: 0;
		padding: 0;
		display: grid;
		gap: var(--space-3);
		margin-left: var(--space-3);
		padding-left: var(--space-6);
		box-shadow: inset 1px 0 0 var(--hairline);
	}

	.history {
		list-style: none;
		margin: 0;
		padding: 0;
		min-width: 0;
	}

	.build {
		padding-block: var(--space-3);
		box-shadow: inset 0 -1px 0 var(--hairline);
	}

	.value {
		display: block;
	}

	.error {
		color: var(--error-ink);
		white-space: pre-wrap;
	}
</style>
