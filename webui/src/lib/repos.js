// Repository display logic: registration choices, bounded build history, and recorded
// policy presentation. Pure — no transport, no state.

export const HistoryLimit = 50;

export function acceptBuild(history, accepted) {
	return [accepted, ...history.filter((build) => build.id !== accepted.id)]
		.sort((left, right) => right.id - left.id)
		.slice(0, HistoryLimit);
}

export function filterCandidates(candidates, filter) {
	const needle = filter.trim().toLowerCase();
	return candidates.filter((candidate) =>
		candidate.full_name.toLowerCase().includes(needle),
	);
}

// latestStatus is the nested feed's headline: builds arrive newest first, so the
// first row speaks for the repo; a repo with no builds reads "none".
export function latestStatus(builds) {
	if (builds.length === 0) {
		return "none";
	}
	return builds[0].status;
}

export function moduleLabel(module) {
	return module.framework === ""
		? module.name
		: `${module.name} (${module.framework})`;
}

export function publishPolicyDetails(policy) {
	switch (policy) {
		case "always":
			return {
				policy,
				hint: "tag: latest",
			};
		case "tags":
			return {
				policy,
				hint: "tag: exact Git tag",
			};
		case "never":
			return {
				policy,
				hint: "",
			};
		default:
			throw new Error(`unknown server publish policy: ${policy}`);
	}
}
