import { render } from "svelte/server";
import { afterEach, describe, expect, test } from "vitest";
import { session } from "$lib/session.svelte.js";
import InstallationAction from "./InstallationAction.svelte";

const baseProps = {
	entries: [],
	locked: false,
	origin: "https://platform.example.com",
	installationID: 0,
	signInURL: "/auth/github",
	appInstallURL: null,
	onconverge: () => {},
	onredo: () => {},
};

function renderAction(current, props = {}) {
	return render(InstallationAction, {
		props: { ...baseProps, ...props, current },
	}).body;
}

afterEach(() => {
	session.user = null;
	session.signingIn = false;
});

describe("InstallationAction", () => {
	test("renders a completed diagnostic step as settled", () => {
		const body = renderAction(
			{ name: "db-reachable", state: "fully_ready" },
			{ locked: true },
		);

		expect(body).toContain("Database reachable");
		expect(body).toContain("Nothing to redo");
	});

	test("prefills an editable server step from settled state", () => {
		const entries = [
			{
				name: "server",
				state: "not_started",
				values: { public_url: "https://saved.example.com" },
			},
		];

		const body = renderAction(entries[0], { entries });

		expect(body).toContain("Name the server");
		expect(body).toContain('value="https://saved.example.com"');
		expect(body).toContain("Save URL");
	});

	test("renders an intervention-required migration as blocked", () => {
		const body = renderAction({
			name: "migrations",
			state: "intervention_required",
			message: "migration history diverged",
		});

		expect(body).toContain("Migration blocked");
		expect(body).toContain("migration history diverged");
	});

	test("renders the signed-in claim action with the landed installation", () => {
		session.user = { name: "Chakrit" };

		const body = renderAction(
			{ name: "claimed", state: "not_started" },
			{ installationID: 84 },
		);

		expect(body).toContain("Bind installation");
		expect(body).toContain("#84");
		expect(body).toContain("Chakrit");
		expect(body).toContain("Claim installation");
	});
});
