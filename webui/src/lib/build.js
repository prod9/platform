// A build's display shape: what the list and detail views derive from a record and its
// fold. Pure — no transport, no state.

// A Go zero time marshals as year 1, so anything before 2000 is "not yet" rather than a
// date the server means.
const earliestReal = 2000;

export function recordedTime(value) {
	if (!value) {
		return null;
	}

	const at = new Date(value);
	if (at.getFullYear() < earliestReal) {
		return null;
	}
	return at;
}

// lastActivity names the most recent real build moment and keeps only its clock time for
// the compact feed.
export function lastActivity(build) {
	const activities = [
		{ label: "finished", at: recordedTime(build.finished_at) },
		{ label: "started", at: recordedTime(build.started_at) },
		{ label: "created", at: recordedTime(build.created_at) },
	];
	const activity = activities.find(({ at }) => at !== null);
	if (activity === undefined) {
		return "";
	}

	const clock = activity.at.toISOString().slice(11, 19);
	return `${activity.label} ${clock}`;
}

const secondsPerMinute = 60;

// ranFor is elapsed execution time, blank until both ends are real.
export function ranFor(build) {
	const from = recordedTime(build.started_at);
	const to = recordedTime(build.finished_at);
	if (from === null || to === null) {
		return "";
	}

	const seconds = Math.round((to - from) / 1000);
	if (seconds < secondsPerMinute) {
		return `${seconds}s`;
	}

	const minutes = Math.floor(seconds / secondsPerMinute);
	return `${minutes}m ${seconds % secondsPerMinute}s`;
}

export function stepStatus(step, moduleStatus) {
	if (recordedTime(step.finished_at) !== null) {
		return step.error === "" ? "succeeded" : "failed";
	}
	if (["succeeded", "failed"].includes(moduleStatus)) {
		return "incomplete";
	}

	return "running";
}

export function groupBuildSteps(build, steps) {
	const recorded = steps.map((step, recordedOrder) => ({ ...step, recordedOrder }));
	return build.modules.map((module) => ({
		...module,
		steps: recorded
			.filter((step) => step.build_module_id === module.build_module_id)
			.map((step) => ({ ...step, status: stepStatus(step, module.status) })),
	}));
}

export function selectBuildStep(groups, selected) {
	const steps = groups.flatMap((module) => module.steps);
	if (selected !== null) {
		const retained = steps.find((step) => (
			step.build_module_id === selected.build_module_id &&
			step.step === selected.step &&
			step.started_at === selected.started_at
		));
		if (retained !== undefined) {
			return retained;
		}
	}

	return steps.reduce((first, step) => (
		first === null || step.recordedOrder < first.recordedOrder ? step : first
	), null);
}
