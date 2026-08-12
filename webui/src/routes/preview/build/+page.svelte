<script>
	// The build detail: modules and their steps in a navigator on the left, the selected
	// step's captured output on the right. Run-level events are navigator entries too.
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
		<h2><a href="/preview/builds/">platform</a> / #127</h2>
		<StatusChip status="failed" />
		<p class="label mono">v0.9.35 · e996f69 · webui · chakrit · 1d ago</p>
		<span class="spacer"></span>
		<Button href="/preview/new-build/">Retry</Button>
	</div>

	<div class="detail">
		<ul class="navigator">
			{#each units as unit (unit.name)}
				<li class="unit">
					<span class="name label">
						<span class="mark mark--{unit.status}">{marks[unit.status]} {unit.name}</span>
						<span>{unit.took}</span>
					</span>
					<ul>
						{#each unit.steps as step (step.name)}
							<li>
								<button
									class="step mono"
									class:sel={selected.unit === unit.name && selected.step === step.name}
									onclick={() => pick(unit, step)}
								>
									<span class="mark mark--{step.status}">{marks[step.status]} {step.name}</span>
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
	.navigator ul {
		list-style: none;
		margin: 0;
		padding: 0;
	}

	.unit {
		margin-bottom: var(--lead-half);
	}

	.unit > .name {
		display: flex;
		justify-content: space-between;
	}

	.step {
		display: flex;
		justify-content: space-between;
		gap: var(--lead-half);
		width: 100%;
		padding: 0 0 0 var(--lead-half);
		border: 0;
		background: none;
		text-align: left;
		line-height: var(--lead);
		color: var(--text-muted);
		cursor: pointer;
	}

	.step:hover {
		color: var(--accent);
	}

	.step.sel {
		color: var(--text);
		background: var(--surface-raised);
		box-shadow: -3px 0 0 var(--accent);
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
	}

	.loghead {
		display: flex;
		align-items: baseline;
		gap: var(--lead);
		padding: var(--lead-half) var(--lead) 0;
	}

	.logpane pre {
		margin: 0;
		padding: var(--lead-half) var(--lead) var(--lead);
		overflow-x: auto;
		line-height: var(--lead);
	}

	.pad {
		padding: var(--lead-half) var(--lead) var(--lead);
	}
</style>
