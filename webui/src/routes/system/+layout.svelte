<script>
	import { page } from "$app/state";
	import PageHeader from "$lib/components/PageHeader.svelte";

	let { children } = $props();

	const destinations = [
		{ href: "/system/settings/", label: "Settings" },
		{ href: "/system/migrations/", label: "Migrations" },
	];
</script>

<section class="system">
	<PageHeader>
		{#snippet title()}<h2>System</h2>{/snippet}
		{#snippet metadata()}
			<nav aria-label="System">
				{#each destinations as destination (destination.href)}
					<a
						class="label"
						class:current={page.url.pathname.startsWith(destination.href)}
						href={destination.href}>{destination.label}</a
					>
				{/each}
			</nav>
		{/snippet}
	</PageHeader>

	{@render children()}
</section>

<style>
	nav {
		display: flex;
		gap: var(--lead);
	}

	a {
		color: var(--text-muted);
		line-height: var(--lead-2);
		text-decoration: none;
	}

	a:hover,
	a.current {
		color: var(--accent);
	}
</style>
