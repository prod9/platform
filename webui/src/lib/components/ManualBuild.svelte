<script>
	import "$lib/styles/wizard.css";
	import { onDestroy, untrack } from "svelte";
	import { goto } from "$app/navigation";
	import { session } from "$lib/session.svelte.js";
	import { BuildRequest } from "$lib/build-request.svelte.js";
	import { moduleLabel, publishPolicyDetails } from "$lib/repos.js";
	import Button from "./Button.svelte";
	import Facts from "./Facts.svelte";
	import LoadingBlock from "./LoadingBlock.svelte";
	import PageHeader from "./PageHeader.svelte";
	import Panel from "./Panel.svelte";

	let { owner, repo, search = "" } = $props();
	// The keyed repository route gives this request owner one repository lifetime.
	const request = untrack(() => new BuildRequest(owner, repo, search));
	let repositoryURL = $derived(`/repos/${owner}/${repo}/`);
	let state = $derived(request.state);
	let source = $derived(request.source);
	let ready = $derived(session.ready && session.user !== null);
	let manifest = $derived(
		state.kind === "reviewed" || state.kind === "queueing" ? state.manifest : null,
	);
	let policy = $derived(manifest === null ? null : publishPolicyDetails(manifest.publish_policy));
	let canEdit = $derived(ready && !request.busy && state.kind !== "accepted");
	let stage = $derived(state.kind === "queueing" ? "Queue build" : manifest === null ? "Resolve source" : "Select modules");
	let predecessor = $derived(source.kind === "linked" ? source.predecessor : null);

	$effect(() => {
		if (ready) untrack(() => request.initialize());
	});

	async function load(event) {
		event.preventDefault();
		if (ready) await request.load();
	}

	async function queue() {
		if (!ready) return;
		const id = await request.queue();
		if (id !== 0) await goto(`/builds/${id}/`);
	}

	onDestroy(() => request.dispose());
</script>

<section aria-busy={!ready || request.busy}>
	<PageHeader>
		{#snippet title()}<h2><a href={repositoryURL}>{owner}/{repo}</a> / new build</h2>{/snippet}
		{#snippet actions()}
			{#if source.kind !== "manual"}<Button href={`${repositoryURL}builds/new/`}>New build</Button>{/if}
			<Button href={repositoryURL}>Cancel</Button>
		{/snippet}
	</PageHeader>

	<div class="wizard">
		<ol class="checklist">
			{#each ["Resolve source", "Select modules", "Queue build"] as label (label)}
				<li class:active={label === stage}>
					<span class="row"><span class="name">{label}</span></span>
				</li>
			{/each}
		</ol>

		<div class="action stack">
			{#if predecessor !== null}
				<Panel label="Retry">
					<p class="mono">Retry of <a href={`/builds/${predecessor.id}/`}>#{predecessor.id}</a>.</p>
					<p>The same ref is resolved again. This creates a new build without replaying the old SHA or cancelling the predecessor.</p>
				</Panel>
			{:else if source.kind === "retry"}
				<Panel label="Retry predecessor">
					{#if request.busy || !ready}
						<LoadingBlock measure="standard" />
					{:else}
						<Button onclick={() => request.initialize()}>Load predecessor</Button>
					{/if}
				</Panel>
			{/if}
			<Panel label="Ref">
				<form onsubmit={load}>
					<label class="mono" for="manual-build-ref">Ref</label>
					<input
						id="manual-build-ref"
						class="mono"
						value={state.ref}
						oninput={(event) => request.setRef(event.currentTarget.value)}
						placeholder="refs/heads/main"
						required
						disabled={!canEdit || source.kind !== "manual"}
					/>
					<Button variant="primary" disabled={!ready || !request.canLoad}>
						{state.kind === "loading" ? "Loading…" : "Load source"}
					</Button>
				</form>
			</Panel>

			{#if state.message}
				<p class="error mono" role="alert">{state.message}</p>
			{/if}

			<Panel label="Review source and modules">
				{#if manifest !== null}
					<Facts measure="standard">
						<dt class="mono">Ref</dt><dd class="mono">{state.ref}</dd>
						<dt class="mono">SHA</dt><dd class="mono">{manifest.sha}</dd>
						<dt class="mono">Publish policy</dt>
						<dd class="mono">{policy.policy}{#if policy.hint} <span class="muted">({policy.hint})</span>{/if}</dd>
					</Facts>
					{#if state.removed.length > 0}
						<p class="mono">Predecessor modules absent from this commit: {state.removed.join(", ")}.</p>
					{/if}
					{#if manifest.modules.length === 0}
						<p class="mono">No modules are defined at this commit. A build cannot be queued.</p>
					{:else}
						<fieldset disabled={!canEdit}>
							<legend class="mono">Modules</legend>
							{#each manifest.modules as module (module.name)}
								<label class="mono">
									<input type="checkbox" checked={state.selected.includes(module.name)}
										onchange={(event) => request.setSelected(module.name, event.currentTarget.checked)} />
									{moduleLabel(module)}
								</label>
							{/each}
						</fieldset>
						{#if state.selected.length === 0}<p class="mono">Select at least one module.</p>{/if}
					{/if}
				{:else if !ready || state.kind === "loading"}
					<LoadingBlock measure="standard" />
					<LoadingBlock />
				{:else if state.kind === "accepted"}
					<p class="mono" role="status">Build <a href={`/builds/${state.build.id}/`}>#{state.build.id}</a> accepted.</p>
				{:else}
					<p class="muted">Load a ref to review its committed source and choose modules.</p>
				{/if}
				<Button variant="primary" onclick={queue} disabled={!ready || !request.canQueue}>
					{state.kind === "queueing" ? "Queueing…" : "Queue build"}
				</Button>
			</Panel>
		</div>

		<aside class="instructions">
			<Panel label="platform.toml">
				{#if manifest !== null}
					<pre class="manifest mono"><code>{manifest.raw}</code></pre>
				{:else if !ready || state.kind === "loading"}
					<div aria-label="Loading platform.toml"><LoadingBlock /><LoadingBlock /><LoadingBlock /></div>
				{:else}
					<p class="muted">The exact manifest at the resolved commit appears here.</p>
				{/if}
			</Panel>
		</aside>
	</div>
</section>

<style>
	.stack,
	form {
		display: grid;
		gap: var(--space-6);
	}

	fieldset {
		min-width: 0;
		margin: var(--space-6) 0;
		padding: 0;
		border: 0;
	}

	.manifest {
		margin: 0;
		white-space: pre-wrap;
		overflow-wrap: anywhere;
	}

	.error {
		color: var(--error-ink);
		white-space: pre-wrap;
	}
</style>
