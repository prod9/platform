<script>
	import { page } from "$app/state";
	import BuildDetail from "$lib/components/BuildDetail.svelte";
</script>

{#key page.params.id}
	<BuildDetail id={page.params.id} />
{/key}
