<script>
	import "$lib/styles/wizard.css";
	// The repo onboarding wizard separates access, manifest loading, and review. The
	// checklist navigates on the left, the action sits in the middle, and operative
	// instructions sit on the right (docs/spec/webui.md).
	import { goto } from "$app/navigation";
	import { session } from "$lib/session.svelte.js";
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
	import Facts from "$lib/components/Facts.svelte";
	import LoadingBlock from "$lib/components/LoadingBlock.svelte";
	import PageHeader from "$lib/components/PageHeader.svelte";
	import Panel from "$lib/components/Panel.svelte";
	import SignalMark from "$lib/components/SignalMark.svelte";

	const loadingCandidates = Array.from({ length: 3 });

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
		if (session.ready) {
			load();
		}
	});
</script>

<section>
	<PageHeader>
		{#snippet title()}<h2><a href="/">Repositories</a> / register</h2>{/snippet}
		{#snippet actions()}<Button href="/">Cancel</Button>{/snippet}
	</PageHeader>

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
						<div class="candidate-loading" aria-busy="true">
							<input
								class="mono filter"
								type="search"
								placeholder="Filter repositories…"
								disabled
							/>
							<ul class="candidates">
								{#each loadingCandidates as _}
									<li>
										<button class="candidate" disabled aria-label="Loading repository">
											<LoadingBlock />
											<SignalMark signal="navigation" />
										</button>
									</li>
								{/each}
							</ul>
						</div>
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
										<SignalMark signal="navigation" />
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
					<Facts measure="standard">
						<dt class="mono">status</dt>
						{#if manifest !== null}
							<dd class="mono ok">
								<SignalMark signal="positive" /> loaded from the default branch
							</dd>
						{:else if manifestAbsent}
							<dd class="mono warn">
								<SignalMark signal="negative" /> platform.toml is not committed
							</dd>
						{:else if manifestError !== ""}
							<dd class="mono warn">{manifestError}</dd>
						{:else if manifestLoading}
							<dd class="mono muted">Reading…</dd>
						{/if}
					</Facts>

					{#if manifest !== null}
						<pre class="mono manifest"><code>{manifest.raw}</code></pre>
					{:else if manifestLoading}
						<div class="mono manifest" aria-busy="true" aria-label="Loading platform.toml">
							<LoadingBlock measure="standard" />
							<LoadingBlock />
							<LoadingBlock />
						</div>
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
					<Facts measure="standard">
						<dt class="mono">Commit</dt>
						<dd class="mono">{manifest.sha}</dd>
						<dt class="mono">Modules</dt>
						<dd class="mono modules">
							{#each manifest.modules as module (module.name)}
								<span>{moduleLabel(module)}</span>
							{/each}
						</dd>
						{#if manifest.maintainer !== ""}
							<dt class="mono">Maintainer</dt>
							<dd class="mono">{manifest.maintainer}</dd>
						{/if}
						{#if manifest.repository !== ""}
							<dt class="mono">Repository</dt>
							<dd class="mono">{manifest.repository}</dd>
						{/if}
						<dt class="mono">Publish policy</dt>
						<dd class="mono">
							{policy.policy}
							{#if policy.hint}<span class="muted"> ({policy.hint})</span>{/if}
						</dd>
					</Facts>

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
	.filter {
		width: 100%;
	}

	.candidates {
		list-style: none;
		margin: 0;
		padding: 0;
	}

	.empty {
		padding: var(--space-4) 0;
	}

	.candidate-loading {
		display: grid;
		gap: var(--space-6);
	}

	.candidate {
		display: grid;
		grid-template-columns: minmax(0, 1fr) var(--space-6);
		align-items: baseline;
		gap: var(--space-3);
		width: 100%;
		padding: var(--space-4) var(--space-3);
		border: 0;
		background: transparent;
		text-align: left;
		box-shadow: inset 0 -1px 0 var(--hairline);
		cursor: pointer;
	}

	.candidate:enabled:hover {
		background: var(--soft);
	}

	.candidate:enabled:active {
		box-shadow: inset 0 3px 0 var(--hairline);
	}

	.repo-name {
		font-family: var(--face-display);
		font-size: var(--step-0);
		font-weight: 800;
		color: var(--indigo);
		overflow-wrap: anywhere;
	}

	.candidate:enabled:hover .repo-name {
		color: var(--error-ink);
	}

	.candidate:disabled {
		cursor: default;
	}

	.manifest {
		min-width: 0;
		max-width: 100%;
		margin: 0;
		padding: var(--space-6);
		overflow-x: auto;
		background: var(--night);
		color: var(--paper);
		box-shadow: 0 3px 0 var(--slab-edge);
	}

	.modules {
		display: grid;
	}

	.ok {
		color: var(--ink);
	}

	.warn {
		color: var(--error-ink);
	}

	.confirm {
		display: flex;
		justify-content: space-between;
		flex-wrap: wrap;
		gap: var(--space-6);
	}
</style>
