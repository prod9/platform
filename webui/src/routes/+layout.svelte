<script>
	import "../p9.css";
	import { onMount } from "svelte";
	import { goto } from "$app/navigation";
	import { page } from "$app/state";

	let { children } = $props();

	// install gate: GET /api/install is present (200) only while the installer fragment
	// is mounted; once completely installed the fragment is gone and the request 404s on
	// the static fallback. So 404 = installed → app; 200 = not installed → /install.
	let checking = $state(true);

	async function gate() {
		const resp = await fetch("/api/install");
		const installed = resp.status === 404;
		const onInstall = page.url.pathname.replace(/\/+$/, "") === "/install";

		if (installed && onInstall) {
			await goto("/");
		} else if (!installed && !onInstall) {
			await goto("/install");
		}

		checking = false;
	}

	onMount(gate);
</script>

<div class="p9-shell">
	<header class="p9-rail">
		<span class="p9-wordmark">PRODIGY9</span>
		<span class="p9-tag">platform</span>
	</header>

	<main>
		{#if checking}
			<section class="p9-section">
				<p class="p9-section__note">Loading…</p>
			</section>
		{:else}
			{@render children()}
		{/if}
	</main>
</div>
