<script>
	// install flow: render GET /api/install as a checklist and surface the first
	// non-done entry as the actionable step. Backend contract in docs/spec/installation.md.
	let entries = $state([]);
	let loading = $state(true);
	let migrating = $state(false);
	let migrateError = $state("");

	const origin = window.location.origin;

	async function load() {
		loading = true;
		const resp = await fetch("/api/install");
		entries = resp.ok ? await resp.json() : [];
		loading = false;
	}

	async function runMigrations() {
		migrating = true;
		migrateError = "";

		const resp = await fetch("/api/install/migrations", { method: "POST" });
		if (resp.ok) {
			entries = await resp.json();
		} else {
			migrateError = await resp.text();
		}

		migrating = false;
	}

	// first non-done entry drives the step content; null once every entry is done.
	let next = $derived(entries.find((entry) => entry.status !== "done") ?? null);

	function isStep(name, status) {
		return next !== null && next.name === name && next.status === status;
	}

	load();
</script>

{#if loading}
	<section class="p9-section">
		<p class="p9-section__note">Loading…</p>
	</section>
{:else}
	<section class="p9-section" style="border-top: none;">
		<h1 class="p9-section__title">Install</h1>
		<p class="p9-section__note">Complete each step to bring the server up.</p>

		<ul class="checklist">
			{#each entries as entry (entry.name)}
				<li>
					<span class="status status--{entry.status}">{entry.status}</span>
					<span class="name">{entry.name}</span>
					{#if entry.message}
						<span class="message">{entry.message}</span>
					{/if}
				</li>
			{/each}
		</ul>

		{#if next === null}
			<div class="p9-panel">
				<p class="p9-panel__label">Installed</p>
				<p class="p9-section__note" style="margin: 0;">
					Restart the server to start.
				</p>
			</div>
		{:else if isStep("db-reachable", "error")}
			<div class="p9-panel">
				<p class="p9-panel__label">Database unreachable</p>
				<p class="p9-section__note" style="margin: 0;">{next.message}</p>
			</div>
		{:else if isStep("app-credentials", "error")}
			<div class="p9-panel">
				<p class="p9-panel__label">Create the GitHub App</p>
				<ol class="steps">
					<li>Create a GitHub App and set these permissions:
						<ul>
							<li><code>contents: write</code></li>
							<li><code>metadata: read</code></li>
						</ul>
					</li>
					<li>Webhook URL: <code>{origin}/hooks/github</code></li>
					<li>OAuth callback URL: <code>{origin}/auth/github/callback</code></li>
					<li>Restrict the App to the managed org.</li>
					<li>
						Copy the created App's id, private key, and client/webhook secrets into
						the server's fx config, then restart.
					</li>
				</ol>
			</div>
		{:else if isStep("app-installed", "pending")}
			<div class="p9-panel">
				<p class="p9-panel__label">Install the App on the org</p>
				<p class="p9-section__note">
					Install the created App on the managed GitHub org. Installation completes
					on redirect back to the server.
				</p>
				<div class="p9-actions">
					<a
						class="p9-btn p9-btn--primary"
						href="https://github.com/settings/apps"
						target="_blank"
						rel="noreferrer">Open GitHub Apps</a
					>
				</div>
			</div>
		{:else if isStep("migrations", "pending")}
			<div class="p9-panel">
				<p class="p9-panel__label">Run migrations</p>
				<p class="p9-section__note">Bring the schema up to date.</p>
				{#if migrateError}
					<p class="p9-section__note error">{migrateError}</p>
				{/if}
				<div class="p9-actions">
					<button
						class="p9-btn p9-btn--primary"
						onclick={runMigrations}
						disabled={migrating}>{migrating ? "Running…" : "Run migrations"}</button
					>
				</div>
			</div>
		{:else if isStep("migrations", "error")}
			<div class="p9-panel">
				<p class="p9-panel__label">Migration blocked</p>
				<p class="p9-section__note" style="margin: 0;">{next.message}</p>
			</div>
		{/if}
	</section>
{/if}

<style>
	.checklist {
		list-style: none;
		padding: 0;
		margin: 0 0 32px;
		font-family: var(--p9-mono);
		font-size: 13px;
	}

	.checklist li {
		display: flex;
		align-items: baseline;
		gap: 12px;
		padding: 10px 0;
		border-bottom: 1px solid var(--p9-line);
	}

	.name {
		color: var(--p9-ink);
	}

	.message {
		color: var(--p9-muted);
	}

	.status {
		font-family: var(--p9-support);
		font-size: 12px;
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.08em;
		min-width: 8ch;
	}

	.status--done {
		color: var(--p9-indigo-2);
	}

	.status--pending {
		color: var(--p9-muted);
	}

	.status--error {
		color: var(--p9-red-deep);
	}

	.steps {
		margin: 0;
		padding-left: 20px;
		color: var(--p9-ink);
	}

	.steps li {
		margin: 8px 0;
	}

	.steps code {
		font-family: var(--p9-mono);
		font-size: 13px;
		background: var(--p9-soft);
		border: 1px solid var(--p9-line);
		border-radius: 4px;
		padding: 1px 6px;
	}

	.error {
		color: var(--p9-red-deep);
	}
</style>
