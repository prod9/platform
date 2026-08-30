<script>
	let { current, locked, appsNewURL, base, appEditURL, appInstallURL, user } = $props();

	function isStep(name, ...states) {
		if (current === null) {
			return false;
		}
		return current.name === name && states.includes(current.state);
	}
</script>

<aside class="instructions">
	{#if current === null}
		<p class="label">Done</p>
		<p>
			Every step is ready. The server restarts itself into the product after the claim;
			the installer retires with it.
		</p>
	{:else if current.name === "db-reachable"}
		<p class="label">What this means</p>
		<p>
			{#if locked}
				The deployment's <code>DATABASE_URL</code> answers. This step has no saved
				values; it re-checks on every load.
			{:else}
				The server cannot reach its database. Fix the deployment's
				<code>DATABASE_URL</code> — this is an operator concern, not a wizard step.
			{/if}
		</p>
	{:else if current.name === "server"}
		<p class="label">Name the server</p>
		<p>
			The server's public URL — the one server-side truth of where this deployment
			lives. Login's OAuth redirect, the go-get vanity host, and every "the server's
			URL" the later steps render come from this value, not from whatever host this
			page happens to be open on.
		</p>
		<p>
			The field suggests this page's own origin; correct it if the canonical host
			differs. Redoing it resets every later step.
		</p>
	{:else if current.name === "org"}
		<p class="label">Name the primary org</p>
		<p>
			Every GitHub link the later steps render is built from this slug — the App is
			created wherever those links point, so it heads the settings-backed steps.
		</p>
		<p>
			A wrong slug simply 404s the links; redo this step to fix it. Redoing it resets
			every later step.
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
				Setup URL: <code>{base}/installation/</code>, with <strong>Redirect on
				update</strong> checked — GitHub sends the browser back here after the App is
				installed later.
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
				Subscribe to events: <strong>Push</strong>, and only Push — tag pushes
				trigger builds through it; nothing ticked means nothing ever builds.
			</li>
			<li>Where can it be installed: Only on this account.</li>
			<li>
				Paste the created App's values (its settings page, About) into the form and
				save:
				<ul>
					<li><strong>App id</strong></li>
					<li>
						<strong>App URL</strong> — paste the URL of the settings page GitHub
						lands you on after creation, straight from the address bar (the App's
						public page works too); only its slug saves, and later steps link the
						App's pages directly through it
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
					<a href={appEditURL} target="_blank">{appEditURL}</a>
				{:else}
					<code>github.com/organizations/&lt;org&gt;/settings/apps/&lt;slug&gt;</code>
				{/if}
			</li>
			<li>
				Under <em>Client secrets</em>, generate a <strong>client secret</strong>.
			</li>
			<li>
				Under <em>Private keys</em>, generate a <strong>private key</strong> — GitHub
				downloads a <code>.pem</code> file; pick that file in the form.
			</li>
			<li>Enter the secret, pick the file, and save.</li>
		</ol>
	{:else if current.name === "registry-token"}
		<p class="label">Create the push token</p>
		<ol class="steps">
			<li>
				The server pushes built images to <code>ghcr.io</code>, and ghcr accepts only
				a <strong>classic personal access token</strong> — no App-derived credential
				works.
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
				The token acts for whoever creates it: prefer a machine user or an org owner.
			</li>
			<li>Paste the token into the form and save.</li>
		</ol>
	{:else if isStep("app-installed", "fully_ready")}
		<p class="label">App installed</p>
		<p>
			This step's truth lives on GitHub — the check reads it fresh every load, and
			uninstalling the App there is what un-does it.
		</p>
	{:else if isStep("app-installed", "not_started")}
		<p class="label">Install the App</p>
		<p>
			Install the App on the managed org — the button opens
			{#if appInstallURL}
				<a href={appInstallURL} target="_blank">{appInstallURL}</a>
			{:else}
				<code>…/settings/apps/&lt;slug&gt;/installations</code>
			{/if}
			in a new tab. Keep this tab open: when the install finishes, GitHub's Setup URL
			redirect brings the browser back here on its own, and this step turns ready.
		</p>
	{:else if isStep("claimed", "fully_ready")}
		<p class="label">Claimed</p>
		<p>
			The org-owner claim is done. Re-org is a de-install + re-install — there is
			nothing to redo here.
		</p>
	{:else if isStep("claimed", "not_started")}
		<p class="label">Claim the installation</p>
		{#if user === null}
			<p>
				Sign in with a GitHub account that owns the org. That account becomes the seed
				admin.
			</p>
		{:else}
			<p>
				Claiming binds this installation to the server and marks it installed. The
				claim verifies you are an active owner of the org.
			</p>
		{/if}
	{:else if isStep("migrations", "fully_ready")}
		<p class="label">Schema is current</p>
		<p>This step has no saved values; it re-checks the schema on every load.</p>
	{:else if isStep("migrations", "not_started", "partially_ready")}
		<p class="label">Run migrations</p>
		<p>
			Creates the schema every later step stores its values in. Re-runnable; it only
			ever applies what is missing.
		</p>
	{:else if current.name === "migrations"}
		<p class="label">What this means</p>
		<p>
			The database schema diverges from what this server ships. Review the database
			by hand — the wizard will not overwrite an unknown schema.
		</p>
	{:else}
		<p class="label">What this means</p>
		<p>The check itself failed; the message on the left carries the cause.</p>
	{/if}
</aside>

<style>
	.label {
		color: var(--accent);
		margin-bottom: var(--lead);
	}

	/* Built URLs (org links, webhook/callback paths) can outgrow the column;
	   break anywhere rather than pushing the grid past the viewport. */
	a,
	code {
		overflow-wrap: anywhere;
	}

	p {
		margin: 0 0 var(--lead);
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
</style>
