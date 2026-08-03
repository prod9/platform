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

// tagOf is the ref's last segment: a build's ref is refs/tags/vX.Y.Z and the image is
// published under vX.Y.Z, so the tag is the part a reader recognizes.
export function tagOf(ref) {
	const lastSlash = ref.lastIndexOf("/");
	if (lastSlash === -1) {
		return ref;
	}
	return ref.slice(lastSlash + 1);
}

const shaDisplayLength = 7;

export function shortSHA(sha) {
	return sha.slice(0, shaDisplayLength);
}

// lastActivity is the most recent moment the build can speak to, falling back through its
// own clock until one of them is real.
export function lastActivity(build) {
	const finished = stamped(build.finished_at);
	const started = stamped(build.started_at);
	const created = stamped(build.created_at);

	const at = finished ?? started ?? created;
	if (at === null) {
		return "";
	}
	return at.toLocaleString();
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
