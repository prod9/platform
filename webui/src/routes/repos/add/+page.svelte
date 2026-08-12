<script>
	// ⚠ MOCK — canned data promoted from /preview; before the real implementation
	// locks in: graduate the design into docs/spec, wire the real reads, extract shared
	// components (outcome mark, feed row, kv list, terminal pane), delete canned data.
	// Onboarding a repo runs as the install wizard does: a checklist on the left is the
	// navigation, the selected step's action renders beside it. Step one picks the repo
	// from a clickable list; step two reviews what the server pre-read and carries the
	// confirm.
	import Button from "$lib/components/Button.svelte";
	import Panel from "$lib/components/Panel.svelte";

	const candidates = [
		{
			full: "prod9/haachang",
			meta: "private · Go · pushed 2d ago",
			toml: true,
			modules: "api (go/basic) · web (pnpm/static)",
			maintainer: "chakrit <chakrit@prodigy9.co>",
			latest: "v0.4.2",
		},
		{
			full: "prod9/bluepages",
			meta: "private · Go · pushed 6d ago",
			toml: true,
			modules: "bluepages (go/basic)",
			maintainer: "chakrit <chakrit@prodigy9.co>",
			latest: "v1.1.0",
		},
		{
			full: "prod9/naxon-api",
			meta: "private · Go · pushed 3w ago",
			toml: false,
			modules: "",
			maintainer: "",
			latest: "",
		},
	];

	let picked = $state(null);
	let confirmed = $state(false);
	let filter = $state("");

	let matches = $derived(
		candidates.filter((repo) =>
			repo.full.toLowerCase().includes(filter.trim().toLowerCase()),
		),
	);

	const steps = [
		{ name: "pick-repo", label: "Pick the repository" },
		{ name: "review", label: "Review & confirm" },
	];

	let current = $derived(picked === null ? "pick-repo" : "review");

	function stateOf(step) {
		if (step === "pick-repo") {
			return picked === null ? "not_started" : "fully_ready";
		}
		return confirmed ? "fully_ready" : "not_started";
	}

	function pick(repo) {
		picked = repo;
	}

	function back() {
		picked = null;
	}
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
					<input
						class="mono filter"
						type="search"
						placeholder="Filter repositories…"
						bind:value={filter}
					/>
					<ul class="candidates">
						{#each matches as repo (repo.full)}
							<li>
								<button class="candidate" onclick={() => pick(repo)}>
									<span class="mono repo-name">{repo.full}</span>
									<span class="mono muted">{repo.meta}</span>
									<span class="mono chev">›</span>
								</button>
							</li>
						{:else}
							<li class="mono muted empty">Nothing matches “{filter}”.</li>
						{/each}
					</ul>
				</Panel>
			{:else}
				<Panel label={picked.full}>
					<dl class="kv">
						<dt class="mono key">platform.toml</dt>
						{#if picked.toml}
							<dd class="mono ok">✓ present on refs/heads/main</dd>
						{:else}
							<dd class="mono warn">✗ not found — builds will fail until one is committed</dd>
						{/if}
						{#if picked.toml}
							<dt class="mono key">modules</dt>
							<dd class="mono">{picked.modules}</dd>
							<dt class="mono key">maintainer</dt>
							<dd class="mono">{picked.maintainer}</dd>
							<dt class="mono key">latest tag</dt>
							<dd class="mono">{picked.latest}</dd>
						{/if}
						<dt class="mono key">builds on</dt>
						<dd class="mono">refs/tags/v*</dd>
					</dl>

					<div class="confirm">
						<Button onclick={back}>Back</Button>
						<Button variant="primary" href="/">Confirm add</Button>
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

	.filter:focus {
		outline: 2px solid var(--accent);
		outline-offset: -1px;
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
		grid-template-columns: auto 1fr var(--lead);
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
