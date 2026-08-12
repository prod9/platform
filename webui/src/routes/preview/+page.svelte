<script>
	// The walkthrough has no page of its own — Repositories is the proposed landing page,
	// so /preview lands there the way / would.
	import { onMount } from "svelte";
	import { goto } from "$app/navigation";

	onMount(() => goto("/preview/repos/", { replaceState: true }));
</script>

<p class="muted">Heading to Repositories…</p>
