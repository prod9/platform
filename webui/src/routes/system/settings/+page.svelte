<script>
	import { onMount } from "svelte";
	import { Answered, errorText, systemSettings } from "$lib/server.js";
	import Facts from "$lib/components/Facts.svelte";
	import SectionHeader from "$lib/components/SectionHeader.svelte";

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
			<SectionHeader title={group.name} />
			<Facts>
				{#each group.facts as fact (fact.key)}
					<dt class="mono">{fact.key}</dt>
					<dd class="mono">{fact.value}</dd>
				{/each}
			</Facts>
		</section>
	{/each}
{/if}

<style>
	.group {
		margin-bottom: var(--lead-2);
	}

	.failure {
		color: var(--accent-signal);
	}
</style>
