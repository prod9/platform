<script>
	// ⚠ MOCK — the builder-UI target design over canned data, promoted from the /preview
	// walkthrough so the tree carries exactly one picture of what to build. Before the
	// real implementation locks in: graduate the design into docs/spec, wire this to
	// GET /api/repos + the per-repo builds read, extract the shared pieces into
	// components (outcome mark, feed row, nested sub-row list), and delete the canned
	// data. The sign-in door below is real.
	import { session } from "$lib/session.svelte.js";
	import Button from "$lib/components/Button.svelte";

	const marks = { succeeded: "✓", failed: "✗", running: "◌", queued: "·" };

	const repos = [
		{
			full: "prod9/platform",
			modules: ["platform"],
			recent: [
				{ id: 128, tag: "v0.9.36", status: "succeeded", trigger: "github-push", took: "4m 12s", when: "2h ago" },
				{ id: 127, tag: "v0.9.35", status: "failed", trigger: "webui · chakrit", took: "2m 40s", when: "1d ago" },
				{ id: 126, tag: "2f4c1d9", status: "succeeded", trigger: "webui · chakrit · refs/heads/main", took: "3m 45s", when: "6h ago" },
			],
		},
		{
			full: "prod9/infra",
			modules: ["infra"],
			recent: [
				{ id: 125, tag: "v0.3.12", status: "running", trigger: "github-push", took: "1m 03s", when: "4m ago" },
				{ id: 121, tag: "v0.3.11", status: "succeeded", trigger: "github-push", took: "2m 21s", when: "3d ago" },
			],
		},
		{ full: "prod9/fx", modules: ["docs", "site"], recent: [] },
	];

	function latest(repo) {
		return repo.recent.length === 0 ? "none" : repo.recent[0].status;
	}
</script>

{#if session.user === null}
	<section class="door">
		<h1>platform</h1>
		<p>
			Tag-triggered builds for the repos on this cluster. Sign in with the GitHub
			account that pushes them.
		</p>
		<Button variant="primary" href="/auth/github">Sign in with GitHub</Button>
	</section>
{:else}
	<section>
		<div class="head">
			<h2>Repositories</h2>
			<p class="label">{repos.length} onboarded</p>
			<span class="spacer"></span>
			<Button variant="primary" href="/repos/add/">Add repository</Button>
		</div>

		<ul class="repos">
			{#each repos as repo (repo.full)}
				<li class="repo">
					<a class="repo-head" href="/builds/">
						<span class="mono state state--{latest(repo)}">{marks[latest(repo)] ?? "·"}</span>
						<span class="mono name">{repo.full}</span>
						<span class="label">{repo.modules.join(" · ")}</span>
						<span class="mono chev">›</span>
					</a>

					<span class="subs">
						{#each repo.recent as build (build.id)}
							<a class="build" href="/builds/127/">
								<span class="mono state state--{build.status}">{marks[build.status]}</span>
								<span class="mono tag">{build.tag}</span>
								<span class="mono muted">#{build.id} · {build.trigger}</span>
								<span class="mono muted timing">{build.took} · {build.when}</span>
							</a>
						{:else}
							<span class="build none">
								<span class="mono state state--none">·</span>
								<span class="mono muted">no builds yet</span>
							</span>
						{/each}
					</span>
				</li>
			{/each}
		</ul>
	</section>
{/if}

<style>
	.door {
		max-width: 62ch;
		padding: var(--lead-3) 0;
	}

	.door h1 {
		margin-bottom: var(--lead);
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

	.repos {
		list-style: none;
		margin: 0;
		padding: 0;
	}

	.repo {
		padding: var(--lead-half) 0;
		box-shadow: 0 -1px 0 var(--border) inset;
	}

	.repo-head {
		display: grid;
		grid-template-columns: var(--lead) auto 1fr var(--lead);
		align-items: baseline;
		gap: var(--lead-half);
		text-decoration: none;
		color: var(--text);
		line-height: var(--lead);
	}

	.repo-head:hover .name,
	.repo-head:hover .chev {
		color: var(--accent-signal);
	}

	/* The repo's name is the block's headline: prose-size, indigo, unmistakably not one
	   of the builds beneath it. */
	.name {
		font-size: var(--size-prose);
		font-weight: 600;
		color: var(--accent);
	}

	.chev {
		color: var(--text-muted);
		text-align: center;
	}

	/* Sub-rows hang off a vertical hairline, so the nesting reads before any row does. */
	.subs {
		display: block;
		margin-left: var(--lead-half);
		padding-left: var(--lead-half);
		box-shadow: 1px 0 0 var(--border) inset;
	}

	.build {
		display: grid;
		grid-template-columns: var(--lead) 10ch 1fr auto;
		align-items: baseline;
		gap: var(--lead-half);
		text-decoration: none;
		color: var(--text);
		line-height: var(--lead);
	}

	a.build:hover {
		background: var(--surface-quiet);
	}

	.timing {
		text-align: right;
	}

	.state {
		text-align: center;
	}

	.state--succeeded {
		color: var(--accent-ok);
	}

	.state--failed {
		color: var(--accent-signal);
	}

	.state--running {
		color: var(--accent);
	}

	.state--none,
	.state--queued {
		color: var(--text-muted);
	}
</style>
