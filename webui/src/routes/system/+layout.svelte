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
		{#snippet navigation()}
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
		flex-wrap: wrap;
		gap: var(--space-2) var(--space-6);
	}

	a {
		padding-block: var(--space-3);
		color: var(--muted);
		text-decoration: none;
	}

	a:hover, a.current {
		color: var(--indigo);
	}

	a.current {
		box-shadow: inset 0 -3px 0 var(--indigo);
	}
</style>
