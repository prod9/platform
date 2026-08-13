<script>
	// The repo onboarding wizard, run as the install wizard is: the checklist on the
	// left is the navigation, the selected step's action renders beside it. Step one
	// picks from the live candidate list; step two reviews what the server pre-read
	// from the repo's platform.toml and carries the confirm (docs/spec/webui.md).
	import { goto } from "$app/navigation";
	import {
		listCandidates,
		getManifest,
		registerRepo,
		errorText,
		Answered,
		Refused,
	} from "$lib/server.js";
	import { filterCandidates, moduleLine } from "$lib/repos.js";
	import Button from "$lib/components/Button.svelte";
	import Panel from "$lib/components/Panel.svelte";

	let candidates = $state([]);
	let loaded = $state(false);
	let loadError = $state("");

	let picked = $state(null);
	let filter = $state("");
	let matches = $derived(filterCandidates(candidates, filter));

	// The manifest pre-read: null until answered; a 404 is a repo with no
	// platform.toml — reviewable, with the absence stated — any other refusal is an
	// error the panel surfaces.
	let manifest = $state(null);
	let manifestAbsent = $state(false);
	let manifestError = $state("");

	let confirming = $state(false);
	let confirmError = $state("");

	const steps = [
		{ name: "pick-repo", label: "Pick the repository" },
		{ name: "review", label: "Review & confirm" },
	];

	let current = $derived(picked === null ? "pick-repo" : "review");

	// Review never reads done: confirming leaves the wizard, so its pending state is
	// the only one this page ever shows.
	function stateOf(name) {
		if (name === "pick-repo") {
			return picked === null ? "not_started" : "fully_ready";
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

		const result = await getManifest(repo.owner, repo.repo);
		if (result.outcome === Answered) {
			manifest = result.body;
		} else if (result.outcome === Refused && result.status === 404) {
			manifestAbsent = true;
		} else {
			manifestError = errorText(result);
		}
	}

	function back() {
		picked = null;
		confirmError = "";
	}

	async function confirm() {
		confirming = true;
		confirmError = "";

		try {
			const result = await registerRepo(picked.owner, picked.repo);
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
		<h2><a href="/">Repositories</a> / add</h2>
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
							{stateOf(step.name) === "fully_ready" ? "done" : "pending"}
						</span>
					</span>
				</li>
			{/each}
		</ol>

		<div class="action">
			{#if current === "pick-repo"}
				<Panel label="Repositories the App reaches, not yet onboarded">
					{#if !loaded}
						<p class="mono muted">Loading…</p>
					{:else if loadError}
						<p class="mono warn">Candidates unavailable: {loadError}</p>
					{:else if candidates.length === 0}
						<p class="mono muted">
							Every repository the App reaches is already onboarded.
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
			{:else}
				<Panel label={picked.full_name}>
					<dl class="kv">
						<dt class="mono key">platform.toml</dt>
						{#if manifest !== null}
							<dd class="mono ok">✓ present on the default branch</dd>
							<dt class="mono key">modules</dt>
							<dd class="mono">{moduleLine(manifest.modules)}</dd>
							{#if manifest.maintainer !== ""}
								<dt class="mono key">maintainer</dt>
								<dd class="mono">{manifest.maintainer}</dd>
							{/if}
							{#if manifest.repository !== ""}
								<dt class="mono key">repository</dt>
								<dd class="mono">{manifest.repository}</dd>
							{/if}
						{:else if manifestAbsent}
							<dd class="mono warn">
								✗ not found — builds will fail until one is committed
							</dd>
						{:else if manifestError !== ""}
							<dd class="mono warn">{manifestError}</dd>
						{:else}
							<dd class="mono muted">Reading…</dd>
						{/if}
						<dt class="mono key">builds on</dt>
						<dd class="mono">refs/tags/v*</dd>
					</dl>

					{#if confirmError !== ""}
						<p class="mono warn">{confirmError}</p>
					{/if}
					<div class="confirm">
						<Button onclick={back}>Back</Button>
						<Button variant="primary" onclick={confirm} disabled={confirming}>
							{confirming ? "Adding…" : "Confirm add"}
						</Button>
					</div>
				</Panel>
			{/if}
		</div>
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
		grid-template-columns: minmax(26ch, 1fr) minmax(0, 3fr);
		gap: var(--lead-2);
		align-items: start;
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
</style>
