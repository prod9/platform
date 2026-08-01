<script>
	import "$lib/styles/app.css";
	import { onMount } from "svelte";
	import { goto } from "$app/navigation";
	import { page } from "$app/state";
	import { installState } from "$lib/api.js";
	import { session, loadSession, signOut } from "$lib/session.svelte.js";
	import Button from "$lib/components/Button.svelte";

	let { children } = $props();

	let ready = $state(false);
	let warm = $state(false);

	// Install is a gate, not a destination: it never appears in the nav, and the server
	// decides which side of it we are on. GET /api/install is served only while the
	// installer fragment is mounted, so its absence is the installed signal.
	async function gate() {
		const onInstall = page.url.pathname.replace(/\/+$/, "") === "/install";
		const installed = (await installState()) === null;

		if (installed && onInstall) {
			await goto("/");
		} else if (!installed && !onInstall) {
			await goto("/install/");
		}

		if (installed) {
			await loadSession();
		}
		ready = true;
	}

	function toggleWarm() {
		warm = !warm;
		document.documentElement.dataset.theme = warm ? "warm" : "";
	}

	// One destination today; the cluster view joins it when that slice lands.
	const destinations = [{ href: "/", label: "Builds" }];

	function isCurrent(href) {
		return page.url.pathname === href;
	}

	onMount(gate);
</script>

<div class="shell">
	<header class="rail">
		<a class="wordmark" href="/">PRODIGY9</a>
		<span class="label tag">platform</span>

		{#if session.user}
			<nav>
				{#each destinations as destination (destination.href)}
					<a
						class="nav-link label"
						class:nav-link--current={isCurrent(destination.href)}
						href={destination.href}>{destination.label}</a
					>
				{/each}
			</nav>
		{/if}

		<div class="account">
			<button class="toggle label" onclick={toggleWarm}>
				{warm ? "Too glum?" : "Too bright?"}
			</button>
			{#if session.user}
				<span class="mono">{session.user.name}</span>
				<Button onclick={signOut}>Log out</Button>
			{/if}
		</div>
	</header>

	<main>
		{#if ready}
			{@render children()}
		{/if}
	</main>
</div>

<style>
	.shell {
		min-height: 100vh;
		background: var(--surface);
	}

	.rail {
		display: flex;
		align-items: center;
		gap: var(--lead);
		padding: var(--lead-half) var(--lead-2);
		box-shadow: 0 -1px 0 var(--border) inset;
	}

	.wordmark {
		font-family: var(--p9-display);
		font-size: var(--size-prose);
		font-weight: 700;
		letter-spacing: 0.06em;
		color: var(--accent);
		text-decoration: none;
	}

	.tag {
		color: var(--text-muted);
	}

	nav {
		display: flex;
		gap: var(--lead);
	}

	.nav-link {
		color: var(--text-muted);
		text-decoration: none;
	}

	.nav-link:hover {
		color: var(--accent);
	}

	/* The current destination is stated by weight and ink, not by a box around it. */
	.nav-link--current {
		color: var(--text);
	}

	.account {
		display: flex;
		align-items: center;
		gap: var(--lead-half);
		margin-left: auto;
	}

	.toggle {
		padding: 0;
		border: 0;
		background: none;
		color: var(--text-muted);
		cursor: pointer;
	}

	.toggle:hover {
		color: var(--accent-signal);
	}

	main {
		padding: var(--lead-2) var(--lead-2) var(--lead-4);
	}
</style>
