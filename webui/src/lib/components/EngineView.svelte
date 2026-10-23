<script>
	import { onDestroy } from "svelte";
	import { session } from "$lib/session.svelte.js";
	import { EngineRead } from "$lib/engine-read.svelte.js";
	import Button from "./Button.svelte";
	import Facts from "./Facts.svelte";
	import PageHeader from "./PageHeader.svelte";
	import Panel from "./Panel.svelte";
	import LoadingBlock from "./LoadingBlock.svelte";
	import EngineEndpoint from "./EngineEndpoint.svelte";
	import EngineAssignments from "./EngineAssignments.svelte";

	let { read, title, scope } = $props();
	const model = new EngineRead(() => read());
	let state = $derived(model.state);
	let snapshot = $derived(state.snapshot);
	let authenticated = $derived(session.ready && session.user !== null);
	let initialized = false;

	$effect(() => {
		if (authenticated && !initialized) {
			initialized = true;
			model.refresh();
		}
	});
	onDestroy(() => model.dispose());
</script>

<section aria-busy={model.busy}>
	<PageHeader>
		{#snippet title()}
			<h2>{#if scope === "instance"}<a href="/engines/">Engines</a> / {/if}{title}</h2>
		{/snippet}
		{#snippet metadata()}
			{#if snapshot !== null}<span class="label">{snapshot.roster_state}</span>{/if}
		{/snippet}
		{#snippet actions()}
			<Button onclick={() => model.refresh()} disabled={!authenticated || model.busy}>
				{model.busy ? "Refreshing…" : "Refresh"}
			</Button>
		{/snippet}
	</PageHeader>
	<div class="stack">
		{#if state.error}
			<p class="error mono" role="alert">
				{snapshot === null ? "Engine view unavailable" : "Refresh failed; showing an earlier snapshot"}: {state.error}
			</p>
		{/if}
		{#if snapshot === null}
			{#if state.phase !== "unavailable"}<LoadingBlock measure="standard" />{/if}
		{:else}
			<Panel label="Roster source">
				<Facts measure="compact">
					<dt class="mono">DAGGER_ENGINE</dt><dd class="mono">{snapshot.source || "Unconfigured"}</dd>
					<dt class="mono">DAGGER_ENGINE_PORT</dt><dd class="mono">{snapshot.port}</dd>
					<dt class="mono">Observed</dt><dd class="mono">{snapshot.observed_at}</dd>
					<dt class="mono">Roster state</dt><dd class="mono">{snapshot.roster_state}</dd>
				</Facts>
				{#if snapshot.error}<p class="error mono">{snapshot.error}</p>{/if}
			</Panel>
			<p class="mono muted">
				Assignments are recorded work, not proof of current execution or engine idleness.
				An address identifies an endpoint; its history may span different engine processes.
			</p>
			{#if scope === "fleet"}
				{#each snapshot.engines as engine (engine.address)}
					<Panel label="Engine endpoint">
						<EngineEndpoint {engine} href={`/engines/${encodeURIComponent(engine.address)}/`} />
						<h3>Recorded unfinished assignments</h3>
						<EngineAssignments assignments={engine.assignments} />
					</Panel>
				{:else}
					<p class="mono muted">
						{#if snapshot.roster_state === "unconfigured"}
							No remote fleet configured; inspection does not start a local engine.
						{:else if snapshot.roster_state === "empty"}
							The configured roster resolved to no endpoints.
						{:else}
							No endpoint observations are available.
						{/if}
					</p>
				{/each}
			{:else}
				<Panel label="Engine endpoint"><EngineEndpoint engine={snapshot.engine} /></Panel>
				<Panel label="Recorded unfinished assignments">
					<EngineAssignments assignments={snapshot.engine.assignments} />
				</Panel>
				<Panel label="Latest 50 readable endpoint assignments">
					<EngineAssignments assignments={snapshot.history} />
				</Panel>
			{/if}
		{/if}
	</div>
</section>

<style>
	.stack { display: grid; gap: var(--space-6); min-width: 0; overflow-wrap: anywhere; }
	h2 { overflow-wrap: anywhere; }
	h3 { margin-top: var(--space-6); }
	p { margin: 0; }
	.error { color: var(--error-ink); white-space: pre-wrap; }
</style>
