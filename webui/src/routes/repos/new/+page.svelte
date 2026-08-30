<script>
	// The repo onboarding wizard separates access, manifest loading, and review. The
	// checklist navigates on the left, the action sits in the middle, and operative
	// instructions sit on the right (docs/spec/webui.md).
	import { goto } from "$app/navigation";
	import {
		listCandidates,
		getManifest,
		registerRepo,
		errorText,
		Answered,
		Refused,
	} from "$lib/server.js";
	import { filterCandidates, moduleLabel, publishPolicyDetails } from "$lib/repos.js";
	import Button from "$lib/components/Button.svelte";
	import Panel from "$lib/components/Panel.svelte";

	let candidates = $state([]);
	let loaded = $state(false);
	let loadError = $state("");

	let picked = $state(null);
	let filter = $state("");
	let matches = $derived(filterCandidates(candidates, filter));

	// The manifest pre-read: null until answered; a 409 is a repo with no
	// platform.toml — recoverable after setup — any other refusal is an error the
	// panel surfaces.
	let manifest = $state(null);
	let manifestAbsent = $state(false);
	let manifestError = $state("");
	let manifestLoading = $state(false);
	let reviewing = $state(false);

	let confirming = $state(false);
	let confirmError = $state("");

	const steps = [
		{ name: "access", label: "Repository access" },
		{ name: "manifest", label: "Load platform.toml" },
		{ name: "review", label: "Review & confirm" },
	];

	let current = $derived(picked === null ? "access" : reviewing ? "review" : "manifest");
	let policy = $derived(
		manifest === null ? null : publishPolicyDetails(manifest.publish_policy),
	);

	function stateOf(name) {
		if (name === "access") {
			return picked === null ? "not_started" : "fully_ready";
		}
		if (name === "manifest") {
			return reviewing ? "fully_ready" : "not_started";
		}
		return "not_started";
	}

	async function load() {
		const result = await listCandidates();
		if (result.outcome === Answered) {
			candidates = result.body;
			loadError = "";
		} else {
			loadError = errorText(result);
		}
		loaded = true;
	}

	async function pick(repo) {
		picked = repo;
		manifest = null;
		manifestAbsent = false;
		manifestError = "";
		reviewing = false;
		await loadManifest();
	}

	async function loadManifest() {
		manifest = null;
		manifestAbsent = false;
		manifestError = "";
		manifestLoading = true;

		try {
			const result = await getManifest(picked.owner, picked.repo);
			if (result.outcome === Answered) {
				manifest = result.body;
			} else if (result.outcome === Refused && result.status === 409) {
				manifestAbsent = true;
			} else {
				manifestError = errorText(result);
			}
		} finally {
			manifestLoading = false;
		}
	}

	function back() {
		if (reviewing) {
			reviewing = false;
			return;
		}
		picked = null;
		confirmError = "";
	}

	function review() {
		reviewing = true;
	}

	async function confirm() {
		confirming = true;
		confirmError = "";

		try {
			const result = await registerRepo(picked.owner, picked.repo, manifest.sha);
			if (result.outcome === Answered) {
				await goto("/");
			} else {
				confirmError = errorText(result);
			}
		} finally {
			confirming = false;
		}
	}

	$effect(() => {
		load();
	});
</script>

