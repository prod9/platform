<section>
	<h2>Builder UI walkthrough</h2>

	<p class="muted">
		Seven views over canned data, in walk order along the strip above. The proposed
		top-level nav is <span class="mono">Repositories · Engines · Settings</span> — the
		repo list is the landing page, builds nest under a repo, and a build's detail page
		carries the module/step navigator.
	</p>

	<p class="muted">
		Repositories here are <em>onboarded</em> repos — registered rows in the repo table —
		with visibility still gated live by GitHub permission on every read. Manual builds
		may pick modules; webhook builds stay whole-repo.
	</p>
</section>

<style>
	section {
		max-width: 72ch;
	}

	h2 {
		margin-bottom: var(--lead);
	}
</style>
