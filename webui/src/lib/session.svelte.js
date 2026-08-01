import { currentUser, endSession, Answered } from "./server.js";

// The signed-in user, shared by the shell and the pages under it: the shell needs it to
// decide what the account area shows, and a page needs it to choose between its content
// and the sign-in door. One fetch answers both.
//
// This is a *login* session. engine.Session is a different thing entirely — the span a
// built container stays usable for — and the two never meet in this file.
export const session = $state({ user: null });

export async function loadSession() {
	const result = await currentUser();
	session.user = result.outcome === Answered ? result.body : null;
}

// A session the server did not agree to end is still live, so the UI keeps showing the
// user rather than pretending they are signed out.
export async function signOut() {
	const result = await endSession();
	if (result.outcome !== Answered) {
		throw new Error(`sign out failed: ${result.outcome}`);
	}

	session.user = null;
}
