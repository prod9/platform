<script>
	import EngineView from "$lib/components/EngineView.svelte";
	import { listEngines } from "$lib/server.js";
</script>

<EngineView read={listEngines} title="Engines" scope="fleet" />
