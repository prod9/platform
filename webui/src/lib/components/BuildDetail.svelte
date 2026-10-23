<script>
	import { onDestroy } from "svelte";
	import { session } from "$lib/session.svelte.js";
	import { Answered, errorText, getBuild, listSteps } from "$lib/server.js";
	import { groupBuildSteps, recordedTime, ranFor, selectBuildStep } from "$lib/build.js";
	import Button from "./Button.svelte";
	import Facts from "./Facts.svelte";
	import LoadingBlock from "./LoadingBlock.svelte";
	import OutcomeMark from "./OutcomeMark.svelte";
	import ObservationFreshness from "./ObservationFreshness.svelte";
	import PageHeader from "./PageHeader.svelte";
	import Panel from "./Panel.svelte";
	import SignalMark from "./SignalMark.svelte";
	import TerminalPane from "./TerminalPane.svelte";

	let { id } = $props();

	const Loading = "loading";
	const Ready = "ready";
	const Refreshing = "refreshing";
	const Unavailable = "unavailable";

	let phase = $state(Loading);
	let snapshot = $state(null);
	let selected = $state(null);
	let failure = $state("");
	let lastReadAt = $state("");
	let active = true;
	let inFlight = false;

	let groups = $derived(snapshot === null ? [] : groupBuildSteps(snapshot.build, snapshot.steps));
	let current = $derived(selectBuildStep(groups, selected));
	let currentModule = $derived(
		current === null ? null : groups.find((module) => module.build_module_id === current.build_module_id),
	);
	let canRefresh = $derived(
		session.ready && session.user !== null && (phase === Ready || phase === Unavailable),
	);

	async function load() {
		if (inFlight || !active) {
			return;
		}
		inFlight = true;

		try {
			const results = await Promise.allSettled([getBuild(id), listSteps(id)]);
			if (!active) {
				return;
			}

			const rejected = results.filter((result) => result.status === "rejected");
			if (rejected.length !== 0) {
				const unexpected = rejected.some((result) =>
					!(result.reason instanceof TypeError || result.reason instanceof SyntaxError),
				);
				if (unexpected) {
					throw new AggregateError(rejected.map((result) => result.reason), "Build reads failed");
				}
				failure = rejected.map((result) => result.reason.message).join("\n");
				phase = Unavailable;
				return;
			}
			const [buildResult, stepsResult] = results.map((result) => result.value);
			const failedReads = [buildResult, stepsResult].filter((result) => result.outcome !== Answered);
			if (failedReads.length !== 0) {
				failure = failedReads.map(errorText).join("\n");
				phase = Unavailable;
				return;
			}

			const nextSnapshot = { build: buildResult.body, steps: stepsResult.body };
			const nextGroups = groupBuildSteps(nextSnapshot.build, nextSnapshot.steps);
			selected = selectBuildStep(nextGroups, selected);
			snapshot = nextSnapshot;
			lastReadAt = new Date().toISOString();
			failure = "";
			phase = Ready;
		} finally {
			inFlight = false;
		}
	}

	function refresh() {
		if (!canRefresh) {
			return;
		}

		phase = Refreshing;
		load();
	}

	$effect(() => {
		if (session.ready && session.user !== null) {
			load();
		}
	});

	onDestroy(() => {
		active = false;
	});
</script>

