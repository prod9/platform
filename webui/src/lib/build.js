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

// lastActivity selects the most recent source timestamp without translating it. The field
// name travels with the exact value so presentation cannot make the sources look equivalent.
export function lastActivity(build) {
	if (stamped(build.finished_at) !== null) {
		return { field: "finished_at", value: build.finished_at };
	}
	if (stamped(build.started_at) !== null) {
		return { field: "started_at", value: build.started_at };
	}
	if (stamped(build.created_at) !== null) {
		return { field: "created_at", value: build.created_at };
	}
	return null;
}

// byAttempt groups the flat steps read under their attempt ordinal — the wire keeps
// steps flat with an ordinal each (spec §Operations), and the detail view reads them as
// one list per attempt. An attempt nothing reported for stays an empty group so ordinals
// keep indexing the detail view's attempts array.
export function byAttempt(steps) {
	const groups = [];
	for (const step of steps) {
		while (groups.length <= step.attempt) {
			groups.push([]);
		}
		groups[step.attempt].push(step);
	}
	return groups;
}

const secondsPerMinute = 60;

// ranFor is how long an attempt took, blank until both of its ends are real.
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
