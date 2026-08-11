<script>
	// The install gate. GET /api/install returns the ordered checklist; the progress
	// list is navigation — the default selection is the first non-fully-ready entry,
	// clicking an entry opens its panel — and the page renders three columns: progress
	// on the left, the selected step's action in the middle, its operative
	// instructions on the right (docs/spec/installation.md §The wizard UI).
	import {
		installState,
		runMigrations,
		saveServer,
		saveOrg,
		saveApp,
		saveCredentials,
		saveRegistryToken,
		claimInstall,
		errorText,
		installSignal,
		Answered,
		Installed,
	} from "$lib/server.js";
	import {
		nextStep,
		stepValues,
		orgSlug,
		publicURL,
		originMismatch,
		serverPayload,
		orgPayload,
		appPayload,
		credentialsPayload,
		registryPayload,
		generateWebhookSecret,
		orgSettingsURL,
		appSettingsURL,
	} from "$lib/install.js";
	import { session } from "$lib/session.svelte.js";
	import Panel from "$lib/components/Panel.svelte";
	import Button from "$lib/components/Button.svelte";

	let entries = $state([]);
	let loaded = $state(false);
	let selected = $state(null); // checklist navigation; null = follow the wizard
	let redoing = $state(false); // client-side unlock of a done panel
	let migrating = $state(false);
	let migrateError = $state("");
	let server = $state({ public_url: "" });
	let savingServer = $state(false);
	let serverError = $state("");
	let org = $state({ org: "" });
	let savingOrg = $state(false);
	let orgError = $state("");
	let app = $state({
		app_id: "",
		app_slug: "",
		client_id: "",
		webhook_secret: generateWebhookSecret(),
	});
	let savingApp = $state(false);
	let appError = $state("");
	let credentials = $state({
		private_key: "",
		client_secret: "",
	});
	let savingCredentials = $state(false);
	let credentialsError = $state("");
	let registry = $state({ token: "" });
	let savingRegistry = $state(false);
	let registryError = $state("");
	let claiming = $state(false);
	let claimError = $state("");

	const origin = window.location.origin;

	// The App's Setup URL lands the browser here carrying GitHub's installation_id — the
	// landing GET only renders; the write sits behind the claim POST
	// (docs/spec/installation.md §The install settings). Signing in bounces through GitHub
	// and back to /, dropping the query string, so the id is stashed for the return trip.
	const stashKey = "install.installation_id";
	const landed = new URLSearchParams(window.location.search).get("installation_id");
	if (landed) {
		sessionStorage.setItem(stashKey, landed);
	}
	const installationID = Number(landed ?? sessionStorage.getItem(stashKey));

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
		prefill();
	}

	// Non-secret fields pre-fill from each entry's saved values; secret fields always
	// render empty (§The wizard UI).
	function prefill() {
		// The server panel suggests the browser origin while the setting is empty — a
		// suggestion only; the saved value is the truth (§the server step).
		server.public_url = stepValues(entries, "server").public_url ?? origin;
		org.org = stepValues(entries, "org").org ?? "";
		const created = stepValues(entries, "app-created");
		app.app_id = created.app_id ?? app.app_id;
		app.app_slug = created.app_slug ?? app.app_slug;
		app.client_id = created.client_id ?? app.client_id;
	}

	function select(name) {
		selected = name;
		redoing = false;
	}

	// redo unlocks the current panel. Secrets are always re-entered: they empty here,
	// and the webhook-secret mint fires for the now-unset field (§The wizard UI).
	function redo() {
		redoing = true;
		if (current.name === "app-created") {
			app.webhook_secret = generateWebhookSecret();
		} else if (current.name === "app-credentials") {
			credentials = { private_key: "", client_secret: "" };
		} else if (current.name === "registry-token") {
			registry = { token: "" };
		}
	}

	async function load() {
		const result = await installState();
		if (result.outcome === Answered) {
			converge(result.body);
		}
		loaded = true;
	}

	async function migrate() {
		migrating = true;
		migrateError = "";

		const result = await runMigrations();
		if (result.outcome === Answered) {
			converge(result.body);
		} else {
			migrateError = errorText(result);
		}

		migrating = false;
	}

	async function submitServer() {
		savingServer = true;
		serverError = "";

		const result = await saveServer(serverPayload(server));
		if (result.outcome === Answered) {
			converge(result.body);
		} else {
			serverError = errorText(result);
		}

		savingServer = false;
	}

	async function submitOrg() {
		savingOrg = true;
		orgError = "";

		const result = await saveOrg(orgPayload(org));
		if (result.outcome === Answered) {
			converge(result.body);
		} else {
			orgError = errorText(result);
		}

		savingOrg = false;
	}

	async function submitApp() {
		savingApp = true;
		appError = "";

		const result = await saveApp(appPayload(app));
		if (result.outcome === Answered) {
			converge(result.body);
		} else {
			appError = errorText(result);
		}

		savingApp = false;
	}

	async function submitCredentials() {
		savingCredentials = true;
		credentialsError = "";

		const result = await saveCredentials(credentialsPayload(credentials));
		if (result.outcome === Answered) {
			converge(result.body);
		} else {
			credentialsError = errorText(result);
		}

		savingCredentials = false;
	}

	async function pickPrivateKey(event) {
		const file = event.target.files[0];
		if (!file) {
			credentials.private_key = "";
			return;
		}

		try {
			credentials.private_key = await file.text();
		} catch (err) {
			credentials.private_key = "";
			credentialsError = String(err);
		}
	}

	async function submitRegistry() {
		savingRegistry = true;
		registryError = "";

		const result = await saveRegistryToken(registryPayload(registry));
		if (result.outcome === Answered) {
			converge(result.body);
		} else {
			registryError = errorText(result);
		}

		savingRegistry = false;
	}

	async function claim() {
		claiming = true;
		claimError = "";

		const result = await claimInstall(installationID);
		if (result.outcome === Answered) {
			await rideRestart();
			return;
		}
		claimError = errorText(result);

		claiming = false;
	}

	// A committed claim makes the server restart itself into the product composition
	// (§Boot composition) — this panel rides the blip: poll the state read until its
	// 404 says the installer is gone, then land on the product.
	async function rideRestart() {
		for (let attempt = 0; attempt < 30; attempt++) {
			const result = await installState();
			if (installSignal(result) === Installed) {
				window.location.assign("/");
				return;
			}
			await new Promise((resolve) => setTimeout(resolve, 1000));
		}
		claimError = "The server has not come back yet — reload this page.";
		claiming = false;
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

	let serverReady = $derived(server.public_url.trim() !== "");
	let orgReady = $derived(org.org.trim() !== "");
	let appReady = $derived(Object.values(app).every((value) => value.trim() !== ""));
	let credentialsReady = $derived(
		Object.values(credentials).every((value) => value.trim() !== ""),
	);
	let registryReady = $derived(registry.token.trim() !== "");

	function isStep(name, ...states) {
		if (current === null) {
			return false;
		}
		return current.name === name && states.includes(current.state);
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
							Every step is ready. The server restarts itself into the product
							after the claim — reload this page if it lingers here.
						</p>
					</Panel>
				{:else if current.name === "db-reachable"}
					<Panel label={locked ? "Database reachable" : "Database unreachable"}>
						{#if locked}
							<p class="muted">The server reaches its database. Nothing to redo.</p>
						{:else}
							<p class="failed mono">{current.message}</p>
						{/if}
					</Panel>
				{:else if current.name === "server"}
					<Panel label="Name the server">
						{#if current.message}
							<p class="failed mono">{current.message}</p>
						{/if}
						{#if serverError}
							<p class="failed mono">{serverError}</p>
						{/if}
						<div class="fields">
							<label>
								<span class="label">Public URL</span>
								<input
									placeholder="https://platform.example.com"
									bind:value={server.public_url}
									disabled={locked || savingServer}
								/>
							</label>
						</div>
						{#if locked}
							<Button onclick={redo}>Redo</Button>
						{:else}
							<Button
								variant="primary"
								onclick={submitServer}
								disabled={!serverReady || savingServer}
							>
								{savingServer ? "Saving…" : "Save URL"}
							</Button>
						{/if}
					</Panel>
				{:else if current.name === "org"}
					<Panel label="Name the primary org">
						{#if current.message}
							<p class="failed mono">{current.message}</p>
						{/if}
						{#if orgError}
							<p class="failed mono">{orgError}</p>
						{/if}
						<div class="fields">
							<label>
								<span class="label">Org slug</span>
								<input placeholder="your-org" bind:value={org.org} disabled={locked || savingOrg} />
							</label>
						</div>
						{#if locked}
							<Button onclick={redo}>Redo</Button>
						{:else}
							<Button
								variant="primary"
								onclick={submitOrg}
								disabled={!orgReady || savingOrg}
							>
								{savingOrg ? "Saving…" : "Save org"}
							</Button>
						{/if}
					</Panel>
				{:else if current.name === "app-created"}
					<Panel label="Create the GitHub App">
						{#if current.message}
							<p class="failed mono">{current.message}</p>
						{/if}
						{#if appError}
							<p class="failed mono">{appError}</p>
						{/if}
						<div class="fields">
							<label>
								<span class="label">Webhook secret (copy into GitHub's form)</span>
								<span class="secret">
									<input bind:value={app.webhook_secret} disabled={locked || savingApp} />
									<button
										type="button"
										title="Regenerate"
										disabled={locked || savingApp}
										onclick={() => (app.webhook_secret = generateWebhookSecret())}
										>↻</button
									>
								</span>
							</label>
							<label>
								<span class="label">App id</span>
								<input inputmode="numeric" bind:value={app.app_id} disabled={locked || savingApp} />
							</label>
							<label>
								<span class="label">App URL (paste from the address bar)</span>
								<input
									placeholder="https://github.com/organizations/…/settings/apps/my-app"
									bind:value={app.app_slug}
									disabled={locked || savingApp}
								/>
							</label>
							<label>
								<span class="label">Client id</span>
								<input bind:value={app.client_id} disabled={locked || savingApp} />
							</label>
						</div>
						{#if locked}
							<Button onclick={redo}>Redo</Button>
						{:else}
							<Button
								variant="primary"
								onclick={submitApp}
								disabled={!appReady || savingApp}
							>
								{savingApp ? "Saving…" : "Save App"}
							</Button>
						{/if}
					</Panel>
				{:else if current.name === "app-credentials"}
					<Panel label="GitHub App keys">
						{#if current.message}
							<p class="failed mono">{current.message}</p>
						{/if}
						{#if credentialsError}
							<p class="failed mono">{credentialsError}</p>
						{/if}
						<div class="fields">
							<label>
								<span class="label">Client secret</span>
								<input bind:value={credentials.client_secret} disabled={locked || savingCredentials} />
							</label>
							<label>
								<span class="label">Private key (the downloaded .pem)</span>
								<input type="file" accept=".pem" onchange={pickPrivateKey} disabled={locked || savingCredentials} />
							</label>
						</div>
						{#if locked}
							<Button onclick={redo}>Redo</Button>
						{:else}
							<Button
								variant="primary"
								onclick={submitCredentials}
								disabled={!credentialsReady || savingCredentials}
							>
								{savingCredentials ? "Saving…" : "Save keys"}
							</Button>
						{/if}
					</Panel>
				{:else if current.name === "registry-token"}
					<Panel label="Registry push token">
						{#if current.message}
							<p class="failed mono">{current.message}</p>
						{/if}
						{#if registryError}
							<p class="failed mono">{registryError}</p>
						{/if}
						<div class="fields">
							<label>
								<span class="label">Classic PAT (write:packages)</span>
								<input type="password" bind:value={registry.token} disabled={locked || savingRegistry} />
							</label>
						</div>
						{#if locked}
							<Button onclick={redo}>Redo</Button>
						{:else}
							<Button
								variant="primary"
								onclick={submitRegistry}
								disabled={!registryReady || savingRegistry}
							>
								{savingRegistry ? "Saving…" : "Save token"}
							</Button>
						{/if}
					</Panel>
				{:else if isStep("app-installed", "fully_ready")}
					<Panel label="App installed">
						<p class="muted">
							GitHub reports the App installed on the org. Undoing this is
							uninstalling it on GitHub — nothing to redo here.
						</p>
					</Panel>
				{:else if isStep("app-installed", "not_started")}
					<Panel label="Install the App on the org">
						{#if current.message}
							<p class="failed mono">{current.message}</p>
						{/if}
						{#if appInstallURL}
							<Button variant="primary" href={appInstallURL} target="_blank">
								Install the App
							</Button>
						{:else}
							<p class="muted">
								The install link needs the org and App slugs — redo the org and
								App steps if they are missing.
							</p>
						{/if}
					</Panel>
				{:else if isStep("claimed", "fully_ready")}
					<Panel label="Claimed">
						<p class="muted">
							The installation is bound to this server; it restarts itself into
							the product. Reload this page if it lingers here.
						</p>
					</Panel>
				{:else if isStep("claimed", "not_started")}
					{#if session.user === null}
						<Panel label="Claim the installation">
							<Button variant="primary" href="/auth/github">Sign in with GitHub</Button>
						</Panel>
					{:else if !installationID}
						<Panel label="Claim the installation">
							<p class="muted">
								The claim needs GitHub's <code>installation_id</code>, which arrives
								on the Setup URL redirect. Open the installed App's page and save its
								repository selection to fire the redirect again.
							</p>
							{#if appInstallURL}
								<Button variant="primary" href={appInstallURL} target="_blank">
									Open the installation
								</Button>
							{/if}
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
				{:else if isStep("migrations", "fully_ready")}
					<Panel label="Schema is current">
						<p class="muted">Every migration is applied. Nothing to redo.</p>
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
				{:else if current.name === "migrations"}
					<Panel label="Migration blocked">
						<p class="failed mono">{current.message}</p>
					</Panel>
				{:else}
					<Panel label="Step failed">
						<p class="failed mono">{current.message}</p>
					</Panel>
				{/if}
			</div>

			<aside class="instructions">
				{#if current === null}
					<p class="label">Done</p>
					<p>
						Every step is ready. The server restarts itself into the product after
						the claim; the installer retires with it.
					</p>
				{:else if current.name === "db-reachable"}
					<p class="label">What this means</p>
					<p>
						{#if locked}
							The deployment's <code>DATABASE_URL</code> answers. This step has no
							saved values; it re-checks on every load.
						{:else}
							The server cannot reach its database. Fix the deployment's
							<code>DATABASE_URL</code> — this is an operator concern, not a wizard step.
						{/if}
					</p>
				{:else if current.name === "server"}
					<p class="label">Name the server</p>
					<p>
						The server's public URL — the one server-side truth of where this
						deployment lives. Login's OAuth redirect, the go-get vanity host, and
						every "the server's URL" the later steps render come from this value,
						not from whatever host this page happens to be open on.
					</p>
					<p>
						The field suggests this page's own origin; correct it if the canonical
						host differs. Redoing it resets every later step.
					</p>
				{:else if current.name === "org"}
					<p class="label">Name the primary org</p>
					<p>
						Every GitHub link the later steps render is built from this slug — the
						App is created wherever those links point, so it heads the settings-backed
						steps.
					</p>
					<p>
						A wrong slug simply 404s the links; redo this step to fix it. Redoing it
						resets every later step.
					</p>
				{:else if current.name === "app-created"}
					<p class="label">Create the GitHub App</p>
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
						<li>Homepage URL: <code>{base}</code></li>
						<li>Callback URL: <code>{base}/auth/github/callback</code></li>
						<li>
							Setup URL: <code>{base}/install/</code>, with
							<strong>Redirect on update</strong> checked — GitHub sends the browser
							back here after the App is installed later.
						</li>
						<li>
							Webhook:
							<ul>
								<li>Active: checked</li>
								<li>URL: <code>{base}/hooks/github</code></li>
								<li>
									Secret: the <strong>webhook secret</strong> the form here minted —
									regenerate it until you trust it, then copy it across
								</li>
							</ul>
						</li>
						<li>
							Permissions (the form's last section):
							<ul>
								<li><em>Repository</em> → Contents: Read and write</li>
								<li><em>Repository</em> → Metadata: Read-only</li>
								<li>
									<em>Organization</em> → Members: Read-only (the claim reads org
									memberships to prove ownership)
								</li>
							</ul>
						</li>
						<li>
							Subscribe to events: <strong>Push</strong>, and only Push — tag
							pushes trigger builds through it; nothing ticked means nothing ever
							builds.
						</li>
						<li>Where can it be installed: Only on this account.</li>
						<li>
							Paste the created App's values (its settings page, About) into the
							form and save:
							<ul>
								<li><strong>App id</strong></li>
								<li>
									<strong>App URL</strong> — paste the created App's page URL
									straight from the address bar; only its slug saves, and later
									steps link the App's pages directly through it
								</li>
								<li><strong>Client id</strong></li>
							</ul>
						</li>
					</ol>
				{:else if current.name === "app-credentials"}
					<p class="label">Generate the App's keys</p>
					<ol class="steps">
						<li>
							Open the created App's settings page —
							{#if appEditURL}
								<a href={appEditURL} target="_blank">{appEditURL}</a>{:else}
								<code>github.com/organizations/&lt;org&gt;/settings/apps/&lt;slug&gt;</code>{/if}
						</li>
						<li>
							Under <em>Client secrets</em>, generate a
							<strong>client secret</strong>.
						</li>
						<li>
							Under <em>Private keys</em>, generate a <strong>private key</strong> —
							GitHub downloads a <code>.pem</code> file; pick that file in the form.
						</li>
						<li>Enter the secret, pick the file, and save.</li>
					</ol>
				{:else if current.name === "registry-token"}
					<p class="label">Create the push token</p>
					<ol class="steps">
						<li>
							The server pushes built images to <code>ghcr.io</code>, and ghcr
							accepts only a <strong>classic personal access token</strong> — no
							App-derived credential works.
						</li>
						<li>
							Create one at
							<a
								href="https://github.com/settings/tokens/new?scopes=write:packages&description=platform+publish"
								target="_blank">github.com/settings/tokens/new</a
							>
							with the single scope <code>write:packages</code> — nothing else.
						</li>
						<li>
							The token acts for whoever creates it: prefer a machine user or an
							org owner.
						</li>
						<li>Paste the token into the form and save.</li>
					</ol>
				{:else if isStep("app-installed", "fully_ready")}
					<p class="label">App installed</p>
					<p>
						This step's truth lives on GitHub — the check reads it fresh every
						load, and uninstalling the App there is what un-does it.
					</p>
				{:else if isStep("app-installed", "not_started")}
					<p class="label">Install the App</p>
					<p>
						Install the App on the managed org — the button opens
						{#if appInstallURL}
							<a href={appInstallURL} target="_blank">{appInstallURL}</a>{:else}
							<code>…/settings/apps/&lt;slug&gt;/installations</code>{/if}
						in a new tab. Keep this tab open: when the install finishes, GitHub's
						Setup URL redirect brings the browser back here on its own, and this
						step turns ready.
					</p>
				{:else if isStep("claimed", "fully_ready")}
					<p class="label">Claimed</p>
					<p>
						The org-owner claim is done. Re-org is a de-install + re-install — there
						is nothing to redo here.
					</p>
				{:else if isStep("claimed", "not_started")}
					<p class="label">Claim the installation</p>
					{#if session.user === null}
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
				{:else if isStep("migrations", "fully_ready")}
					<p class="label">Schema is current</p>
					<p>
						This step has no saved values; it re-checks the schema on every load.
					</p>
				{:else if isStep("migrations", "not_started", "partially_ready")}
					<p class="label">Run migrations</p>
					<p>
						Creates the schema every later step stores its values in. Re-runnable;
						it only ever applies what is missing.
					</p>
				{:else if current.name === "migrations"}
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
		box-shadow: 0 -1px 0 var(--border) inset;
	}

	.checklist li.active {
		box-shadow:
			2px 0 0 var(--accent-signal) inset,
			0 -1px 0 var(--border) inset;
	}

	/* The whole row is the navigation affordance — the progress list is clickable
	   by spec (§The wizard UI, progress is navigation). */
	.checklist .row {
		display: grid;
		grid-template-columns: minmax(0, 1fr) auto;
		width: 100%;
		padding: var(--lead-half) 0 var(--lead-half) var(--lead-half);
		border: 0;
		background: none;
		font: inherit;
		color: inherit;
		text-align: left;
		cursor: pointer;
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

	.steps ul {
		margin: 0;
		padding-left: var(--lead);
		list-style: square;
	}

	.steps ul li::marker {
		color: var(--text-muted);
	}

	.mark {
		color: var(--accent-signal);
		font-family: var(--p9-mono);
	}

	.failed {
		color: var(--accent-signal);
	}

	.mismatch {
		color: var(--accent-signal);
		border: 1px solid var(--accent-signal);
		border-radius: var(--radius-sm);
		padding: var(--lead-half);
		margin-bottom: var(--lead);
	}

	.mismatch code {
		overflow-wrap: anywhere;
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

	.fields input {
		padding: 0 var(--lead-half);
		border: 1px solid var(--border);
		border-radius: var(--radius-sm);
		background: var(--surface-raised);
		font-family: var(--p9-mono);
		line-height: var(--lead);
		color: var(--text);
	}

	.fields input:focus {
		outline: 2px solid var(--accent);
		outline-offset: -1px;
	}

	.fields input:disabled {
		color: var(--text-muted);
		cursor: not-allowed;
	}

	.secret {
		display: flex;
		gap: var(--lead-half);
	}

	.secret input {
		flex: 1;
		min-width: 0;
	}

	.secret button {
		padding: 0 var(--lead-half);
		border: 1px solid var(--border);
		border-radius: var(--radius-sm);
		background: var(--surface-raised);
		color: var(--text);
		cursor: pointer;
	}

	.secret button:hover:not(:disabled) {
		color: var(--accent);
	}

	.secret button:disabled {
		color: var(--text-muted);
		cursor: not-allowed;
	}
</style>
