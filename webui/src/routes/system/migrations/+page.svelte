<script>
	import { onMount } from "svelte";
	import Button from "$lib/components/Button.svelte";
	import {
		Answered,
		classifyMigrationPlan,
		errorText,
		runSystemMigrations,
		systemMigrations,
	} from "$lib/server.js";

	let plan = $state([]);
	let phase = $state("loading");
	let failure = $state("");

	async function read() {
		phase = "loading";
		failure = "";
		const result = await systemMigrations();
		accept(result);
	}

	async function run() {
		phase = "running";
		failure = "";
		const result = await runSystemMigrations();
		accept(result);
	}

	function accept(result) {
		if (result.outcome === Answered) {
			plan = result.body;
			phase = "ready";
			return;
		}
		failure = errorText(result);
		phase = "failed";
	}

	onMount(read);

	let state = $derived(classifyMigrationPlan(plan));
</script>

<section>
	<div class="head">
		<h3>Migrations</h3>
		<span class="spacer"></span>
		{#if phase === "ready" && state === "runnable"}
			<Button variant="primary" onclick={run}>Run migrations</Button>
		{/if}
	</div>

	{#if phase === "loading"}
		<p class="muted">Reading migration plan…</p>
	{:else if phase === "running"}
		<p class="muted">Running migrations…</p>
	{:else if failure}
		<p class="failure mono">{failure}</p>
	{:else if state === "current"}
		<p>The schema is current.</p>
	{:else}
		<ul class="plan">
			{#each plan as item (`${item.action}:${item.migration}`)}
				<li class="line">
					<span class="mono action">{item.action}</span>
					<span class="mono">{item.migration}</span>
				</li>
			{/each}
		</ul>

		{#if state === "intervention_required"}
			<div class="warning">
				<p class="label">Manual recovery required</p>
				<p>
					Shell into the server and run
					<code>./platform srv data resync-migrations --force</code>, then refresh.
				</p>
			</div>
		{/if}
	{/if}
</section>

<style>
	.head {
		display: flex;
		align-items: baseline;
		gap: var(--lead);
		margin-bottom: var(--lead);
	}

	.spacer {
		margin-left: auto;
	}

	.plan {
		margin: 0 0 var(--lead);
		padding: 0;
		list-style: none;
	}

	.line {
		display: grid;
		grid-template-columns: 14ch minmax(0, 1fr);
		gap: var(--lead);
		line-height: var(--lead);
		box-shadow: 0 -1px 0 var(--border) inset;
	}

	.action {
		color: var(--accent);
	}

	.warning {
		padding: var(--lead-half) var(--lead);
		box-shadow: 0 -1px 0 var(--accent-signal) inset;
		background: var(--surface-quiet);
		color: var(--accent-signal-strong);
	}

	.warning p {
		margin: 0;
		line-height: var(--lead);
	}

	.failure {
		color: var(--accent-signal);
	}
</style>
