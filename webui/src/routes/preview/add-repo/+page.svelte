<script>
	// The add-repo wizard, mid-flight on step 2: the repo is picked, the server has
	// pre-read what it can reach, and the confirm registers the row. When the App does not
	// reach the repo yet, the confirm becomes the GitHub App-install link instead and
	// registration completes on the setup redirect.
	import Button from "$lib/components/Button.svelte";
	import Panel from "$lib/components/Panel.svelte";
</script>

<section>
	<div class="head">
		<h2>Add repository</h2>
		<span class="spacer"></span>
		<Button href="/preview/repos/">Cancel</Button>
	</div>

	<ol class="wsteps">
		<li class="done">Pick the repository</li>
		<li class="now">Review what the server found</li>
		<li class="todo">Confirm</li>
	</ol>

	<div class="stack">
		<Panel label="Repository">
			<select class="mono">
				<option selected>prod9/haachang</option>
				<option>prod9/bluepages</option>
				<option>prod9/naxon-api</option>
			</select>
		</Panel>

		<Panel label="What the server found">
			<dl class="kv">
				<dt class="mono key">app reach</dt>
				<dd class="mono ok">✓ installed on this repo</dd>
				<dt class="mono key">platform.toml</dt>
				<dd class="mono ok">✓ present on <span class="muted">refs/heads/main</span></dd>
				<dt class="mono key">maintainer</dt>
				<dd class="mono">chakrit &lt;chakrit@prodigy9.co&gt;</dd>
				<dt class="mono key">modules</dt>
				<dd class="mono">api (go/basic) · web (pnpm/static)</dd>
				<dt class="mono key">latest tag</dt>
				<dd class="mono">v0.4.2</dd>
			</dl>
		</Panel>

		<div class="confirm">
			<span class="mono">prod9/haachang · builds on <span class="muted">refs/tags/v*</span></span>
			<Button variant="primary" href="/preview/repos/">Confirm add</Button>
		</div>
	</div>
</section>

<style>
	section {
		max-width: 90ch;
	}

	.head {
		display: flex;
		align-items: baseline;
		gap: var(--lead);
		margin-bottom: var(--lead);
	}

	.spacer {
		margin-left: auto;
	}

	.wsteps {
		list-style: none;
		margin: 0 0 var(--lead);
		padding: 0;
	}

	.wsteps li {
		line-height: var(--lead);
	}

	.wsteps .done::before {
		content: "✓ ";
	}

	.wsteps .now::before {
		content: "→ ";
		color: var(--accent-signal);
	}

	.wsteps .todo {
		color: var(--text-muted);
	}

	.wsteps .todo::before {
		content: "· ";
	}

	.stack {
		display: grid;
		gap: var(--lead);
	}

	select {
		padding: 0 var(--lead-half);
		border: 1px solid var(--border);
		border-radius: var(--radius-sm);
		background: var(--surface-raised);
		line-height: var(--lead);
		color: var(--text);
	}

	.confirm {
		display: flex;
		align-items: baseline;
		justify-content: space-between;
		gap: var(--lead);
		padding: var(--lead-half) 0;
		box-shadow: 0 1px 0 var(--border) inset;
	}

	.kv {
		display: grid;
		grid-template-columns: 18ch minmax(0, 1fr);
		gap: 0 var(--lead);
		margin: 0;
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
</style>
