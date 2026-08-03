// Wizard logic for the install page: which step is next, and the credentials wire
// payload (docs/spec/installation.md §The wizard UI).

// nextStep picks the entry the wizard renders a panel for — the first one not done —
// or null once the whole checklist is.
export function nextStep(entries) {
	return entries.find((entry) => entry.status !== "done") ?? null;
}

// credentialsPayload shapes the five-field form into the action's wire shape: trimmed
// strings and a numeric app_id. Emptiness and zero are left in for the server's
// validator to refuse — the form only decides when to enable save.
export function credentialsPayload(fields) {
	const text = (value) => (value ?? "").trim();

	return {
		app_id: Number(text(fields.app_id)) || 0,
		private_key: text(fields.private_key),
		webhook_secret: text(fields.webhook_secret),
		client_id: text(fields.client_id),
		client_secret: text(fields.client_secret),
	};
}
