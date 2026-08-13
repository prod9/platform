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

// moduleLine renders a manifest's modules the way the review step states them:
// "api (go/basic) · web (pnpm/static)"; a module with no framework is its name alone.
export function moduleLine(modules) {
	return modules
		.map((module) =>
			module.framework === "" ? module.name : `${module.name} (${module.framework})`,
		)
		.join(" · ");
}
