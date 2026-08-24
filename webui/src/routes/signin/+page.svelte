<script>
	import { signInRetryURL } from "$lib/server.js";
	import Button from "$lib/components/Button.svelte";

	const retryURL = signInRetryURL(window.location.search);
</script>

<section class="signin">
	<p class="label">GitHub sign-in</p>
	<h1>Sign-in did not complete.</h1>
	<p class="muted">
		GitHub cancelled the request or could not finish it. Nothing will retry until you
		ask it to.
	</p>
	<Button variant="primary" href={retryURL}>Try GitHub again</Button>
</section>

<style>
	.signin {
		max-width: 38rem;
		padding: var(--lead-3) 0;
	}

	h1 {
		margin: var(--lead-half) 0 var(--lead);
	}

	p {
		margin: 0 0 var(--lead);
	}
</style>
