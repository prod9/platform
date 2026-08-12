<script>
	// The engine roster as the server sees it this instant: the DAGGER_ENGINE seed and
	// what DNS resolves right now. Momentary by design — two loads a second apart may
	// legitimately differ. The second card shows the empty-roster warning variant.
	import Panel from "$lib/components/Panel.svelte";
</script>

<section>
	<div class="head">
		<h2>Engines</h2>
		<p class="label">resolved just now</p>
	</div>

	<div class="stack">
		<Panel label="Roster">
			<dl class="kv">
				<dt class="label">Seed</dt>
				<dd class="mono">DAGGER_ENGINE = dagger-engine.platform.svc</dd>
				<dt class="label">Port</dt>
				<dd class="mono">1234</dd>
				<dt class="label">Resolves to</dt>
				<dd class="mono">tcp://10.2.1.14:1234<br />tcp://10.2.3.87:1234</dd>
			</dl>
			<p class="muted gap">
				Each build dials one endpoint chosen at random. The roster is DNS, read per
				dial — pods joining or leaving show up as soon as records do.
			</p>
		</Panel>

		<Panel label="Empty-roster variant">
			<dl class="kv">
				<dt class="label">Seed</dt>
				<dd class="mono muted">(unset)</dd>
				<dt class="label">Resolves to</dt>
				<dd class="mono warn">nothing — builds will auto-provision a local engine inside the srv pod</dd>
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

	.gap {
		margin-top: var(--lead-half);
	}

	.warn {
		color: var(--accent-signal);
	}
</style>
