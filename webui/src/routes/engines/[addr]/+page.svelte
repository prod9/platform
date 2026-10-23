<script>
	import { page } from "$app/state";
	import EngineView from "$lib/components/EngineView.svelte";
	import { getEngine } from "$lib/server.js";
</script>

{#key page.params.addr}
	<EngineView read={() => getEngine(page.params.addr)} title={page.params.addr} scope="instance" />
{/key}
