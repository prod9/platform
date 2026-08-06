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

// appSettingsURL builds the created App's direct settings link — its edit page, or
// a sub-page via the trailing path (e.g. "/installations") — from the org and App
// slugs the server surfaced, or null until both are saved.
export function appSettingsURL(entries, path = "") {
	const app = text(stepValues(entries, "app-created").app_slug);
	if (app === "") {
		return null;
	}
	return orgSettingsURL(orgSlug(entries), `apps/${app}${path}`);
}

// appSlugFromURL extracts the App's slug from whatever the operator pasted: the
// settings-page URL (…/settings/apps/<slug>[/…]), the public-page URL
// (github.com/apps/<slug>), or the bare slug itself. Creation-flow pages carry no
// slug ("apps", "apps/new") and come back empty for the server to refuse.
export function appSlugFromURL(value) {
	const pasted = text(value);
	if (!pasted.includes("/")) {
		return pasted;
	}

	const segments = pasted.split("/").filter((s) => s !== "");
	const apps = segments.lastIndexOf("apps");
	const slug = apps === -1 ? "" : (segments[apps + 1] ?? "");
	return slug === "new" ? "" : slug;
}

// appPayload shapes the create-the-App form — what GitHub's creation form yields —
// into the action's wire shape: trimmed strings and a numeric app_id. Emptiness and
// zero are left in for the server's validator to refuse — the form only decides when
// to enable save.
export function appPayload(fields) {
	return {
		app_id: Number(text(fields.app_id)) || 0,
		app_slug: appSlugFromURL(fields.app_slug),
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

// serverPayload shapes the name-the-server form the same way.
export function serverPayload(fields) {
	return { public_url: text(fields.public_url) };
}

// enginePayload shapes the bind-the-engine form the same way.
export function enginePayload(fields) {
	return { hosts: text(fields.hosts) };
}

// publicURL reads the URL the server step surfaced — the server-side value every
// instruction that says "the server's URL" renders from, never the browser origin
// (docs/spec/installation.md, the server step).
export function publicURL(entries) {
	return text(stepValues(entries, "server").public_url);
}

// originMismatch reports a saved public URL that differs from the browser's origin —
// the wizard warns then: values pasted into GitHub from a non-canonical host would
// point at the wrong place (docs/spec/installation.md, the server step).
export function originMismatch(entries, origin) {
	const saved = publicURL(entries);
	return saved !== "" && saved.replace(/\/+$/, "") !== text(origin).replace(/\/+$/, "");
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
