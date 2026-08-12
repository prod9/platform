<script>
	// The landing page: each onboarded repo is a block — its header opens the repo's build
	// list, and its latest builds sit under it as sub-rows of the feed itself.
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

<section>
	<div class="head">
		<h2>Repositories</h2>
		<p class="label">{repos.length} onboarded</p>
		<span class="spacer"></span>
		<Button variant="primary" href="/preview/add-repo/">Add repository</Button>
	</div>

	<ul class="repos">
		{#each repos as repo (repo.full)}
			<li class="repo">
				<a class="repo-head" href="/preview/builds/">
					<span class="mono state state--{latest(repo)}">{marks[latest(repo)] ?? "·"}</span>
					<span class="mono name">{repo.full}</span>
					<span class="label">{repo.modules.join(" · ")}</span>
					<span class="mono chev">›</span>
				</a>

				<span class="subs">
					{#each repo.recent as build (build.id)}
						<a class="build" href="/preview/build/">
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

<style>
	section {
		max-width: 100ch;
	}

	.head {
		display: flex;
		align-items: baseline;
		gap: var(--lead);
		margin-bottom: var(--lead);
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

	.spacer {
		margin-left: auto;
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
