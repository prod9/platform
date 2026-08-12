<script>
	// The install-time facts, read-only, secrets shown as presence only. Whether editing
	// (re-using the installer actions) belongs in v1 is an open question for the walk.
	import Panel from "$lib/components/Panel.svelte";
</script>

<section>
	<div class="head">
		<h2>Settings</h2>
	</div>

	<div class="stack">
		<Panel label="Server">
			<dl class="kv">
				<dt class="label">Public URL</dt>
				<dd class="mono">https://platform.prodigy9.co</dd>
				<dt class="label">Organization</dt>
				<dd class="mono">prod9</dd>
			</dl>
		</Panel>

		<Panel label="GitHub App">
			<dl class="kv">
				<dt class="label">App</dt>
				<dd class="mono">prod9-platform (id 412887)</dd>
				<dt class="label">Client ID</dt>
				<dd class="mono">Iv1.8a61f9b3a7aba766</dd>
				<dt class="label">Private key</dt>
				<dd class="mono">✓ saved</dd>
				<dt class="label">Webhook secret</dt>
				<dd class="mono">✓ saved</dd>
				<dt class="label">Client secret</dt>
				<dd class="mono">✓ saved</dd>
			</dl>
		</Panel>

		<Panel label="Registry">
			<dl class="kv">
				<dt class="label">ghcr.io token</dt>
				<dd class="mono">✓ saved · pushes as chakrit</dd>
			</dl>
		</Panel>
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

	.stack {
		display: grid;
		gap: var(--lead);
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
</style>
