<script>
	import { session } from "$lib/session.svelte.js";
	import Button from "$lib/components/Button.svelte";
	import LoadingBlock from "$lib/components/LoadingBlock.svelte";
	import SectionHeader from "$lib/components/SectionHeader.svelte";
	import {
		Answered,
		classifyMigrationPlan,
		errorText,
		runSystemMigrations,
		systemMigrations,
	} from "$lib/server.js";

	const loadingPlan = Array.from({ length: 3 });

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

	$effect(() => {
		if (session.ready) {
			read();
		}
	});

	let state = $derived(classifyMigrationPlan(plan));
</script>

<section aria-busy={phase === "loading" || phase === "running"}>
	<SectionHeader title="Migrations">
		{#snippet actions()}
			{#if phase === "loading"}
				<Button disabled>Run migrations</Button>
			{:else if state === "runnable"}
				<Button variant="primary" onclick={run} disabled={phase === "running"}>
					{phase === "running" ? "Running…" : "Run migrations"}
				</Button>
			{/if}
		{/snippet}
	</SectionHeader>

	{#if phase === "loading"}
		<ul class="plan">
			{#each loadingPlan as _, index (index)}
				<li class="line">
					<LoadingBlock measure="compact" />
					<LoadingBlock measure="standard" />
				</li>
			{/each}
		</ul>
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
