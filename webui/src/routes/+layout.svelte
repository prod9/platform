<script>
	import "$lib/styles/app.css";
	import { onMount } from "svelte";
	import { goto } from "$app/navigation";
	import { page } from "$app/state";
	import { installState, unreachable } from "$lib/api.svelte.js";
	import { session, loadSession, signOut } from "$lib/session.svelte.js";
	import Button from "$lib/components/Button.svelte";

	let { children } = $props();

	let ready = $state(false);
	let warm = $state(false);

	// Install is a gate, not a destination: it never appears in the nav, and the server
	// decides which side of it we are on. GET /api/install is served only while the
	// installer fragment is mounted, so its absence is the installed signal — but only
	// when the server answered at all. An unreachable server is neither state, so the
	// gate stands down and the shell says so rather than routing on a guess.
	async function gate() {
		const onInstall = page.url.pathname.replace(/\/+$/, "") === "/install";
		const state = await installState();

		if (unreachable.hit) {
			ready = true;
			return;
		}

		const installed = state === null;
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

		<nav>
			{#each destinations as destination (destination.href)}
				<a
					class="nav-link label"
					class:nav-link--current={isCurrent(destination.href)}
					href={destination.href}>{destination.label}</a
				>
			{/each}
		</nav>

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

	{#if unreachable.hit}
		<p class="offline mono">
			No answer from the platform server. Start it on :8210, or the pages below will
			stay empty.
		</p>
	{/if}

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

	.offline {
		margin: 0;
		padding: 0 var(--lead-2);
		line-height: var(--lead-2);
		color: var(--accent-signal);
		box-shadow: 0 -1px 0 var(--border) inset;
	}

	main {
		padding: var(--lead-2) var(--lead-2) var(--lead-4);
	}
</style>
