<script>
	// The living specimen: every token the UI is built from, rendered. It is the surface
	// design changes are reviewed on, so it shows the system rather than describing it.
	let warm = $state(false);

	function toggle() {
		warm = !warm;
		document.documentElement.dataset.theme = warm ? "warm" : "";
	}

	const roles = [
		"surface",
		"surface-raised",
		"panel",
		"text",
		"text-muted",
		"border",
		"accent",
		"accent-signal",
	];

	const pigments = [
		"p9-logo-indigo",
		"p9-indigo",
		"p9-indigo-2",
		"p9-red",
		"p9-red-deep",
		"p9-ink",
		"p9-muted",
		"p9-line",
		"p9-paper",
		"p9-soft",
	];
</script>

<header class="rail rule-bottom">
	<span class="wordmark">PRODIGY9</span>
	<span class="label">platform · design</span>
	<button class="toggle label" onclick={toggle}>
		{warm ? "Too glum?" : "Too bright?"}
	</button>
</header>

<main>
	<section class="galley rule-top">
		<p class="label spine">Voices</p>
		<div class="measure">
			<h1>Iosevka Curly speaks</h1>
			<p>
				Spectral reads. It carries running prose at seventeen pixels on a
				twenty-eight pixel lead, never set in caps and never below fourteen. Its
				<em>italic</em> is the star for emphasis and quotes.
			</p>
			<p class="mono">Iosevka computes — sha 4f2a91c · 2026-08-01T09:14:02Z · 38.4s</p>
		</div>
	</section>

	<section class="galley rule-top">
		<p class="label spine">Scale</p>
		<div class="measure">
			<div class="specimen" style="font-size: var(--size-display-lg);">84</div>
			<div class="specimen" style="font-size: var(--size-display);">56</div>
			<div class="specimen" style="font-size: var(--size-heading);">28</div>
			<p class="label">Display snaps its font-size to the lattice and leads at 1</p>
		</div>
	</section>

	<section class="galley rule-top">
		<p class="label spine">Rhythm</p>
		<div class="measure">
			<div class="lattice">
				<p>
					Every vertical dimension is a whole multiple of the twenty-eight pixel
					lead. The banding behind this paragraph is the lattice itself: prose and
					mono lead identically, so two columns of different sizes land on the same
					baselines.
				</p>
				<p class="mono">two columns · one set of baselines · no drift</p>
			</div>
		</div>
	</section>

	<section class="galley rule-top">
		<p class="label spine">Roles</p>
		<div class="measure">
			<p class="muted">What components consume. The only layer warm rebinds.</p>
			<ul class="swatches">
				{#each roles as role (role)}
					<li>
						<span class="chip" style="background: var(--{role});"></span>
						<span class="mono">{role}</span>
					</li>
				{/each}
			</ul>
		</div>
	</section>

	<section class="galley rule-top">
		<p class="label spine">Pigments</p>
		<div class="measure">
			<p class="muted">The fixed palette. Immutable — warm never moves these.</p>
			<ul class="swatches">
				{#each pigments as pigment (pigment)}
					<li>
						<span class="chip" style="background: var(--{pigment});"></span>
						<span class="mono">{pigment}</span>
					</li>
				{/each}
			</ul>
		</div>
	</section>

	<section class="galley rule-top">
		<p class="label spine">Signal</p>
		<div class="measure">
			<p>
				Indigo owns the system; red creates a single moment of pressure and never
				carries equal weight.
			</p>
			<p class="pressure">One red note per view, at most.</p>
		</div>
	</section>
</main>

<style>
	.rail {
		display: flex;
		align-items: baseline;
		gap: var(--lead);
		padding: var(--lead-half) var(--lead-2);
		background: var(--surface-raised);
		box-shadow: var(--raised-shadow), 0 -1px 0 var(--border) inset;
	}

	.wordmark {
		font-family: var(--p9-display);
		font-size: var(--size-prose);
		font-weight: 700;
		letter-spacing: 0.06em;
		color: var(--accent);
	}

	.toggle {
		margin-left: auto;
		padding: 0;
		border: 0;
		background: none;
		color: var(--text-muted);
		cursor: pointer;
	}

	.toggle:hover {
		color: var(--accent-signal);
	}

	main {
		padding-bottom: var(--lead-4);
	}

	/* Every row hangs off one shared spine, so a single vertical line runs the length of
	   the page across otherwise unrelated sections. */
	.galley {
		display: grid;
		grid-template-columns: var(--spine) minmax(0, 1fr);
		gap: var(--lead);
		padding: var(--lead-2) var(--lead-2);
	}

	.spine {
		margin: 0;
		color: var(--accent);
	}

	.measure > :global(*:last-child) {
		margin-bottom: 0;
	}

	.specimen {
		font-family: var(--p9-display);
		font-weight: 700;
		line-height: 1;
		letter-spacing: -0.05em;
		margin-bottom: var(--lead);
		color: var(--accent);
	}

	/* The lead made visible: one band per unit, so drift is seen rather than measured. */
	.lattice {
		background-image: repeating-linear-gradient(
			to bottom,
			color-mix(in srgb, var(--accent) 7%, transparent) 0 1px,
			transparent 1px var(--lead)
		);
	}

	.swatches {
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
		gap: var(--lead-half) var(--lead);
		margin: 0;
		padding: 0;
		list-style: none;
	}

	.swatches li {
		display: flex;
		align-items: center;
		gap: var(--lead-half);
		line-height: var(--lead);
	}

	.chip {
		width: var(--lead);
		height: var(--lead);
		border: 1px solid var(--border);
		border-radius: var(--radius-sm);
	}

	.pressure {
		font-family: var(--p9-support);
		font-weight: 600;
		color: var(--accent-signal);
	}
</style>
