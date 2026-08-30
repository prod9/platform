<script>
	import { signInRetryURL } from "$lib/server.js";
	import { beginSignIn, session } from "$lib/session.svelte.js";
	import Button from "$lib/components/Button.svelte";

	const retryURL = signInRetryURL(window.location.search);
</script>

<section class="session">
	<p class="label">GitHub session</p>
	<h1>Sign-in did not complete.</h1>
	<p class="muted">
		GitHub cancelled the request or could not finish it. Nothing will retry until you
		ask it to.
	</p>
	<Button
		variant="primary"
		href={retryURL}
		onclick={beginSignIn}
		disabled={session.signingIn}
	>
		{session.signingIn ? "Signing in…" : "Try GitHub again"}
	</Button>
</section>

<style>
	.session {
		max-width: 38rem;
		padding: var(--lead-3) 0;
	}

	h1 {
		margin: var(--lead) 0;
	}

	p {
		margin: 0 0 var(--lead);
	}
</style>
