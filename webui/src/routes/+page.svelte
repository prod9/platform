<script>
	// session gate: /api/users/me decides between the login panel and the builds list.
	let user = $state(null);
	let builds = $state([]);
	let loading = $state(true);

	async function load() {
		loading = true;

		const me = await fetch("/api/users/me");
		user = me.ok ? await me.json() : null;
		if (user) {
			const resp = await fetch("/api/builds");
			builds = resp.ok ? await resp.json() : [];
		}

		loading = false;
	}

	async function logout() {
		await fetch("/api/session", { method: "DELETE" });
		user = null;
		builds = [];
	}

	function when(iso) {
		return new Date(iso).toLocaleString();
	}

	// newline-joined images: one line per published module image.
	function imageLines(build) {
		return build.image === "" ? [] : build.image.split("\n");
	}

	load();
</script>

{#if loading}
	<section class="p9-section">
		<p class="p9-section__note">Loading…</p>
	</section>
{:else if user === null}
	<section class="p9-hero">
		<div>
			<p class="p9-eyebrow">CI/CD server</p>
			<h1 class="p9-title">platform</h1>
			<p class="p9-lede">
				Tag-triggered builds for repos on this cluster. Sign in with the GitHub
				account that pushes them.
			</p>
			<div class="p9-actions">
				<a class="p9-btn p9-btn--primary" href="/auth/github">Sign in with GitHub</a>
			</div>
		</div>
	</section>
{:else}
	<section class="p9-section" style="border-top: none;">
		<div class="page-head">
			<div>
				<h1 class="p9-section__title">Builds</h1>
				<p class="p9-section__note">Latest 50, newest first.</p>
			</div>
			<div class="whoami">
				<span class="p9-tag">{user.name}</span>
				<button class="p9-btn p9-btn--ghost" onclick={logout}>Log out</button>
			</div>
		</div>

		{#if builds.length === 0}
			<div class="p9-panel">
				<p class="p9-panel__label">No builds yet</p>
				<p class="p9-section__note" style="margin: 0;">
					Push a version tag (refs/tags/v*) on an installed repo to queue one.
				</p>
			</div>
		{:else}
			<table>
				<thead>
					<tr>
						<th>#</th>
						<th>repo</th>
						<th>tag</th>
						<th>status</th>
						<th>published</th>
						<th>updated</th>
					</tr>
				</thead>
				<tbody>
					{#each builds as build (build.id)}
						<tr>
							<td>{build.id}</td>
							<td>{build.owner}/{build.repo}</td>
							<td>{build.tag}</td>
							<td>
								<span class="status status--{build.status}">{build.status}</span>
								{#if build.status === "failed"}
									<span class="error" title={build.error}>{build.error}</span>
								{/if}
							</td>
							<td>
								{#each imageLines(build) as line (line)}
									<div>{line}</div>
								{/each}
							</td>
							<td>{when(build.updated_at)}</td>
						</tr>
					{/each}
				</tbody>
			</table>
		{/if}
	</section>
{/if}

<style>
	.page-head {
		display: flex;
		justify-content: space-between;
		align-items: start;
		gap: 24px;
	}

	.whoami {
		display: flex;
		align-items: center;
		gap: 16px;
	}

	.p9-hero {
		padding: 64px 0;
	}

	table {
		width: 100%;
		border-collapse: collapse;
		font-family: var(--p9-mono);
		font-size: 13px;
	}

	th {
		font-family: var(--p9-support);
		font-size: 12px;
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.1em;
		color: var(--p9-muted);
		text-align: left;
		padding: 10px 12px 10px 0;
		border-bottom: 1px solid var(--p9-line);
	}

	td {
		padding: 10px 12px 10px 0;
		border-bottom: 1px solid var(--p9-line);
		vertical-align: top;
	}

	.status {
		font-family: var(--p9-support);
		font-size: 12px;
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.08em;
	}

	.status--queued {
		color: var(--p9-muted);
	}

	.status--running {
		color: var(--p9-indigo);
	}

	.status--succeeded {
		color: var(--p9-indigo-2);
	}

	.status--failed {
		color: var(--p9-red-deep);
	}

	.error {
		display: block;
		color: var(--p9-muted);
		max-width: 36ch;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
</style>
