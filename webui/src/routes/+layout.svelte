<script>
	import "$lib/styles/app.css";
	import { onMount } from "svelte";
	import { goto } from "$app/navigation";
	import { page } from "$app/state";
	import {
		installState,
		installSignal,
		errorText,
		Installing,
		Unknown,
	} from "$lib/server.js";
	import { session, loadSession, signOut } from "$lib/session.svelte.js";
	import { warm, loadTheme, toggleTheme } from "$lib/theme.svelte.js";
	import Button from "$lib/components/Button.svelte";

	let { children } = $props();

	// The shell is in exactly one of these, and each renders something different. Three
	// booleans said the same thing worse: two of their eight combinations were reachable.
	const Checking = "checking";
	const Unreachable = "unreachable";
	const Open = "open";

	let phase = $state(Checking);
	let unreachableReason = $state("");

	// The builder-UI nav; the cluster view joins it when that slice lands.
	const destinations = [
		{ href: "/", label: "Repositories" },
		{ href: "/engines/", label: "Engines" },
		{ href: "/system/settings/", label: "System" },
	];

	// Install is a gate, not a destination: it never appears in the nav, and the server
	// decides which side of it a visitor is on. GET /api/installation is served only while
	// the installation is unclaimed, so specifically a 404 is the installed signal — no
	// answer, or a server erroring on the probe, is neither state, and routing on it
	// would be a guess, so the shell stays shut and says why.
	async function gate() {
		const state = await installState();
		const signal = installSignal(state);
		if (signal === Unknown) {
			unreachableReason = errorText(state);
			phase = Unreachable;
			return;
		}

		await routeToSide(signal === Installing);

		// The org-owner claim needs a login before the server is installed, so the
		// session loads on both sides of the installation gate.
		await loadSession();
		phase = Open;
	}

	async function routeToSide(installing) {
		const path = page.url.pathname.replace(/\/+$/, "");
		const onInstall = path === "/installation";
		if (path === "/session") {
			return;
		}
		if (installing && !onInstall) {
			await goto("/installation/");
		} else if (!installing && onInstall) {
			await goto("/");
		}
	}

	// The repo drill-down — builds, wizards, build detail — lives under Repositories,
	// so the rail keeps it lit anywhere in that stack.
	function isCurrent(href) {
		const path = page.url.pathname;
		if (href === "/") {
			return path === "/" || path.startsWith("/repos") || path.startsWith("/builds");
		}
		if (href.startsWith("/system/")) {
			return path.startsWith("/system/");
		}
		return path.startsWith(href);
	}

	onMount(() => {
		loadTheme();
		gate();
	});
</script>

<div class="shell">
	<header class="rail">
		<a class="wordmark" href="/">PRODIGY9</a>

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
			<button class="toggle label" onclick={toggleTheme}>
				{warm.on ? "Too glum?" : "Too bright?"}
			</button>
			{#if session.user}
				<span class="mono">{session.user.name}</span>
				<Button onclick={signOut}>Sign out</Button>
			{/if}
		</div>
	</header>

	<main inert={phase !== Open} aria-busy={phase === Checking}>
		{#if phase === Unreachable}
			<p class="offline mono">{unreachableReason} Reload once it's back on :8210.</p>
		{:else}
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

	.wordmark:hover {
		color: var(--accent-signal);
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

	/* The current destination is stated by ink, not by a box around it. */
	.nav-link--current {
		color: var(--text);
	}

	.account {
		display: flex;
		align-items: center;
		column-gap: var(--lead-half);
		margin-left: auto;
	}

	.toggle {
		padding: var(--lead-half) 0;
		border: 0;
		background: none;
		color: var(--text-muted);
		cursor: pointer;
	}

	.toggle:hover {
		color: var(--accent-signal);
	}

	.offline {
		color: var(--accent-signal);
	}

	main {
		padding: var(--lead-2) var(--lead-2) var(--lead-4);
	}
</style>
