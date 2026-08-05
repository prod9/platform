<script>
	// The install gate. GET /api/install returns the ordered checklist; the first
	// non-fully-ready entry is the step, and this page renders it in three columns:
	// progress on the left, the step's action in the middle, its operative
	// instructions on the right (docs/spec/installation.md §The wizard UI).
	import {
		installState,
		runMigrations,
		saveCredentials,
		claimInstall,
		setupFlux,
		errorText,
		Answered,
	} from "$lib/server.js";
	import { nextStep, credentialsPayload, orgSettingsURL } from "$lib/install.js";
	import { session } from "$lib/session.svelte.js";
	import Panel from "$lib/components/Panel.svelte";
	import Button from "$lib/components/Button.svelte";

	let entries = $state([]);
	let loaded = $state(false);
	let migrating = $state(false);
	let migrateError = $state("");
	let credentials = $state({
		app_id: "",
		private_key: "",
		webhook_secret: "",
		client_id: "",
		client_secret: "",
	});
	let savingCredentials = $state(false);
	let credentialsError = $state("");
	let claiming = $state(false);
	let claimError = $state("");
	let receiverURL = $state("");
	let savingFlux = $state(false);
	let fluxError = $state("");

	const origin = window.location.origin;

	// The App's Setup URL lands the browser here carrying GitHub's installation_id — the
	// landing GET only renders; the write sits behind the claim POST
	// (docs/spec/installation.md §The install settings). Signing in bounces through GitHub
	// and back to /, dropping the query string, so the id is stashed for the return trip.
	// The org slug only feeds the instruction links; it survives the OAuth bounce the
	// same way the installation id does. The server never sees it — the claim derives
	// the real org from the installation.
	const orgKey = "install.org";
	let org = $state(sessionStorage.getItem(orgKey) ?? "");
	$effect(() => sessionStorage.setItem(orgKey, org));
	let appsNewURL = $derived(orgSettingsURL(org, "apps/new"));
	let appsURL = $derived(orgSettingsURL(org, "apps"));

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
			migrateError = errorText(result);
		}

		migrating = false;
	}

	async function submitCredentials() {
		savingCredentials = true;
		credentialsError = "";

		const result = await saveCredentials(credentialsPayload(credentials));
		if (result.outcome === Answered) {
			entries = result.body;
		} else {
			credentialsError = errorText(result);
		}

		savingCredentials = false;
	}

	async function claim() {
		claiming = true;
		claimError = "";

		const result = await claimInstall(installationID);
		if (result.outcome === Answered) {
			await load();
		} else {
			claimError = errorText(result);
		}

		claiming = false;
	}

	async function submitFlux() {
		savingFlux = true;
		fluxError = "";

		const result = await setupFlux(receiverURL.trim());
		if (result.outcome === Answered) {
			entries = result.body;
		} else {
			fluxError = errorText(result);
		}

		savingFlux = false;
	}

	let next = $derived(nextStep(entries));
	let credentialsReady = $derived(
		Object.values(credentials).every((value) => value.trim() !== ""),
	);

	function isStep(name, ...states) {
		if (next === null) {
			return false;
		}
		return next.name === name && states.includes(next.state);
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
		<div class="wizard">
			<ol class="checklist">
				{#each entries as entry (entry.name)}
					<li class:active={next !== null && entry.name === next.name}>
						<span class="mono name">{entry.name}</span>
						<span class="state state--{entry.state || 'unknown'} label"
							>{entry.state || "unknown"}</span
						>
						{#if entry.message}
							<span class="mono failed message">{entry.message}</span>
						{/if}
					</li>
				{/each}
			</ol>

			<div class="action">
				{#if next === null}
					<Panel label="Installed">
						<p class="muted">Restart the server to start.</p>
					</Panel>
				{:else if next.name === "db-reachable"}
					<Panel label="Database unreachable">
						<p class="failed mono">{next.message}</p>
					</Panel>
				{:else if next.name === "app-credentials"}
					<Panel label="GitHub App credentials">
						{#if next.message}
							<p class="failed mono">{next.message}</p>
						{/if}
						{#if credentialsError}
							<p class="failed mono">{credentialsError}</p>
						{/if}
						<div class="fields">
							<label>
								<span class="label">App id</span>
								<input inputmode="numeric" bind:value={credentials.app_id} />
							</label>
							<label>
								<span class="label">Private key (PEM)</span>
								<textarea rows="6" bind:value={credentials.private_key}></textarea>
							</label>
							<label>
								<span class="label">Webhook secret</span>
								<input bind:value={credentials.webhook_secret} />
							</label>
							<label>
								<span class="label">Client id</span>
								<input bind:value={credentials.client_id} />
							</label>
							<label>
								<span class="label">Client secret</span>
								<input bind:value={credentials.client_secret} />
							</label>
						</div>
						<Button
							variant="primary"
							onclick={submitCredentials}
							disabled={!credentialsReady || savingCredentials}
						>
							{savingCredentials ? "Saving…" : "Save credentials"}
						</Button>
					</Panel>
				{:else if isStep("app-installed", "not_started")}
					{#if !installationID}
						<Panel label="Install the App on the org">
							<Button
								variant="primary"
								href={appsURL ?? "https://github.com/settings/organizations"}
							>
								{appsURL ? "Open the org's Apps" : "Open your orgs"}
							</Button>
						</Panel>
					{:else if session.user === null}
						<Panel label="Claim the installation">
							<Button variant="primary" href="/auth/github">Sign in with GitHub</Button>
						</Panel>
					{:else}
						<Panel label="Claim the installation">
							<p class="muted">
								Bind installation <span class="mark">#{installationID}</span> to this
								server as {session.user.name}.
							</p>
							{#if claimError}
								<p class="failed mono">{claimError}</p>
							{/if}
							<Button variant="primary" onclick={claim} disabled={claiming}>
								{claiming ? "Claiming…" : "Claim installation"}
							</Button>
						</Panel>
					{/if}
				{:else if isStep("flux-setup", "not_started")}
					<Panel label="Wire delivery">
						{#if fluxError}
							<p class="failed mono">{fluxError}</p>
						{/if}
						<div class="fields">
							<label>
								<span class="label">Flux Receiver URL</span>
								<input placeholder="https://…" bind:value={receiverURL} />
							</label>
						</div>
						<Button
							variant="primary"
							onclick={submitFlux}
							disabled={receiverURL.trim() === "" || savingFlux}
						>
							{savingFlux ? "Wiring…" : "Create org webhook"}
						</Button>
					</Panel>
				{:else if isStep("migrations", "not_started", "partially_ready")}
					<Panel label="Run migrations">
						{#if migrateError}
							<p class="failed mono">{migrateError}</p>
						{/if}
						<Button variant="primary" onclick={migrate} disabled={migrating}>
							{migrating ? "Running…" : "Run migrations"}
						</Button>
					</Panel>
				{:else if next.name === "migrations"}
					<Panel label="Migration blocked">
						<p class="failed mono">{next.message}</p>
					</Panel>
				{:else}
					<Panel label="Step failed">
						<p class="failed mono">{next.message}</p>
					</Panel>
				{/if}
			</div>

			<aside class="instructions">
				{#if next === null}
					<p class="label">Done</p>
					<p>
						Every step is ready. Restart the server so it boots into the product —
						the installer retires itself.
					</p>
				{:else if next.name === "db-reachable"}
					<p class="label">What this means</p>
					<p>
						The server cannot reach its database. Fix the deployment's
						<code>DATABASE_URL</code> — this is an operator concern, not a wizard step.
					</p>
				{:else if next.name === "app-credentials"}
					<p class="label">Create the GitHub App</p>
					<label class="org">
						<span class="label">Org slug</span>
						<input placeholder="your-org" bind:value={org} />
					</label>
					<ol class="steps">
						<li>
							Create a GitHub App <strong>under the managed org</strong> at
							{#if appsNewURL}
								<a href={appsNewURL} target="_blank">{appsNewURL}</a>
							{:else}
								<code>github.com/organizations/&lt;org&gt;/settings/apps/new</code>
							{/if}
							(the org's Settings → Developer settings → GitHub Apps). The form top to
							bottom:
						</li>
						<li>Callback URL: <code>{origin}/auth/github/callback</code></li>
						<li>
							Webhook: Active, URL <code>{origin}/hooks/github</code>, and set a
							webhook secret — keep a copy, the form here needs it.
						</li>
						<li>
							Permissions (the form's last section):
							<em>Repository permissions</em> → Contents: Read and write, Metadata:
							Read-only; <em>Organization permissions</em> → Members: Read-only (the
							claim reads org memberships to prove ownership).
						</li>
						<li>Where can it be installed: Only on this account.</li>
						<li>
							After creating, on the App's settings page generate a
							<strong>private key</strong> and a <strong>client secret</strong>.
						</li>
						<li>Paste the App's values into the form and save.</li>
					</ol>
				{:else if isStep("app-installed", "not_started")}
					<p class="label">Install and claim</p>
					{#if !installationID}
						<p>
							In the App's settings at
							{#if appsURL}
								<a href={appsURL} target="_blank">{appsURL}</a>{:else}
								<code>github.com/organizations/&lt;org&gt;/settings/apps</code>{/if},
							set the Setup URL to <code>{origin}/install/</code>, then install the App
							on the managed org — GitHub redirects back here to finish.
						</p>
					{:else if session.user === null}
						<p>
							Sign in with a GitHub account that owns the org. That account becomes
							the seed admin.
						</p>
					{:else}
						<p>
							Claiming binds this installation to the server and marks it installed.
							The claim verifies you are an active owner of the org.
						</p>
					{/if}
				{:else if isStep("flux-setup", "not_started")}
					<p class="label">Wire delivery</p>
					<p>
						A published image reaches the cluster because GitHub's
						<code>registry_package</code> webhook pokes the cluster's Flux Receiver.
						Paste the Receiver's public URL (from the cluster's Flux setup — ask your
						infra operator); the server creates the org-wide webhook through the App.
						Wired once, never per repo; re-running converges.
					</p>
				{:else if isStep("migrations", "not_started", "partially_ready")}
					<p class="label">Run migrations</p>
					<p>
						Creates the schema every later step stores its values in. Re-runnable;
						it only ever applies what is missing.
					</p>
				{:else if next.name === "migrations"}
					<p class="label">What this means</p>
					<p>
						The database schema diverges from what this server ships. Review the
						database by hand — the wizard will not overwrite an unknown schema.
					</p>
				{:else}
					<p class="label">What this means</p>
					<p>The check itself failed; the message on the left carries the cause.</p>
				{/if}
			</aside>
		</div>
	{/if}
</section>

<style>
	section {
		max-width: 150ch;
	}

	.head {
		display: flex;
		align-items: baseline;
		gap: var(--lead);
		margin-bottom: var(--lead);
	}

	.wizard {
		display: grid;
		grid-template-columns: minmax(26ch, 1fr) minmax(0, 2fr) minmax(30ch, 1.5fr);
		gap: var(--lead-2);
		align-items: start;
	}

	@media (max-width: 900px) {
		.wizard {
			grid-template-columns: minmax(0, 1fr);
			gap: var(--lead);
		}
	}

	.checklist {
		margin: 0;
		padding: 0;
		list-style: none;
	}

	.checklist li {
		display: grid;
		grid-template-columns: minmax(0, 1fr) auto;
		padding: var(--lead-half) 0 var(--lead-half) var(--lead-half);
		box-shadow: 0 -1px 0 var(--border) inset;
	}

	.checklist li.active {
		box-shadow:
			2px 0 0 var(--accent-signal) inset,
			0 -1px 0 var(--border) inset;
	}

	.checklist .name {
		line-height: var(--lead);
	}

	.checklist li.active .name {
		color: var(--accent);
		font-weight: 600;
	}

	.checklist .state {
		line-height: var(--lead);
	}

	.checklist .message {
		grid-column: 1 / -1;
	}

	.state--fully_ready {
		color: var(--accent);
	}

	.state--not_started,
	.state--partially_ready {
		color: var(--text-muted);
	}

	.state--intervention_required,
	.state--unknown {
		color: var(--accent-signal);
	}

	.instructions .label {
		color: var(--accent);
		margin-bottom: var(--lead-half);
	}

	/* Built URLs (org links, webhook/callback paths) can outgrow the column;
	   break anywhere rather than pushing the grid past the viewport. */
	.instructions a,
	.instructions code {
		overflow-wrap: anywhere;
	}

	.org {
		display: grid;
		gap: 2px;
		margin-bottom: var(--lead);
	}

	.org input {
		max-width: 24ch;
		padding: 0 var(--lead-half);
		border: 1px solid var(--border);
		border-radius: var(--radius-sm);
		background: var(--surface-raised);
		font-family: var(--p9-mono);
		line-height: var(--lead);
		color: var(--text);
	}

	.instructions p {
		margin: 0 0 var(--lead-half);
	}

	.steps {
		margin: 0;
		padding-left: var(--lead);
	}

	.steps li {
		line-height: var(--lead);
	}

	.steps li::marker {
		color: var(--accent-signal);
		font-family: var(--p9-mono);
	}

	.mark {
		color: var(--accent-signal);
		font-family: var(--p9-mono);
	}

	.failed {
		color: var(--accent-signal);
	}

	.fields {
		display: grid;
		gap: var(--lead-half);
		margin: var(--lead) 0;
	}

	.fields label {
		display: grid;
		gap: 2px;
	}

	.fields input,
	.fields textarea {
		padding: 0 var(--lead-half);
		border: 1px solid var(--border);
		border-radius: var(--radius-sm);
		background: var(--surface-raised);
		font-family: var(--p9-mono);
		line-height: var(--lead);
		color: var(--text);
	}

	.fields input:focus,
	.fields textarea:focus {
		outline: 2px solid var(--accent);
		outline-offset: -1px;
	}

	.fields textarea {
		padding: var(--lead-half);
		resize: vertical;
	}
</style>
