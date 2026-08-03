<script>
	// The install gate. GET /api/install returns the ordered checklist; the first non-done
	// entry is the step, and this page carries the operative instructions for it
	// (docs/spec/installation.md).
	import { installState, runMigrations, claimInstall, Answered } from "$lib/server.js";
	import { session } from "$lib/session.svelte.js";
	import Panel from "$lib/components/Panel.svelte";
	import Button from "$lib/components/Button.svelte";

	let entries = $state([]);
	let loaded = $state(false);
	let migrating = $state(false);
	let migrateError = $state("");
	let claiming = $state(false);
	let claimError = $state("");

	const origin = window.location.origin;

	// The App's Setup URL lands the browser here carrying GitHub's installation_id — the
	// landing GET only renders; the write sits behind the claim POST
	// (docs/spec/installation.md §The install record). Signing in bounces through GitHub
	// and back to /, dropping the query string, so the id is stashed for the return trip.
	const stashKey = "install.installation_id";
	const landed = new URLSearchParams(window.location.search).get("installation_id");
	if (landed) {
		sessionStorage.setItem(stashKey, landed);
	}
	const installationID = Number(landed ?? sessionStorage.getItem(stashKey));

	async function load() {
		const result = await installState();
		if (result.outcome === Answered) {
			entries = result.body;
		}
		loaded = true;
	}

	async function migrate() {
		migrating = true;
		migrateError = "";

		const result = await runMigrations();
		if (result.outcome === Answered) {
			entries = result.body;
		} else {
			migrateError = result.body;
		}

		migrating = false;
	}

	async function claim() {
		claiming = true;
		claimError = "";

		const result = await claimInstall(installationID);
		if (result.outcome === Answered) {
			await load();
		} else {
			claimError = result.body;
		}

		claiming = false;
	}

	// The first entry that is not done is the step; null once every one of them is.
	let next = $derived(entries.find((entry) => entry.status !== "done") ?? null);

	function isStep(name, status) {
		if (next === null) {
			return false;
		}
		return next.name === name && next.status === status;
	}

	load();
</script>

<section>
	<div class="head">
		<h2>Install</h2>
		<p class="label">Each step brings the server up</p>
	</div>

	{#if !loaded}
		<p class="muted">Loading…</p>
	{:else}
		<ol class="checklist">
			{#each entries as entry (entry.name)}
				<li>
					<span class="state state--{entry.status} label">{entry.status}</span>
					<span class="mono">{entry.name}</span>
					<span class="mono muted">{entry.message ?? ""}</span>
				</li>
			{/each}
		</ol>

		{#if next === null}
			<Panel label="Installed">
				<p class="muted">Restart the server to start.</p>
			</Panel>
		{:else if isStep("db-reachable", "error")}
			<Panel label="Database unreachable">
				<p class="muted">{next.message}</p>
			</Panel>
		{:else if isStep("app-credentials", "error")}
			<Panel label="Create the GitHub App">
				<ol class="steps">
					<li>
						Create a GitHub App with <code>contents: write</code>,
						<code>metadata: read</code>, and <code>organization members: read</code>
						(the claim reads org memberships to prove ownership).
					</li>
					<li>Webhook URL <code>{origin}/hooks/github</code></li>
					<li>OAuth callback URL <code>{origin}/auth/github/callback</code></li>
					<li>Restrict the App to the managed org.</li>
					<li>
						Copy the App's id, private key, and client and webhook secrets into the
						server's config, then restart.
					</li>
					<li>
						Add an organization webhook delivering <code>registry_package</code> to the
						cluster's Flux Receiver, so a published image pokes delivery. Org-wide,
						wired once.
					</li>
				</ol>
			</Panel>
		{:else if isStep("app-installed", "pending")}
			{#if !installationID}
				<Panel label="Install the App on the org">
					<p class="muted">
						Set the App's Setup URL to <code>{origin}/install/</code>, then install it
						on the managed org — GitHub redirects back here to finish.
					</p>
					<Button variant="primary" href="https://github.com/settings/apps">
						Open GitHub Apps
					</Button>
				</Panel>
			{:else if session.user === null}
				<Panel label="Claim the installation">
					<p class="muted">
						Sign in with a GitHub account that owns the org. That account becomes the
						seed admin.
					</p>
					<Button variant="primary" href="/auth/github">Sign in with GitHub</Button>
				</Panel>
			{:else}
				<Panel label="Claim the installation">
					<p class="muted">
						Bind installation #{installationID} to this server as {session.user.name}.
					</p>
					{#if claimError}
						<p class="failed mono">{claimError}</p>
					{/if}
					<Button variant="primary" onclick={claim} disabled={claiming}>
						{claiming ? "Claiming…" : "Claim installation"}
					</Button>
				</Panel>
			{/if}
		{:else if isStep("migrations", "pending")}
			<Panel label="Run migrations">
				<p class="muted">Bring the schema up to date.</p>
				{#if migrateError}
					<p class="failed mono">{migrateError}</p>
				{/if}
				<Button variant="primary" onclick={migrate} disabled={migrating}>
					{migrating ? "Running…" : "Run migrations"}
				</Button>
			</Panel>
		{:else if isStep("migrations", "error")}
			<Panel label="Migration blocked">
				<p class="muted">{next.message}</p>
			</Panel>
		{/if}
	{/if}
</section>

<style>
	section {
		max-width: 80ch;
	}

	.head {
		display: flex;
		align-items: baseline;
		gap: var(--lead);
		margin-bottom: var(--lead);
	}

	.checklist {
		margin: 0 0 var(--lead-2);
		padding: 0;
		list-style: none;
	}

	.checklist li {
		display: grid;
		grid-template-columns: 10ch 24ch minmax(0, 1fr);
		gap: var(--lead-half);
		line-height: var(--lead);
		box-shadow: 0 -1px 0 var(--border) inset;
	}

	.state--done {
		color: var(--text);
	}

	.state--pending {
		color: var(--text-muted);
	}

	.state--error {
		color: var(--accent-signal);
	}

	.steps {
		margin: 0;
		padding-left: var(--lead);
	}

	.steps li {
		line-height: var(--lead);
	}

	.failed {
		color: var(--accent-signal);
	}
</style>
