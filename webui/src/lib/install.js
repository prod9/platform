// Wizard logic for the install page: which step is next, and the credentials wire
// payload (docs/spec/installation.md §The wizard UI).

// nextStep picks the entry the wizard renders a panel for — the first one not fully
// ready — or null once the whole checklist is.
export function nextStep(entries) {
	return entries.find((entry) => entry.state !== "fully_ready") ?? null;
}

// orgSettingsURL builds the org's developer-settings link for a given trailing path
// (e.g. "apps/new", "apps"), or null when no org slug has been entered — callers fall
// back to a placeholder instruction then.
export function orgSettingsURL(slug, path) {
	const org = (slug ?? "").trim();
	if (org === "") {
		return null;
	}
	return `https://github.com/organizations/${org}/settings/${path}`;
}

// appPayload shapes the create-the-App form — what GitHub's creation form yields —
// into the action's wire shape: trimmed strings and a numeric app_id. Emptiness and
// zero are left in for the server's validator to refuse — the form only decides when
// to enable save.
export function appPayload(fields) {
	return {
		app_id: Number(text(fields.app_id)) || 0,
		client_id: text(fields.client_id),
		webhook_secret: text(fields.webhook_secret),
	};
}

// credentialsPayload shapes the generated-keys form — the pair GitHub generates on
// the created App's settings page — the same way.
export function credentialsPayload(fields) {
	return {
		private_key: text(fields.private_key),
		client_secret: text(fields.client_secret),
	};
}

// registryPayload shapes the registry-token form — the ghcr PAT the operator
// creates by hand — the same way.
export function registryPayload(fields) {
	return {
		token: text(fields.token),
	};
}

// orgPayload shapes the name-the-org form the same way.
export function orgPayload(fields) {
	return { org: text(fields.org) };
}

// stepValues returns the named entry's saved non-secret values, for panel pre-fill
// (docs/spec/installation.md §The wizard UI — secret fields always render empty and
// never arrive here; the server puts only non-secret fields in values).
export function stepValues(entries, name) {
	const entry = entries.find((e) => e.name === name);
	return entry?.values ?? {};
}

// orgSlug reads the slug the org step surfaced — the server-side source every
// GitHub link is built from, so any tab or browser renders real links.
export function orgSlug(entries) {
	return stepValues(entries, "org").org ?? "";
}

// generateWebhookSecret mints the webhook secret the operator copies into GitHub's
// creation form — 32 random bytes as hex, from the platform's CSPRNG.
export function generateWebhookSecret() {
	const bytes = crypto.getRandomValues(new Uint8Array(32));
	return Array.from(bytes, (b) => b.toString(16).padStart(2, "0")).join("");
}

function text(value) {
	return (value ?? "").trim();
}
