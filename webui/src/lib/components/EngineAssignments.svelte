<script>
	import { recordedTime } from "$lib/build.js";
	import FeedRow from "./FeedRow.svelte";
	import OutcomeMark from "./OutcomeMark.svelte";
	import ObservationFreshness from "./ObservationFreshness.svelte";

	let { assignments } = $props();
</script>

<ul>
	{#each assignments as assignment (assignment.build_module_id)}
		<li>
			<FeedRow href={`/builds/${assignment.build_id}/`}>
				{#snippet mark()}<OutcomeMark status={assignment.status} />{/snippet}
				{#snippet subject()}
					{assignment.owner}/{assignment.repo} #{assignment.build_id} · {assignment.name}
					<span class="status">{assignment.status}</span>
				{/snippet}
				{#snippet facts()}
					<span>{assignment.ref}</span>
					<span>{assignment.sha}</span>
					<ObservationFreshness observation={assignment.observation} />
				{/snippet}
				{#snippet timing()}
					<span>Assigned {assignment.engine_assigned_at}</span>
					{#if recordedTime(assignment.claimed_at) !== null}
						<span>Claimed {assignment.claimed_at}</span>
					{/if}
					{#if recordedTime(assignment.last_event_at) !== null}
						<span>{assignment.last_event_kind} {assignment.last_event_at}</span>
					{/if}
				{/snippet}
			</FeedRow>
		</li>
	{:else}
		<li class="mono muted empty">No readable assignments in this view.</li>
	{/each}
</ul>

<style>
	ul { list-style: none; padding: 0; margin: 0; }
	li { box-shadow: inset 0 -1px 0 var(--hairline); }
	span { display: block; }
	.status { color: var(--muted); }
	.empty { padding-block: var(--space-3); }
</style>
