<script>
	import { untrack } from "svelte";
	import {
		runMigrations,
		saveServer,
		saveOrg,
		saveApp,
		saveCredentials,
		saveRegistryToken,
		claimInstall,
		installState,
		errorText,
		installSignal,
		Answered,
		Installed,
	} from "$lib/server.js";
	import {
		stepValues,
		serverPayload,
		orgPayload,
		appPayload,
		credentialsPayload,
		registryPayload,
		generateWebhookSecret,
	} from "$lib/install.js";
	import { beginSignIn, session } from "$lib/session.svelte.js";
	import Panel from "$lib/components/Panel.svelte";
	import Button from "$lib/components/Button.svelte";

	let {
		current,
		entries,
		locked,
		origin,
		installationID,
		signInURL,
		appInstallURL,
		onconverge,
		onredo,
	} = $props();

	let migrating = $state(false);
	let migrateError = $state("");
	let server = $state({
		public_url: untrack(() => stepValues(entries, "server").public_url ?? origin),
	});
	let savingServer = $state(false);
	let serverError = $state("");
	let org = $state({
		org: untrack(() => stepValues(entries, "org").org ?? ""),
	});
	let savingOrg = $state(false);
	let orgError = $state("");
	let app = $state({
		app_id: untrack(() => stepValues(entries, "app-created").app_id ?? ""),
		app_slug: untrack(() => stepValues(entries, "app-created").app_slug ?? ""),
		client_id: untrack(() => stepValues(entries, "app-created").client_id ?? ""),
		webhook_secret: generateWebhookSecret(),
	});
	let savingApp = $state(false);
	let appError = $state("");
	let credentials = $state({ private_key: "", client_secret: "" });
	let savingCredentials = $state(false);
	let credentialsError = $state("");
	let registry = $state({ token: "" });
	let savingRegistry = $state(false);
	let registryError = $state("");
	let claiming = $state(false);
	let claimError = $state("");

	let serverReady = $derived(server.public_url.trim() !== "");
	let orgReady = $derived(org.org.trim() !== "");
	let appReady = $derived(Object.values(app).every((value) => value.trim() !== ""));
	let credentialsReady = $derived(
		Object.values(credentials).every((value) => value.trim() !== ""),
	);
	let registryReady = $derived(registry.token.trim() !== "");

	$effect(() => {
		server.public_url = stepValues(entries, "server").public_url ?? origin;
		org.org = stepValues(entries, "org").org ?? "";

		const created = stepValues(entries, "app-created");
		app.app_id = created.app_id ?? "";
		app.app_slug = created.app_slug ?? "";
		app.client_id = created.client_id ?? "";
	});

	function isStep(name, ...states) {
		return current.name === name && states.includes(current.state);
	}

	function redo() {
		onredo();
		if (current.name === "app-created") {
			app.webhook_secret = generateWebhookSecret();
		} else if (current.name === "app-credentials") {
			credentials = { private_key: "", client_secret: "" };
		} else if (current.name === "registry-token") {
			registry = { token: "" };
		}
	}

	async function migrate() {
		migrating = true;
		migrateError = "";

		const result = await runMigrations();
		if (result.outcome === Answered) {
			onconverge(result.body);
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
			onconverge(result.body);
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
			onconverge(result.body);
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
			onconverge(result.body);
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
			onconverge(result.body);
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
			onconverge(result.body);
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

	// A committed claim makes the server restart itself into the product composition.
	// Poll until the install endpoint's 404 signals the fresh product shell is ready
	// (docs/spec/installation.md §Boot composition).
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
</script>

{#if current.name === "db-reachable"}
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
				<input
					placeholder="your-org"
					bind:value={org.org}
					disabled={locked || savingOrg}
				/>
			</label>
		</div>
		{#if locked}
			<Button onclick={redo}>Redo</Button>
		{:else}
			<Button variant="primary" onclick={submitOrg} disabled={!orgReady || savingOrg}>
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
						onclick={() => (app.webhook_secret = generateWebhookSecret())}>↻</button
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
			<Button variant="primary" onclick={submitApp} disabled={!appReady || savingApp}>
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
				<input
					bind:value={credentials.client_secret}
					disabled={locked || savingCredentials}
				/>
			</label>
			<label>
				<span class="label">Private key (the downloaded .pem)</span>
				<input
					type="file"
					accept=".pem"
					onchange={pickPrivateKey}
					disabled={locked || savingCredentials}
				/>
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
				<input
					type="password"
					bind:value={registry.token}
					disabled={locked || savingRegistry}
				/>
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
			GitHub reports the App installed on the org. Undoing this is uninstalling it on
			GitHub — nothing to redo here.
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
				The install link needs the org and App slugs — redo the org and App steps if they
				are missing.
			</p>
		{/if}
	</Panel>
{:else if isStep("claimed", "fully_ready")}
	<Panel label="Claimed">
		<p class="muted">
			The installation is bound to this server; it restarts itself into the product.
			Reload this page if it lingers here.
		</p>
	</Panel>
{:else if isStep("claimed", "not_started")}
	{#if session.user === null}
		<Panel label="Claim the installation">
			<Button
				variant="primary"
				href={signInURL}
				onclick={beginSignIn}
				disabled={session.signingIn}
			>
				{session.signingIn ? "Signing in…" : "Sign in with GitHub"}
			</Button>
		</Panel>
	{:else if !installationID}
		<Panel label="Claim the installation">
			<p class="muted">
				The claim needs GitHub's <code>installation_id</code>, which arrives on the Setup
				URL redirect. Open the installed App's page and save its repository selection to
				fire the redirect again.
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
				Bind installation <span class="mark">#{installationID}</span> to this server as
				{session.user.name}.
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

<style>
	.mark,
	.failed {
		color: var(--accent-signal);
	}

	.mark {
		font-family: var(--p9-mono);
	}

	.fields {
		display: grid;
		gap: var(--lead);
		margin: var(--lead) 0;
	}

	.fields label {
		display: grid;
		gap: 0;
	}

	.fields input {
		padding: calc(var(--lead-half) - var(--plate-edge)) var(--lead-half);
		border: var(--plate-edge) solid var(--border);
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
		column-gap: var(--lead-half);
	}

	.secret input {
		flex: 1;
		min-width: 0;
	}

	.secret button {
		padding: calc(var(--lead-half) - var(--plate-edge)) var(--lead-half);
		border: var(--plate-edge) solid var(--border);
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
