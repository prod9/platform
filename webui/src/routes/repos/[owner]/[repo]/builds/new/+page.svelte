<script>
	import { page } from "$app/state";
	import ManualBuild from "$lib/components/ManualBuild.svelte";
</script>

{#key `${page.params.owner}/${page.params.repo}/${page.url.search}`}
	<ManualBuild owner={page.params.owner} repo={page.params.repo} search={page.url.search} />
{/key}
