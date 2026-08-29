// Repo display logic: what the landing page and the onboarding wizard derive from
// the registration reads. Pure — no transport, no state.

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
				policy: "Always publish",
				cadence: "Every successful build",
				imageTag: "latest",
			};
		case "tags":
			return {
				policy: "Publish tags",
				cadence: "Successful tag builds",
				imageTag: "The exact Git tag",
			};
		case "never":
			return {
				policy: "Never publish",
				cadence: "Builds validate without publishing",
				imageTag: "None",
			};
		default:
			throw new Error(`unknown server publish policy: ${policy}`);
	}
}
