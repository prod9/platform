<script>
	import { page } from "$app/state";

	let { children } = $props();

	const destinations = [
		{ href: "/system/settings/", label: "Settings" },
		{ href: "/system/migrations/", label: "Migrations" },
	];
</script>

<section class="system">
	<header class="head">
		<h2>System</h2>
		<nav aria-label="System">
			{#each destinations as destination (destination.href)}
				<a
					class="label"
					class:current={page.url.pathname.startsWith(destination.href)}
					href={destination.href}>{destination.label}</a
				>
			{/each}
		</nav>
	</header>

	{@render children()}
</section>

<style>
	.head {
		display: flex;
		align-items: baseline;
		gap: var(--lead);
		margin-bottom: var(--lead-2);
		box-shadow: 0 -1px 0 var(--border) inset;
	}

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
