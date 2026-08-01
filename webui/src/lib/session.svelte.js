import { currentUser, logout as endSession } from "./api.svelte.js";

// The signed-in user, shared by the shell and the pages under it — the shell needs it to
// decide whether there is anything to navigate, and a page needs it to decide between its
// content and the sign-in door. One fetch answers both.
//
// This is a *login* session. engine.Session is a different thing entirely — the span a
// built container stays usable for — and the two never meet in this file.
export const session = $state({
	user: null,
	resolved: false,
});

export async function loadSession() {
	session.user = await currentUser();
	session.resolved = true;
}

export async function signOut() {
	await endSession();
	session.user = null;
}
