<script>
	import "../p9.css";

	let { children } = $props();
</script>

<div class="p9-shell">
	<header class="p9-rail">
		<span class="p9-wordmark">PRODIGY9</span>
		<span class="p9-tag">platform</span>
	</header>

	<main>
		{@render children()}
	</main>
</div>
