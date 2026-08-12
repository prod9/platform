<script>
	// ⚠ MOCK — canned data promoted from /preview; the id param is ignored. Before the
	// real implementation locks in: graduate the design into docs/spec, wire this back to
	// GET /api/builds/{id} + /steps (the previous wired page is in git history at
	// f034a5a), extract shared components (outcome mark, navigator row, terminal pane),
	// delete canned data.
	import StatusChip from "$lib/components/StatusChip.svelte";
	import Button from "$lib/components/Button.svelte";

	const marks = { succeeded: "✓", failed: "✗", running: "◌", queued: "·" };

	const units = [
		{
			name: "platform",
			status: "failed",
			took: "2m 40s",
			steps: [
				{ name: "base", status: "succeeded", took: "0:03", output: "FROM golang:1.24-alpine\nresolved in 3s" },
				{ name: "deps", status: "succeeded", took: "0:41", output: "$ go mod download\nall modules verified" },
				{
					name: "test",
					status: "failed",
					took: "1:48",
					output:
						"$ go test ./...\n" +
						"ok      platform.prodigy9.co/conf       0.312s\n" +
						"ok      platform.prodigy9.co/engine     4.108s\n" +
						"ok      platform.prodigy9.co/framework  2.914s\n" +
						"--- FAIL: TestCreatedStepSurfacesNonSecretValues (1.02s)\n" +
						"    install_test.go:88: pq: sorry, too many clients already (SQLSTATE 53300)\n" +
						"FAIL    platform.prodigy9.co/srv/install 6.881s\n" +
						"FAIL\n" +
						"exit status 1",
				},
				{ name: "build", status: "queued", took: "—", output: "" },
				{ name: "publish", status: "queued", took: "—", output: "" },
			],
		},
		{
			name: "srv",
			status: "succeeded",
			took: "2m 05s",
			steps: [
				{ name: "base", status: "succeeded", took: "0:03", output: "FROM golang:1.24-alpine\nresolved in 3s" },
				{ name: "deps", status: "succeeded", took: "0:39", output: "$ go mod download\nall modules verified" },
				{ name: "test", status: "succeeded", took: "1:12", output: "$ go test ./...\nok — all packages" },
				{ name: "build", status: "succeeded", took: "0:11", output: "$ go build -o srv .\ndone" },
				{
					name: "publish",
					status: "succeeded",
					took: "0:08",
					output: "pushed ghcr.io/prod9/platform-srv:v0.9.35\ndigest sha256:19af…c2",
				},
			],
		},
	];

	let selected = $state({ unit: "platform", step: "test" });

	let current = $derived(
		units
			.find((unit) => unit.name === selected.unit)
			.steps.find((step) => step.name === selected.step),
	);

	function pick(unit, step) {
		selected = { unit: unit.name, step: step.name };
	}
</script>

<section>
	<div class="head">
		<h2><a href="/builds/">platform</a> / #127</h2>
		<StatusChip status="failed" />
		<p class="label mono">v0.9.35 · e996f69 · webui · chakrit · 1d ago</p>
		<span class="spacer"></span>
		<Button href="/builds/new/">Retry</Button>
	</div>

	<div class="detail">
		<ul class="navigator">
			{#each units as unit (unit.name)}
				<li class="unit">
					<span class="nav-row">
						<span class="mono mark mark--{unit.status}">{marks[unit.status]}</span>
						<span class="mono unit-name">{unit.name}</span>
						<span class="label">{unit.took}</span>
					</span>
					<ul class="steps">
						{#each unit.steps as step (step.name)}
							<li>
								<button
									class="nav-row step mono"
									class:sel={selected.unit === unit.name && selected.step === step.name}
									onclick={() => pick(unit, step)}
								>
									<span class="mark mark--{step.status}">{marks[step.status]}</span>
									<span class="step-name">{step.name}</span>
									<span class="muted">{step.took}</span>
								</button>
							</li>
						{/each}
					</ul>
				</li>
			{/each}
		</ul>

		<div class="logpane">
			<div class="loghead">
				<h3>{selected.unit} · {selected.step}</h3>
				<StatusChip status={current.status} />
				<span class="label">attempt 1 · {current.took}</span>
			</div>
			{#if current.output}
				<pre class="mono">{current.output}</pre>
			{:else}
				<p class="muted pad">No output yet.</p>
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

	.detail {
		display: grid;
		grid-template-columns: 34ch minmax(0, 1fr);
		gap: var(--lead-2);
		align-items: start;
	}

	.navigator,
	.steps {
		list-style: none;
		margin: 0;
		padding: 0;
	}

	.unit {
		padding: var(--lead-half) 0;
		box-shadow: 0 -1px 0 var(--border) inset;
	}

	/* One grid for every navigator line — unit heads and steps alike — so all the marks
	   sit in a single gutter column and all the times land on the right edge. Hierarchy
	   comes from type, not indentation: the module speaks prose-size indigo, its steps
	   stay in the machine's muted voice. */
	.nav-row {
		display: grid;
		grid-template-columns: var(--lead) 1fr auto;
		align-items: baseline;
		gap: var(--lead-half);
		width: 100%;
		line-height: var(--lead);
	}

	.nav-row .mark {
		text-align: center;
	}

	.unit-name {
		font-size: var(--size-prose);
		font-weight: 600;
		color: var(--accent);
	}

	.step {
		padding: 0;
		border: 0;
		background: none;
		text-align: left;
		color: var(--text-muted);
		cursor: pointer;
	}

	.step .step-name {
		color: var(--text);
	}

	.step:hover .step-name {
		color: var(--accent);
	}

	.step.sel {
		background: var(--surface-quiet);
		box-shadow: 2px 0 0 var(--accent) inset;
	}

	.mark--succeeded {
		color: var(--accent-ok);
	}

	.mark--failed {
		color: var(--accent-signal);
	}

	.mark--running {
		color: var(--accent);
	}

	.mark--queued {
		color: var(--text-muted);
	}

	.logpane {
		border: 1px solid var(--border);
		border-radius: var(--radius-md);
		background: var(--surface-raised);
		overflow: hidden;
	}

	.loghead {
		display: flex;
		align-items: baseline;
		gap: var(--lead);
		padding: var(--lead-half) var(--lead);
	}

	/* The log is a terminal, and a terminal is dark — the page's one night band. It holds
	   these pigments across modes, so it never rebinds with the theme. */
	.logpane pre {
		margin: 0;
		padding: var(--lead-half) var(--lead) var(--lead);
		overflow-x: auto;
		line-height: var(--lead);
		background: var(--p9-night);
		color: var(--p9-line);
	}

	.pad {
		padding: var(--lead-half) var(--lead) var(--lead);
	}
</style>