<section aria-busy={phase === Loading || phase === Refreshing}>
	<PageHeader>
		{#snippet title()}
			<h2>
				{#if snapshot === null}
					<a href="/">Repositories</a> / #{id}
				{:else}
					<a href={`/repos/${snapshot.build.owner}/${snapshot.build.repo}/`}>
						{snapshot.build.owner}/{snapshot.build.repo}</a> / #{snapshot.build.id}
				{/if}
			</h2>
		{/snippet}
		{#snippet metadata()}
			{#if snapshot === null && phase !== Unavailable}
				<LoadingBlock measure="compact" />
			{:else if snapshot === null}
				<span class="mono muted">Unavailable</span>
			{:else}
				<span class="outcome mono"><OutcomeMark status={snapshot.build.status} /> {snapshot.build.status}</span>
			{/if}
		{/snippet}
		{#snippet actions()}
			{#if snapshot !== null}
				<Button href={`/repos/${snapshot.build.owner}/${snapshot.build.repo}/builds/new/?retry_of=${snapshot.build.id}`} disabled={!canRefresh}>Retry</Button>
			{/if}
			<Button onclick={refresh} disabled={!canRefresh}>
				{phase === Refreshing ? "Refreshing…" : "Refresh"}
			</Button>
		{/snippet}
	</PageHeader>

	<div class="stack">
		{#if failure}
			<p class="error mono" role="alert">
				{snapshot === null ? "Build unavailable" : "Refresh failed; showing earlier states"}: {failure}
			</p>
		{/if}
		{#if lastReadAt !== ""}<p class="label mono">Last refreshed {lastReadAt}</p>{/if}

		<Panel label="Build facts">
			<Facts measure="compact">
				{#if snapshot === null}
					{#each ["Ref", "SHA", "Trigger", "User id", "Created"] as label (label)}
						<dt class="mono">{label}</dt>
						<dd>
							{#if phase === Unavailable}
								<span class="mono muted">Unavailable</span>
							{:else}
								<LoadingBlock measure="standard" />
							{/if}
						</dd>
					{/each}
				{:else}
					<dt class="mono">Ref</dt><dd class="mono">{snapshot.build.ref}</dd>
					<dt class="mono">SHA</dt><dd class="mono">{snapshot.build.sha}</dd>
					<dt class="mono">Trigger</dt><dd class="mono">{snapshot.build.trigger}</dd>
					<dt class="mono">User id</dt><dd class="mono">{snapshot.build.user_id}</dd>
					<dt class="mono">Created</dt><dd class="mono">{snapshot.build.created_at}</dd>
					{#if recordedTime(snapshot.build.started_at) !== null}
						<dt class="mono">Started</dt><dd class="mono">{snapshot.build.started_at}</dd>
					{/if}
					{#if recordedTime(snapshot.build.finished_at) !== null}
						<dt class="mono">Finished</dt><dd class="mono">{snapshot.build.finished_at}</dd>
					{/if}
					{#if ranFor(snapshot.build) !== ""}
						<dt class="mono">Duration</dt><dd class="mono">{ranFor(snapshot.build)}</dd>
					{/if}
					{#if snapshot.build.retry_of !== 0}
						<dt class="mono">Retry of</dt><dd class="mono"><a href={`/builds/${snapshot.build.retry_of}/`}>#{snapshot.build.retry_of}</a></dd>
					{/if}
				{/if}
			</Facts>
			{#if snapshot !== null && snapshot.build.error !== ""}<p class="error mono">{snapshot.build.error}</p>{/if}
		</Panel>

		<div class="detail">
			<div class="navigator">
				{#if snapshot === null}
					<Panel label="Modules">
						{#if phase === Unavailable}
							<p class="mono muted">Module results unavailable.</p>
						{:else}
							<LoadingBlock measure="standard" />
							<LoadingBlock /><LoadingBlock /><LoadingBlock />
						{/if}
					</Panel>
				{:else}
					<ul class="modules">
						{#each groups as module (module.build_module_id)}
							<li class="module">
								<div class="nav-row">
									<OutcomeMark status={module.status} />
									<span class="mono module-name">{module.name} · {module.status}</span>
									<span class="mono muted">{ranFor(module)}</span>
								</div>
								<ObservationFreshness observation={module.observation} />
								<Facts measure="compact">
									<dt class="mono">Module id</dt><dd class="mono">{module.build_module_id}</dd>
									{#if recordedTime(module.started_at) !== null}<dt class="mono">Started</dt><dd class="mono">{module.started_at}</dd>{/if}
									{#if recordedTime(module.finished_at) !== null}<dt class="mono">Finished</dt><dd class="mono">{module.finished_at}</dd>{/if}
									{#if module.claimed_by !== ""}<dt class="mono">Worker</dt><dd class="mono">{module.claimed_by}</dd>{/if}
									{#if recordedTime(module.claimed_at) !== null}<dt class="mono">Claimed</dt><dd class="mono">{module.claimed_at}</dd>{/if}
									{#if module.engine_host !== ""}<dt class="mono">Engine</dt><dd class="mono"><a href={`/engines/${encodeURIComponent(module.engine_host)}/`}>{module.engine_host}</a></dd>{/if}
									{#if recordedTime(module.engine_assigned_at) !== null}<dt class="mono">Assigned</dt><dd class="mono">{module.engine_assigned_at}</dd>{/if}
									{#if module.last_event_kind !== ""}<dt class="mono">Last event</dt><dd class="mono">{module.last_event_kind}</dd>{/if}
									{#if recordedTime(module.last_event_at) !== null}<dt class="mono">Event at</dt><dd class="mono">{module.last_event_at}</dd>{/if}
									{#if module.image !== ""}<dt class="mono">Image</dt><dd class="mono">{module.image}</dd>{/if}
									{#if module.hash !== ""}<dt class="mono">Hash</dt><dd class="mono">{module.hash}</dd>{/if}
								</Facts>
								{#if module.error !== ""}<p class="error mono">{module.error}</p>{/if}
								<ul class="steps">
									{#each module.steps as step (`${step.step}/${step.started_at}`)}
										<li>
											<button
												class="nav-row step mono"
												class:selected={current === step}
												aria-pressed={current === step}
												onclick={() => selected = step}
											>
												{#if step.status === "incomplete"}
													<SignalMark signal="pending" label="incomplete" />
												{:else}
													<OutcomeMark status={step.status} />
												{/if}
												<span>{step.step} · {step.status}</span>
												<span class="muted">{ranFor(step)}</span>
											</button>
										</li>
									{:else}
										<li class="muted mono">No recorded steps yet.</li>
									{/each}
								</ul>
							</li>
						{:else}
							<li class="muted mono">No recorded modules.</li>
						{/each}
					</ul>
				{/if}
			</div>
			<TerminalPane
				step={current}
				moduleName={currentModule === null ? "" : currentModule.name}
				availability={snapshot === null ? phase : Ready}
			/>
		</div>
	</div>
</section>

<style>
	.stack {
		display: grid;
		gap: var(--space-6);
		min-width: 0;
		overflow-wrap: anywhere;
	}

	.stack > p {
		margin: 0;
	}

	.detail {
		display: grid;
		grid-template-columns: minmax(34ch, 1fr) minmax(0, 2fr);
		gap: var(--space-6);
		align-items: start;
	}

	.navigator {
		min-width: 0;
	}

	.modules,
	.steps {
		list-style: none;
		margin: 0;
		padding: 0;
	}

	.module {
		display: grid;
		gap: var(--space-3);
		padding: var(--space-3) 0;
		box-shadow: 0 -1px 0 var(--hairline) inset;
	}

	.nav-row {
		display: grid;
		grid-template-columns: var(--space-6) minmax(0, 1fr) auto;
		align-items: baseline;
		column-gap: var(--space-3);
		width: 100%;
		line-height: var(--step-0-lh);
	}

	.module-name {
		font-family: var(--face-display);
		font-size: var(--step-0);
		font-weight: 800;
		color: var(--indigo);
	}

	.step {
		padding: var(--space-3) var(--space-1);
		border: 0;
		background: transparent;
		text-align: left;
		color: var(--muted);
		cursor: pointer;
	}

	.step:hover {
		background: var(--soft);
		color: var(--indigo);
	}

	.step:active {
		box-shadow: 0 3px 0 var(--hairline) inset;
	}

	.step.selected {
		background: var(--soft);
		color: var(--ink);
		box-shadow: 3px 0 0 var(--indigo) inset;
	}

	.outcome {
		display: inline-flex;
		align-items: baseline;
		gap: var(--space-3);
	}

	.error {
		margin: 0;
		color: var(--error-ink);
		white-space: pre-wrap;
	}

	@media (max-width: 64rem) {
		.detail {
			grid-template-columns: minmax(0, 1fr);
		}
	}
</style>
