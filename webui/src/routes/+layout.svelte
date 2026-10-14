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
		gate();
	});
</script>

<div class="shell">
	<header class="rail">
		<a class="wordmark" href="/"><img src="/logo/wordmark-red.png" alt="PRODIGY9" /></a>

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
 .shell { min-height: 100dvh; background: var(--paper); }
 .rail {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: var(--space-6) var(--space-12);
  min-height: var(--space-18);
  padding: var(--space-4) var(--space-12);
  background: var(--white);
  box-shadow: inset 0 -1px 0 var(--hairline), 0 3px 0 color-mix(in srgb, var(--hairline), var(--night) 12%);
 }
 .wordmark { display: block; flex: 0 0 auto; line-height: 0; }
 .wordmark img { display: block; width: 10rem; height: auto; }
 nav { display: flex; flex-wrap: wrap; gap: var(--space-2) var(--space-6); }
 .nav-link {
  padding-block: var(--space-2);
  font-family: var(--face-wide);
  font-weight: 700;
  color: var(--muted);
  text-decoration: none;
 }
 .nav-link:hover { color: var(--indigo); }
 .nav-link--current { color: var(--indigo); box-shadow: inset 0 -3px 0 var(--indigo); }
 .account {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: var(--space-3);
  min-width: 0;
  margin-left: auto;
 }
 .account span { overflow-wrap: anywhere; }
 .offline { color: var(--error-ink); overflow-wrap: anywhere; }
 main { min-width: 0; padding: var(--space-8) var(--space-12) var(--space-24); }
 @media (max-width: 48rem) {
  .rail { gap: var(--space-3) var(--space-6); padding: var(--space-4); }
  nav { order: 3; flex-basis: 100%; }
  .account { max-width: 100%; }
  main { padding: var(--space-6) var(--space-4) var(--space-12); }
 }
 @media (max-width: 26rem) {
  .account { margin-left: 0; }
  .nav-link { letter-spacing: 0; }
 }
</style>
