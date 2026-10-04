<script>
	import { page } from "$app/state";
	import RepositoryBuilds from "$lib/components/RepositoryBuilds.svelte";
</script>

{#key `${page.params.owner}/${page.params.repo}`}
	<RepositoryBuilds owner={page.params.owner} repo={page.params.repo} />
{/key}
