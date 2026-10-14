<script>
	import { recordedTime } from "$lib/build.js";
	import Facts from "./Facts.svelte";
	import LoadingBlock from "./LoadingBlock.svelte";
	import OutcomeMark from "./OutcomeMark.svelte";
	import SignalMark from "./SignalMark.svelte";

	let { step = null, moduleName = "", availability = "ready" } = $props();
	let loading = $derived(availability === "loading" || availability === "refreshing");
</script>

<section class="terminal">
	<div class="heading">
		<h3>{step === null ? "Step output" : `${moduleName} · ${step.step}`}</h3>
		{#if loading}
			<LoadingBlock measure="compact" />
		{:else if step !== null}
			<span class="outcome mono">
				{#if step.status === "incomplete"}
					<SignalMark signal="pending" label="incomplete" />
				{:else}
					<OutcomeMark status={step.status} />
				{/if}
				{step.status}
			</span>
		{/if}
	</div>
	{#if loading}
		<div class="placeholder"><LoadingBlock /><LoadingBlock /><LoadingBlock /></div>
	{:else if availability === "unavailable"}
		<p class="empty muted">Step output unavailable.</p>
	{:else if step === null}
		<p class="empty muted">No recorded steps yet.</p>
	{:else}
		<div class="facts">
			<Facts measure="compact">
				{#if recordedTime(step.started_at) !== null}
					<dt class="mono">Started</dt><dd class="mono">{step.started_at}</dd>
				{/if}
				{#if recordedTime(step.finished_at) !== null}
					<dt class="mono">Finished</dt><dd class="mono">{step.finished_at}</dd>
				{/if}
			</Facts>
		</div>
		{#if step.error !== ""}<p class="error mono">{step.error}</p>{/if}
		<div class="stream">
			<h3 class="label">stdout</h3>
			{#if step.stdout !== ""}
				<pre>{step.stdout}</pre>
			{:else}
				<p class="empty muted">No captured stdout.</p>
			{/if}
		</div>
		<div class="stream">
			<h3 class="label">stderr</h3>
			{#if step.stderr !== ""}
				<pre>{step.stderr}</pre>
			{:else}
				<p class="empty muted">No captured stderr.</p>
			{/if}
		</div>
	{/if}
</section>

<style>
	.terminal {
		min-width: 0;
		background: var(--white);
		box-shadow: 0 3px 0 var(--slab-edge);
	}

	.heading {
		--signal-positive: var(--paper);
		--signal-negative: var(--paper);
		--signal-active: var(--paper);
		--signal-pending: var(--paper);
		--signal-navigation: var(--paper);
		display: flex;
		align-items: baseline;
		flex-wrap: wrap;
		gap: var(--space-3) var(--space-6);
		padding: var(--space-6);
		background: var(--night);
		color: var(--paper);
	}

	.heading h3 {
		min-width: 0;
		flex: 1;
		color: var(--paper);
		overflow-wrap: anywhere;
	}

	.outcome {
		display: inline-flex;
		align-items: baseline;
		gap: var(--space-3);
	}

	.stream h3 {
		padding: var(--space-3) var(--space-6);
		box-shadow: 0 1px 0 var(--hairline) inset;
	}

	pre {
		max-width: 100%;
		margin: 0;
		padding: var(--space-6);
		overflow-x: auto;
		font-family: var(--face-figures);
		font-size: var(--step--1);
		line-height: var(--step-0-lh);
		background: var(--night);
		color: var(--paper);
		tab-size: 4;
	}

	.empty,
	.error,
	.facts,
	.placeholder {
		margin: 0;
		padding: var(--space-6);
	}

	.placeholder {
		display: grid;
		gap: var(--space-3);
	}

	.error {
		color: var(--error-ink);
		white-space: pre-wrap;
		overflow-wrap: anywhere;
	}
</style>
