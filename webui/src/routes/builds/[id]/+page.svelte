<script>
	// The build detail: the record, its attempts folded, and the steps with their captured
	// output. Everything shown is derived from the event stream on every read — nothing is
	// stored, so a running build advances by re-reading.
	import { goto } from "$app/navigation";
	import { page } from "$app/state";
	import {
		getBuild,
		listSteps,
		createBuild,
		errorText,
		Answered,
		Refused,
	} from "$lib/server.js";
	import { tagOf, shortSHA, lastActivity, ranFor, byAttempt } from "$lib/build.js";
	import { session } from "$lib/session.svelte.js";
	import StatusChip from "$lib/components/StatusChip.svelte";
	import Button from "$lib/components/Button.svelte";
	import Panel from "$lib/components/Panel.svelte";

	let build = $state(null);
	let steps = $state([]);
	let loaded = $state(false);
	let missing = $state(false);
	let stepsError = $state("");
	let retrying = $state(false);
	let retryError = $state("");

	const pollMs = 5000;
	const liveStatuses = ["queued", "running"];

	let id = $derived(page.params.id);
	let attempts = $derived(byAttempt(steps));

	async function load() {
		const [detail, stepsRead] = await Promise.all([getBuild(id), listSteps(id)]);
		if (detail.outcome === Answered) {
			build = detail.body;
		} else if (detail.outcome === Refused && detail.status === 404) {
			missing = true;
		}
		if (stepsRead.outcome === Answered) {
			steps = stepsRead.body;
			stepsError = "";
		} else {
			stepsError = errorText(stepsRead);
		}
		loaded = true;
	}

	function isLive() {
		return build !== null && liveStatuses.includes(build.status);
	}

	// A retry is just a new build: the same repo and ref, re-resolved, recorded through
	// the manual trigger — the superseded build simply runs out (spec §Build lifecycle).
	async function retry() {
		retrying = true;
		retryError = "";

		try {
			const result = await createBuild(build.owner, build.repo, build.ref);
			if (result.outcome === Answered) {
				await goto(`/builds/${result.body.id}/`);
				build = null;
				steps = [];
				missing = false;
				loaded = false;
				await load();
			} else {
				retryError = errorText(result);
			}
		} finally {
			retrying = false;
		}
	}

	$effect(() => {
		if (session.user === null) {
			return;
		}

		load();
		const timer = setInterval(() => {
			if (isLive()) {
				load();
			}
		}, pollMs);
		return () => clearInterval(timer);
	});
</script>

{#if session.user === null}
	<p class="muted">Sign in to see this build.</p>
{:else if !loaded}
	<p class="muted">Loading…</p>
{:else if missing}
	<Panel label="No such build">
		<p class="muted">Nothing here answers to #{id}. <a href="/">Back to the list.</a></p>
	</Panel>
{:else if build !== null}
	<section>
		<div class="head">
			<h2>Build #{build.id}</h2>
			<StatusChip status={build.status} />
			<span class="spacer"></span>
			<Button onclick={retry} disabled={retrying}>
				{retrying ? "Queueing…" : "Retry"}
			</Button>
		</div>

		{#if retryError}
			<p class="failed mono">{retryError}</p>
		{/if}

		<dl class="facts">
			<dt class="label">Repository</dt>
			<dd class="mono">{build.owner}/{build.repo}</dd>
			<dt class="label">Tag</dt>
			<dd class="mono">{tagOf(build.ref)} <span class="muted">({build.ref})</span></dd>
			<dt class="label">Commit</dt>
			<dd class="mono">{shortSHA(build.sha)}</dd>
			<dt class="label">Trigger</dt>
			<dd class="mono">{build.trigger}</dd>
			{#if build.image}
				<dt class="label">Image</dt>
				<dd class="mono">{build.image}</dd>
			{/if}
			{#if build.error}
				<dt class="label">Error</dt>
				<dd class="mono failed">{build.error}</dd>
			{/if}
			<dt class="label">Took</dt>
			<dd class="mono">{ranFor(build)}</dd>
			<dt class="label">When</dt>
			<dd class="mono">{lastActivity(build)}</dd>
		</dl>

		{#if stepsError}
			<p class="failed mono">Steps unavailable: {stepsError}</p>
		{/if}

		{#if steps.length === 0}
			<Panel label="No steps yet">
				<p class="muted">The worker reports each step as it runs it.</p>
			</Panel>
		{:else}
			{#each attempts as attemptSteps, ordinal (ordinal)}
				<div class="attempt">
					{#if attempts.length > 1}
						<h3 class="label">Attempt {ordinal + 1}</h3>
					{/if}
					{#each attemptSteps as step (`${step.attempt}/${step.unit}/${step.step}`)}
						<details class="step" open={step.error !== ""}>
							<summary>
								<span class="mono">{step.unit} / {step.step}</span>
								<span class="mono muted">{ranFor(step)}</span>
								{#if step.error}
									<span class="failed mono">{step.error}</span>
								{/if}
							</summary>
							{#if step.stdout}
								<pre class="mono">{step.stdout}</pre>
							{/if}
							{#if step.stderr}
								<pre class="mono stderr">{step.stderr}</pre>
							{/if}
							{#if !step.stdout && !step.stderr}
								<p class="muted">No output captured.</p>
							{/if}
						</details>
					{/each}
				</div>
			{/each}
		{/if}
	</section>
{/if}

<style>
	section {
		max-width: 100ch;
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

	.facts {
		display: grid;
		grid-template-columns: 12ch minmax(0, 1fr);
		gap: 0 var(--lead);
		margin: 0 0 var(--lead-2);
	}

	.facts dt,
	.facts dd {
		margin: 0;
		line-height: var(--lead);
	}

	.attempt {
		margin-bottom: var(--lead-2);
	}

	.attempt h3 {
		margin-bottom: var(--lead-half);
	}

	.step {
		box-shadow: 0 -1px 0 var(--border) inset;
	}

	.step summary {
		display: flex;
		gap: var(--lead);
		line-height: var(--lead);
		cursor: pointer;
	}

	.step pre {
		margin: 0 0 var(--lead-half);
		padding: var(--lead-half) var(--lead);
		overflow-x: auto;
		background: var(--surface-raised);
		border-radius: var(--radius-md);
	}

	.stderr {
		color: var(--text-muted);
	}

	.failed {
		color: var(--accent-signal);
	}
</style>