<section>
	<div class="head">
		<h2><a href="/">Repositories</a> / register</h2>
		<span class="spacer"></span>
		<Button href="/">Cancel</Button>
	</div>

	<div class="wizard">
		<ol class="checklist">
			{#each steps as step (step.name)}
				<li class:active={step.name === current}>
					<span class="row">
						<span class="mono name">{step.label}</span>
						<span class="state state--{stateOf(step.name)} label">
							{stateOf(step.name)}
						</span>
					</span>
				</li>
			{/each}
		</ol>

		<div class="action">
			{#if current === "access"}
				<Panel label="Repositories you and the App reach, not yet registered">
					{#if !loaded}
						<p class="mono muted">Loading…</p>
					{:else if loadError}
						<p class="mono warn">Candidates unavailable: {loadError}</p>
					{:else if candidates.length === 0}
						<p class="mono muted">
							Every repository you and the App both reach is already registered.
						</p>
					{:else}
						<input
							class="mono filter"
							type="search"
							placeholder="Filter repositories…"
							bind:value={filter}
						/>
						<ul class="candidates">
							{#each matches as repo (repo.full_name)}
								<li>
									<button class="candidate" onclick={() => pick(repo)}>
										<span class="mono repo-name">{repo.full_name}</span>
										<span class="mono chev">›</span>
									</button>
								</li>
							{:else}
								<li class="mono muted empty">Nothing matches “{filter}”.</li>
							{/each}
						</ul>
					{/if}
				</Panel>
			{:else if current === "manifest"}
				<Panel label={`Load ${picked.full_name}/platform.toml`}>
					<dl class="kv">
						<dt class="mono key">status</dt>
						{#if manifest !== null}
							<dd class="mono ok">✓ loaded from the default branch</dd>
						{:else if manifestAbsent}
							<dd class="mono warn">✗ platform.toml is not committed</dd>
						{:else if manifestError !== ""}
							<dd class="mono warn">{manifestError}</dd>
						{:else if manifestLoading}
							<dd class="mono muted">Reading…</dd>
						{/if}
					</dl>

					{#if manifest !== null}
						<pre class="mono manifest"><code>{manifest.raw}</code></pre>
					{/if}

					<div class="confirm">
						<Button onclick={back}>Back</Button>
						{#if manifest === null}
							<Button variant="primary" onclick={loadManifest} disabled={manifestLoading}>
								{manifestLoading ? "Reading…" : "Retry load"}
							</Button>
						{:else}
							<Button variant="primary" onclick={review}>Review configuration</Button>
						{/if}
					</div>
				</Panel>
			{:else}
				<Panel label={picked.full_name}>
					<dl class="kv">
						<dt class="mono key">Commit</dt>
						<dd class="mono">{manifest.sha}</dd>
						<dt class="mono key">Modules</dt>
						<dd class="mono modules">
							{#each manifest.modules as module (module.name)}
								<span>{moduleLabel(module)}</span>
							{/each}
						</dd>
						{#if manifest.maintainer !== ""}
							<dt class="mono key">Maintainer</dt>
							<dd class="mono">{manifest.maintainer}</dd>
						{/if}
						{#if manifest.repository !== ""}
							<dt class="mono key">Repository</dt>
							<dd class="mono">{manifest.repository}</dd>
						{/if}
						<dt class="mono key">Publish policy</dt>
						<dd class="mono">
							{policy.policy}
							{#if policy.hint}<span class="muted"> ({policy.hint})</span>{/if}
						</dd>
					</dl>

					{#if confirmError !== ""}
						<p class="mono warn">{confirmError}</p>
					{/if}
					<div class="confirm">
						<Button onclick={back}>Back</Button>
						<Button
							variant="primary"
							onclick={confirm}
							disabled={confirming || manifest === null}
						>
							{confirming ? "Registering…" : "Register repository"}
						</Button>
					</div>
				</Panel>
			{/if}
		</div>

		<aside class="instructions">
			{#if current === "access"}
				<p class="label">Repository access</p>
				<p>
					A repository appears only when both your signed-in GitHub account and the
					installed GitHub App can reach it.
				</p>
				<p>
					If it is missing, first verify that you can open the repository on GitHub.
					Then open the organization’s Settings → GitHub Apps, configure the installed
					App, and include the repository in its repository access.
				</p>
			{:else if current === "manifest"}
				<p class="label">Set up platform.toml</p>
				<p>From the repository root, let platform detect the project and write the file:</p>
				<pre class="mono"><code>platform init</code></pre>
				<p>Review the generated files, then commit and push <code>platform.toml</code>.</p>
				<p>
					For manual authoring, set the scheme-less GitHub repository and declare each
					buildable module with its platform framework:
				</p>
				<pre class="mono"><code>repository = "github.com/{picked.full_name}"

[modules.app]
framework = "go/basic"</code></pre>
				<p>Replace the example module and framework with the repository’s actual layout.</p>
			{:else}
				<p class="label">Review the observation</p>
				<p>
					The commit identifies the exact <code>platform.toml</code> shown here. Confirming
					registers configuration from that commit even if the default branch moves.
				</p>
				<p>
					Publish policy controls which successful server builds produce images and which
					registry tag those images receive.
				</p>
			{/if}
		</aside>
	</div>
</section>

<style>
	.head {
		display: flex;
		align-items: baseline;
		gap: var(--lead);
		margin-bottom: var(--lead);
	}

	.head h2 a {
		text-decoration: none;
	}

	.spacer {
		margin-left: auto;
	}

	.wizard {
		display: grid;
		grid-template-columns: minmax(24ch, 1fr) minmax(0, 2fr) minmax(28ch, 1fr);
		gap: var(--lead-2);
		align-items: start;
	}

	.instructions {
		padding-left: var(--lead);
		border-left: 1px solid var(--border);
	}

	.instructions p {
		margin: 0 0 var(--lead-half);
	}

	.instructions pre {
		overflow-x: auto;
		margin: 0 0 var(--lead-half);
		padding: var(--lead-half);
		border: 1px solid var(--border);
		border-radius: var(--radius-sm);
		background: var(--surface-quiet);
		line-height: var(--lead);
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

	.checklist .row {
		display: grid;
		grid-template-columns: minmax(0, 1fr) auto;
		padding: var(--lead-half) 0 var(--lead-half) var(--lead-half);
	}

	.checklist .name,
	.checklist .state {
		line-height: var(--lead);
	}

	.checklist li.active .name {
		color: var(--accent);
		font-weight: 600;
	}

	.state--fully_ready {
		color: var(--accent-ok);
	}

	.state--not_started {
		color: var(--text-muted);
	}

	.filter {
		width: 100%;
		padding: 0 var(--lead-half);
		border: 1px solid var(--border);
		border-radius: var(--radius-sm);
		background: var(--surface-raised);
		line-height: var(--lead);
		color: var(--text);
	}

	.candidates {
		list-style: none;
		margin: 0;
		padding: 0;
	}

	.empty {
		padding: var(--lead-half) 0;
	}

	.candidate {
		display: grid;
		grid-template-columns: 1fr var(--lead);
		align-items: baseline;
		gap: var(--lead-half);
		width: 100%;
		padding: var(--lead-half) 0;
		border: 0;
		background: none;
		text-align: left;
		line-height: var(--lead);
		box-shadow: 0 -1px 0 var(--border) inset;
		cursor: pointer;
	}

	.candidate:hover {
		background: var(--surface-quiet);
	}

	.repo-name {
		font-size: var(--size-prose);
		font-weight: 600;
		color: var(--accent);
	}

	.candidate:hover .repo-name {
		color: var(--accent-signal);
	}

	.chev {
		color: var(--text-muted);
		text-align: center;
	}

	.kv {
		display: grid;
		grid-template-columns: 18ch minmax(0, 1fr);
		gap: 0 var(--lead);
		margin: 0 0 var(--lead);
	}

	.kv dt,
	.kv dd {
		margin: 0;
		line-height: var(--lead);
	}

	.manifest {
		overflow-x: auto;
		margin: 0 0 var(--lead);
		padding: var(--lead-half);
		border: 1px solid var(--border);
		border-radius: var(--radius-sm);
		background: var(--surface-quiet);
		line-height: var(--lead);
	}

	.modules {
		display: grid;
	}

	.key {
		color: var(--text-muted);
	}

	.ok {
		color: var(--accent-ok);
	}

	.warn {
		color: var(--accent-signal);
	}

	.confirm {
		display: flex;
		justify-content: space-between;
		gap: var(--lead);
	}

	@media (max-width: 70rem) {
		.wizard {
			grid-template-columns: minmax(22ch, 1fr) minmax(0, 2fr);
		}

		.instructions {
			grid-column: 2;
			padding-left: 0;
			border-left: 0;
		}
	}

	@media (max-width: 48rem) {
		.wizard {
			grid-template-columns: 1fr;
		}

		.instructions {
			grid-column: 1;
		}
	}
</style>
