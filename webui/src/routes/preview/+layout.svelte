<script>
	// The builder-UI walkthrough: every proposed view as a real page over canned data, so
	// the design is judged in the components and lattice it will actually ship in. Nothing
	// here talks to the server; delete the directory once the real pages land.
	import { page } from "$app/state";

	let { children } = $props();

	const views = [
		{ href: "/preview/repos/", label: "1 Repos" },
		{ href: "/preview/add-repo/", label: "2 Add repo" },
		{ href: "/preview/builds/", label: "3 Builds" },
		{ href: "/preview/new-build/", label: "4 New build" },
		{ href: "/preview/build/", label: "5 Build detail" },
		{ href: "/preview/engines/", label: "6 Engines" },
		{ href: "/preview/settings/", label: "7 Settings" },
	];

	function isCurrent(href) {
		return page.url.pathname === href;
	}
</script>

<nav class="strip">
	<a class="label" href="/preview/">Preview</a>
	{#each views as view (view.href)}
		<a class="label view" class:current={isCurrent(view.href)} href={view.href}>
			{view.label}
		</a>
	{/each}
</nav>

{@render children()}

<style>
	.strip {
		display: flex;
		gap: var(--lead);
		margin-bottom: var(--lead-2);
		box-shadow: 0 -1px 0 var(--border) inset;
	}

	.strip a {
		text-decoration: none;
	}

	.view {
		color: var(--text-muted);
	}

	.view:hover {
		color: var(--accent);
	}

	.view.current {
		color: var(--text);
	}
</style>
