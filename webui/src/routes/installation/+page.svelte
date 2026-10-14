<script>
	import "$lib/styles/wizard.css";
	// The installation gate. GET /api/installation returns the ordered checklist; the progress
	// list is navigation — the default selection is the first non-fully-ready entry,
	// clicking an entry opens its panel — and the page renders three columns: progress
	// on the left, the selected step's action in the middle, its operative
	// instructions on the right (docs/spec/installation.md §The wizard UI).
	import {
		installState,
		errorText,
		installSignal,
		Answered,
		Installed,
		authenticationURL,
	} from "$lib/server.js";
	import {
		nextStep,
		orgSlug,
		publicURL,
		originMismatch,
		orgSettingsURL,
		appSettingsURL,
	} from "$lib/install.js";
	import { session } from "$lib/session.svelte.js";
	import Panel from "$lib/components/Panel.svelte";
	import Button from "$lib/components/Button.svelte";
	import LoadingBlock from "$lib/components/LoadingBlock.svelte";
	import PageHeader from "$lib/components/PageHeader.svelte";
	import InstallationAction from "$lib/components/InstallationAction.svelte";
	import InstallationInstructions from "$lib/components/InstallationInstructions.svelte";

	const loadingSteps = Array.from({ length: 9 });

	let entries = $state([]);
	let loaded = $state(false);
	let loadError = $state("");
	let selected = $state(null); // checklist navigation; null = follow the wizard
	let redoing = $state(false); // client-side unlock of a done panel

	const origin = window.location.origin;

	// The App's Setup URL lands the browser here carrying GitHub's installation_id — the
	// landing GET only renders; the write sits behind the claim POST
	// (docs/spec/installation.md §The install settings). Signing in bounces through GitHub
	// and back to this exact location; the stash also survives an interrupted browser
	// navigation before OAuth begins.
	const stashKey = "install.installation_id";
	const landed = new URLSearchParams(window.location.search).get("installation_id");
	if (landed) {
		sessionStorage.setItem(stashKey, landed);
	}
	const installationID = Number(landed ?? sessionStorage.getItem(stashKey));
	const signInURL = installationID
		? authenticationURL(
				`/installation/?installation_id=${installationID}`,
				String(installationID),
			)
		: authenticationURL("/installation/", null);

	// current is the one panel on screen: the operator's pick, or the wizard's next.
	let current = $derived(
		entries.find((entry) => entry.name === selected) ?? nextStep(entries),
	);
	// Only fully_ready locks a panel; Redo is a client-side unlock — the server learns
	// of a redo only as an ordinary save (docs/spec/installation.md §Redo).
	let locked = $derived(current !== null && current.state === "fully_ready" && !redoing);

	// Every save's response (and every page load) is a fresh state read; adopting it
	// drops the navigation pick and re-locks, so the wizard always converges onto the
	// first unfinished step (§The wizard UI, restartable).
	function converge(body) {
		entries = body;
		selected = null;
		redoing = false;
	}

	function select(name) {
		selected = name;
		redoing = false;
	}

	function redo() {
		redoing = true;
	}

	// The wizard renders operative values and controls only after this first read is
	// adopted (§The wizard UI, a form is editable only when its values are settled).
	// The read classifies by the install signal: a 404 means the server got installed
	// since this shell loaded — load the product shell, same
	// as rideRestart. Only a genuinely troubled read renders as the error it is, never as
	// an empty checklist masquerading as done.
	async function load() {
		const result = await installState();
		if (installSignal(result) === Installed) {
			window.location.assign("/");
			return;
		}
		if (result.outcome === Answered) {
			converge(result.body);
			loaded = true;
		} else {
			loadError = errorText(result);
		}
	}

	// Every GitHub link builds from the slug the org step saved server-side, so any
	// tab or browser renders real links (§The state surface).
	let slug = $derived(orgSlug(entries));
	// Instructions render the server-side public URL, never the browser origin; the
	// origin is only the pre-save suggestion (§the server step).
	let base = $derived(publicURL(entries) || origin);
	let mismatch = $derived(originMismatch(entries, origin));
	let appsNewURL = $derived(orgSettingsURL(slug, "apps/new"));
	let appEditURL = $derived(appSettingsURL(entries));
	let appInstallURL = $derived(appSettingsURL(entries, "/installations"));

	$effect(() => {
		if (session.ready) {
			load();
		}
	});
</script>

<section>
	<PageHeader>
		{#snippet title()}<h2>Install</h2>{/snippet}
		{#snippet metadata()}<p class="label">Each step brings the server up</p>{/snippet}
	</PageHeader>

	{#if loadError}
		<p class="failed mono">{loadError}</p>
		<Button onclick={() => ((loadError = ""), load())}>Retry</Button>
	{:else if !loaded}
		<div class="wizard" aria-busy="true">
			<ol class="checklist">
				{#each loadingSteps as _}
					<li>
						<button class="row" disabled aria-label="Loading installation step">
							<LoadingBlock measure="standard" />
							<LoadingBlock measure="compact" />
						</button>
					</li>
				{/each}
			</ol>

			<div class="action">
				<InstallationAction current={null} />
			</div>

			<aside class="loading-instructions">
				<LoadingBlock measure="standard" />
				<LoadingBlock />
				<LoadingBlock />
			</aside>
		</div>
	{:else}
		{#if mismatch}
			<p class="mismatch mono">
				This page is open on <code>{origin}</code> but the server's public URL is
				<code>{base}</code> — values pasted into GitHub from here may point at the
				wrong place. Prefer the canonical host, or redo the server step.
			</p>
		{/if}
		<div class="wizard">
			<ol class="checklist">
				{#each entries as entry (entry.name)}
					<li class:active={current !== null && entry.name === current.name}>
						<button type="button" class="row" onclick={() => select(entry.name)}>
							<span class="mono name">{entry.name}</span>
							<span class="state state--{entry.state || 'unknown'} label"
								>{entry.state || "unknown"}</span
							>
							{#if entry.message}
								<span class="mono failed message">{entry.message}</span>
							{/if}
						</button>
					</li>
				{/each}
			</ol>

			<div class="action">
				{#if current === null}
					<Panel label="Installed">
						<p class="muted">
							Every step is ready. The server restarts itself into the product after the
							claim — reload this page if it lingers here.
						</p>
					</Panel>
				{:else}
					<InstallationAction
						{current}
						{entries}
						{locked}
						{origin}
						{installationID}
						{signInURL}
						{appInstallURL}
						onconverge={converge}
						onredo={redo}
					/>
				{/if}
			</div>

			<InstallationInstructions
				{current}
				{locked}
				{appsNewURL}
				{base}
				{appEditURL}
				{appInstallURL}
				user={session.user}
			/>
		</div>
	{/if}
</section>

<style>
	.failed {
		color: var(--error-ink);
		overflow-wrap: anywhere;
	}

	.mismatch {
		padding: var(--space-4) var(--space-6);
		margin-bottom: var(--space-6);
		background: var(--soft);
		box-shadow: inset 3px 0 0 var(--red);
		color: var(--error-ink);
		overflow-wrap: anywhere;
	}
</style>
