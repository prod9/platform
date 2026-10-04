<script>
	import { session } from "$lib/session.svelte.js";
	import { Answered, createBuild, errorText, listRepoBuilds } from "$lib/server.js";
	import { recordedTime } from "$lib/build.js";
	import Button from "./Button.svelte";
	import Facts from "./Facts.svelte";
	import LoadingBlock from "./LoadingBlock.svelte";
	import OutcomeMark from "./OutcomeMark.svelte";
	import PageHeader from "./PageHeader.svelte";
	import Panel from "./Panel.svelte";

	let { owner, repo } = $props();

	const Loading = "loading";
	const Ready = "ready";
	const Refreshing = "refreshing";
	const Starting = "starting";
	const Unavailable = "unavailable";
	const recentLimit = 3;

	let phase = $state(Loading);
	let builds = $state([]);
	let ref = $state("");
	let readError = $state("");
	let startError = $state("");
	let acceptedID = $state(0);
	let lastReadAt = $state("");

	let canInteract = $derived(
		phase === Ready && session.ready && session.user !== null,
	);
	let canStart = $derived(canInteract && ref.trim() !== "");
	let canRefresh = $derived(
		session.ready && session.user !== null && (phase === Ready || phase === Unavailable),
	);

	async function load() {
		try {
			const result = await listRepoBuilds(owner, repo, recentLimit);
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
			const result = await createBuild(owner, repo, ref.trim());
			if (result.outcome !== Answered) {
				startError = errorText(result);
				return;
			}

			acceptedID = result.body.id;
			builds = [result.body, ...builds.filter((build) => build.id !== acceptedID)]
				.sort((left, right) => right.id - left.id)
				.slice(0, recentLimit);
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
				<p class="mono" role="status">Build #{acceptedID} accepted.</p>
			{/if}
		</Panel>

		{#if readError}
			<p class="error mono" role="alert">
				{lastReadAt === "" ? "Builds unavailable" : "Refresh failed; showing earlier states"}:
				{readError}
			</p>
		{/if}

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
			{#each builds as build (build.id)}
				<Panel label={`Build #${build.id}`}>
					<div class="outcome mono"><OutcomeMark status={build.status} /> {build.status}</div>
					<Facts measure="compact">
						<dt class="mono">Ref</dt><dd class="mono">{build.ref}</dd>
						<dt class="mono">SHA</dt><dd class="mono">{build.sha}</dd>
						<dt class="mono">Trigger</dt><dd class="mono">{build.trigger}</dd>
						<dt class="mono">Created</dt><dd class="mono">{build.created_at}</dd>
						{#if recordedTime(build.started_at) !== null}
							<dt class="mono">Started</dt><dd class="mono">{build.started_at}</dd>
						{/if}
						{#if recordedTime(build.finished_at) !== null}
							<dt class="mono">Finished</dt><dd class="mono">{build.finished_at}</dd>
						{/if}
					</Facts>
					{#if build.error}<p class="error mono">{build.error}</p>{/if}
					<ul class="modules">
						{#each build.modules as module (module.build_module_id)}
							<li>
								<div class="outcome mono">
									<OutcomeMark status={module.status} /> {module.name} · {module.status}
								</div>
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
				</Panel>
			{/each}
		{/if}
	</div>
</section>

<style>
	.stack {
		display: grid;
		gap: var(--lead);
		min-width: 0;
		overflow-wrap: anywhere;
	}

	form {
		display: flex;
		align-items: center;
		flex-wrap: wrap;
		row-gap: var(--lead);
		column-gap: var(--lead-half);
	}

	input {
		flex: 1;
		min-width: 16ch;
		padding: calc(var(--lead-half) - var(--plate-edge)) var(--lead-half);
		border: var(--plate-edge) solid var(--border);
		border-radius: var(--radius-sm);
		background: var(--surface-raised);
		line-height: var(--lead);
		color: var(--text);
	}

	input:disabled {
		color: var(--text-muted);
	}

	.outcome {
		display: flex;
		align-items: baseline;
		gap: var(--lead-half);
	}

	.modules {
		list-style: none;
		margin: 0;
		padding: 0;
		display: grid;
		gap: var(--lead);
	}

	.error {
		color: var(--accent-signal);
		white-space: pre-wrap;
	}
</style>
