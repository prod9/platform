<script>
	import { recordedTime } from "$lib/build.js";
	import Facts from "./Facts.svelte";
	import SignalMark from "./SignalMark.svelte";

	let { engine, href = "" } = $props();
	let signal = $derived(engine.state === "ready" ? "positive" :
		engine.state === "unreachable" ? "negative" : "pending");
</script>

<div class="endpoint">
	<div class="heading mono">
		<SignalMark {signal} />
		{#if href}<a {href}>{engine.address}</a>{:else}<strong>{engine.address}</strong>{/if}
		<span>{engine.state}</span>
	</div>
	<Facts measure="compact">
		<dt class="mono">Version</dt><dd class="mono">{engine.version || "Unavailable"}</dd>
		{#if recordedTime(engine.observed_at) !== null}
			<dt class="mono">Observed</dt><dd class="mono">{engine.observed_at}</dd>
		{/if}
	</Facts>
	{#if engine.error}<p class="error mono">{engine.error}</p>{/if}
	{#if engine.state === "absent"}
		<p class="mono muted">Historical endpoint absent from the current roster; no probe was made.</p>
	{:else if engine.state === "unknown"}
		<p class="mono muted">Current membership is unknown because discovery failed; no historical address was probed.</p>
	{:else if engine.state === "local"}
		<p class="mono muted">SDK-managed execution on the recorded worker; no remote endpoint to inspect.</p>
	{:else if engine.state === "version_unavailable"}
		<p class="mono muted">Connection succeeded; the engine version could not be read.</p>
	{:else if engine.state === "unreachable"}
		<p class="mono muted">The connection attempt failed at the recorded observation time.</p>
	{/if}
</div>

<style>
	.endpoint { display: grid; gap: var(--space-3); min-width: 0; overflow-wrap: anywhere; }
	.heading { display: flex; align-items: baseline; flex-wrap: wrap; gap: var(--space-3); }
	.heading a, strong { font-weight: 800; }
	p { margin: 0; }
	.error { color: var(--error-ink); }
</style>
