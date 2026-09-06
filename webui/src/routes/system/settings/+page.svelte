<script>
	import { session } from "$lib/session.svelte.js";
	import { Answered, errorText, systemSettings } from "$lib/server.js";
	import Facts from "$lib/components/Facts.svelte";
	import LoadingBlock from "$lib/components/LoadingBlock.svelte";
	import SectionHeader from "$lib/components/SectionHeader.svelte";

	const pendingSections = [
		{ name: "Server", keys: ["public_url", "org"] },
		{
			name: "GitHub App",
			keys: ["app", "client_id", "private_key", "webhook_secret", "client_secret"],
		},
		{ name: "Registry", keys: ["ghcr.io token"] },
	];

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

	$effect(() => {
		if (session.ready) {
			load();
		}
	});

	let displayedSections = $derived(
		loading
			? pendingSections.map((group) => ({
					name: group.name,
					facts: group.keys.map((key) => ({
						key,
						value: "",
					})),
				}))
			: sections,
	);
</script>

<div aria-busy={loading}>
	{#if failure}
		<p class="failure mono">{failure}</p>
	{:else}
		{#each displayedSections as group (group.name)}
			<section class="group">
				<SectionHeader title={group.name} />
				<Facts>
					{#each group.facts as fact (fact.key)}
						<dt class="mono">{fact.key}</dt>
						<dd class="mono">
							{#if loading}
								<LoadingBlock measure="standard" />
							{:else}
								{fact.value}
							{/if}
						</dd>
					{/each}
				</Facts>
			</section>
		{/each}
	{/if}
</div>

<style>
	.group {
		margin-bottom: var(--lead-2);
	}

	.failure {
		color: var(--accent-signal);
	}
</style>
