<script>
	import { onMount } from "svelte";
	import { Answered, errorText, systemSettings } from "$lib/server.js";

	let sections = $state([]);
	let loading = $state(true);
	let failure = $state("");

	async function load() {
		const result = await systemSettings();
		if (result.outcome === Answered) {
			sections = result.body;
		} else {
			failure = errorText(result);
		}
		loading = false;
	}

	onMount(load);
</script>

{#if loading}
	<p class="muted">Reading system settings…</p>
{:else if failure}
	<p class="failure mono">{failure}</p>
{:else}
	{#each sections as group (group.name)}
		<section class="group">
			<h3 class="crosshead">{group.name}</h3>
			<dl class="kv">
				{#each group.facts as fact (fact.key)}
					<dt class="mono key">{fact.key}</dt>
					<dd class="mono">{fact.value}</dd>
				{/each}
			</dl>
		</section>
	{/each}
{/if}

<style>
	.group {
		margin-bottom: var(--lead-2);
	}

	.crosshead {
		margin-bottom: var(--lead-half);
		box-shadow: 0 -1px 0 var(--border) inset;
		color: var(--accent);
	}

	.kv {
		display: grid;
		grid-template-columns: 22ch minmax(0, 1fr);
		gap: 0 var(--lead);
		margin: 0;
	}

	.kv dt,
	.kv dd {
		margin: 0;
		line-height: var(--lead);
	}

	.key {
		color: var(--text-muted);
	}

	.failure {
		color: var(--accent-signal);
	}
</style>
