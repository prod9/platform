// A build's display shape: what the list and detail views derive from a record and its
// fold. Pure — no transport, no state.

// A Go zero time marshals as year 1, so anything before 2000 is "not yet" rather than a
// date the server means.
const earliestReal = 2000;

function stamped(value) {
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
		{ label: "finished", at: stamped(build.finished_at) },
		{ label: "started", at: stamped(build.started_at) },
		{ label: "created", at: stamped(build.created_at) },
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
	const from = stamped(build.started_at);
	const to = stamped(build.finished_at);
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
