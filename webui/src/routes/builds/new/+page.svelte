<script>
	// ⚠ MOCK — canned data promoted from /preview; before the real implementation
	// locks in: graduate the design into docs/spec, wire the real reads, extract shared
	// components (outcome mark, feed row, kv list, terminal pane), delete canned data.
	// The manual-trigger wizard: a ref in, its sha resolved server-side, the modules read
	// from platform.toml at that commit and selectable — all on by default. Webhook builds
	// stay whole-repo; selection is the manual trigger's alone.
	import Button from "$lib/components/Button.svelte";
	import Panel from "$lib/components/Panel.svelte";
</script>

<section>
	<div class="head">
		<h2><a href="/builds/">platform</a> / new build</h2>
		<span class="spacer"></span>
		<Button href="/builds/">Cancel</Button>
	</div>

	<div class="stack">
		<Panel label="Ref">
			<input class="mono" type="text" value="refs/tags/v0.9.36" />
			<p class="muted gap">
				Resolved: <span class="mono">8c0db6e</span>
				<span class="mono muted">tests: Re-record the golden for the v0.9.36 launcher pin</span>
			</p>
		</Panel>

		<Panel label="Modules · from platform.toml at 8c0db6e">
			<ul class="modpick">
				<li class="mono">
					<label><input type="checkbox" checked /> platform <span class="muted">— go/basic</span></label>
				</li>
				<li class="mono">
					<label><input type="checkbox" checked /> srv <span class="muted">— go/basic</span></label>
				</li>
			</ul>
		</Panel>

		<div class="confirm">
			<span class="mono">
				prod9/platform @ 8c0db6e <span class="muted">→</span> v0.9.36
			</span>
			<Button variant="primary" href="/builds/127/">Queue build</Button>
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

	.stack {
		display: grid;
		gap: var(--lead);
	}

	input[type="text"] {
		width: 32ch;
		padding: 0 var(--lead-half);
		border: 1px solid var(--border);
		border-radius: var(--radius-sm);
		background: var(--surface-raised);
		line-height: var(--lead);
		color: var(--text);
	}

	.gap {
		margin-top: var(--lead-half);
	}

	.confirm {
		display: flex;
		align-items: baseline;
		justify-content: space-between;
		gap: var(--lead);
		padding: var(--lead-half) 0;
		box-shadow: 0 1px 0 var(--border) inset;
	}

	.modpick {
		list-style: none;
		margin: 0;
		padding: 0;
	}

	.modpick li {
		line-height: var(--lead);
	}

	.modpick input {
		vertical-align: baseline;
		margin-right: var(--lead-half);
		accent-color: var(--accent);
	}
</style>
