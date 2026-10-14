<!-- not spec/decision because: execution record; visual contract lives in spec/webui.md -->

# Platform web UI: rebuild the visual system from PRODIGY9 v2027

Status: executed locally in `942271f`, following spec-only commit `93a9c84`.
Frontend tests/build, Go tests, and smoke completed; smoke was UNCHANGED. Browser
fixture review covered every existing route and its selected states. Verification and
captures: `tmp/v2027-redesign.local/report.md`. No push, release, or publication occurred.
Audience: chakrit and engineers implementing the redesign in a fresh session.

## Goal and authority

Replace platform's existing visual system with the settled v2027 design language from
the sibling `prod9/prodigy9.co` repository. Rebuild the styling at its owning components
and layouts rather than applying an override skin. Preserve the application's existing
content, data, actions, and state distinctions.

Chakrit requested: "Plan for a long redesign (cosmetics) session please" and
"Rip-out all stylings in platform and re-build it using stuff in v2027 please."
The visual contract is in `docs/spec/webui.md`; this document records the execution
approach used for the completed cosmetic rebuild.

Redesign scope is all of `webui/`, including installation, authentication, product
screens, existing mock screens, and every loading/error/empty/disabled/stale state.
Chakrit ruled "remove the explainer site": remove `www/` and its dedicated publishing
script `scripts/docs-site-deploy.sh`, rather than redesigning them. The source design
record in `docs/` remains the maintained documentation.
The CLI/TUI and server feature roadmap are separate work.

The working tree already contains an unrelated `.gitignore` edit adding ACE `.pi`
paths. Its provenance is recorded in `.ace/save.md`; leave it outside redesign commits.
The earlier styling failure remains an unresolved report, not a diagnosed cause or
evidence that this redesign repairs the user's browser rendering.

## Design sources and precedence

Use the sibling repository's current v2027 styles, assets, and settled preview records.
Do not use pre-v2027 designs or the generic brand skill's superseded defaults.
`../prodigy9.co/AGENTS.md:35–43` explicitly states that v2027 rulings override the older
brand law and that the previous site's code was removed.

That repository names its Outline collection as the authority and requires reading
"0 · Start here" (`../prodigy9.co/AGENTS.md:6–15`). This session has no callable Outline
connector; the collection has not been read. The local settled handoffs support this
plan, but completing the authoritative source check remains the first implementation
prerequisite if access becomes available; surface any conflict before adopting it.

| Subject          | Settled source in `../prodigy9.co/`                           | Adopted direction                                               |
|------------------|--------------------------------------------------------------|-----------------------------------------------------------------|
| Palette          | `src/styles/tokens.css:1–21`                                  | Paper, soft, ink, indigo, night; one bike red `#e2282e`            |
| Typography       | `src/styles/type.css:1–44`                                    | Curly headings, Aile body, Iosevka data, Mona Sans buttons         |
| Type scale       | `src/styles/type.css:30–43`                                   | Perfect-fourth steps; line heights on a 4px unit                  |
| Geometry         | `src/styles/diagonals.css:1–19,47–75`                          | Logo-angle chamfers; leaning tags reserved for caps              |
| Depth            | `src/styles/depth.css:1–25,73–84`                             | Crisp keys, 3px edge, press/latch states, restrained sheen        |
| Controls         | `src/styles/controls.css:1–33`                                | Native inputs, chamfered soft fields, visible focus and selection |
| Panels and rules | `src/styles/components.css:1–22`                              | Raised surfaces, cap hierarchy, inset separators                 |
| Art              | `src/styles/texture.css:1–28,80–82`                           | Settled Akira masks, 6% base strength, cap gain 1.25               |
| Logo             | `src/components/preview/akira-logo/meta.js:1–10`              | Approved monocolor masters from `logo_bundle/akira/`              |
| Navigation       | `src/components/preview/nav/meta.js:1–22`                     | Responsive shared navigation and clear hierarchy                 |
| Page ground      | `src/styles/base.css:31–38`                                  | Paper page; night surfaces are contextual, not a dark-mode ruling |

Later rulings win over stale notes: the Akira palette explicitly makes logo, buttons,
and lit tags the same red. Do not revive the older separate logo-red rule.

Adapt the vocabulary to an operational application: readable feeds, forms, and captured
output take precedence over marketing-page proportions. Keep product pages full-width
as required by `docs/spec/webui.md:24–30`; do not copy the site's 64rem page cap or its
sales CTA. Reuse art on appropriate capped surfaces, keeping data and logs unobstructed.
Do not import the site's 3D scene, sales content, localization system, or consent flow.

Proposed domain treatment: indigo for active work, muted ink for pending work, a clear
positive glyph for success, and red for failure. These are application adaptations,
not additional settled site rulings; keep accessible labels and existing status meanings.

## Settled old-design removal

Chakrit ruled: "remove EVERYTHING from the old design ok? including mode toggles."
Remove the old visual system in full, including the warm-mode toggle, its labels,
theme state module, preference reads/writes, mode-specific tokens, and unused old assets
and dependencies. Use the settled v2027 palette with contextual paper/night surfaces;
do not carry an old mode forward or introduce a replacement mode toggle.

This explicit ruling authorizes the theme exception to the existing cosmetics contract
that preserves visible words and behavior (`docs/spec/webui.md:48–52`). The current
toggle and stored preference are in `webui/src/routes/+layout.svelte:103–106` and
`webui/src/lib/theme.svelte.js:1–24`.

## Execution sequence

### Establish the contract before code

Record the accepted visual contract and old-design removal in
`docs/spec/webui.md` in a dedicated spec-only commit before implementation.
Keep the shared ownership rule, page content, and existing route/API contracts explicit.
Only the approved theme exception may change behavior in this cosmetic session.

Make a route/state inventory and inspect current rendering using a server chakrit has
started. Record representative desktop and narrow layouts as visual comparison evidence.
Use existing remote configuration; do not start a server or assign a new port.
Evidence and captures belong in `tmp/v2027-redesign.local/`.

### Replace the foundation and establish one complete screen

Replace `webui/src/lib/styles/{tokens,fonts,base,app}.css` and root-shell styling with
the v2027 palette, font roles, 4px rhythm, chamfer geometry, and depth mechanics.
Use self-contained copies of the selected settled assets in platform; runtime and build
must not depend on the sibling checkout. Record source paths and the source revision.

Reuse the actual monocolor logo and supplied font assets; remove obsolete Spectral
imports/dependency when unused. Inventory font weights and script coverage before
copying: transfer only what the UI consumes while retaining suitable fallbacks.
Review the site's font declarations carefully because some font data is embedded.

Rebuild `Button`, `Panel`, `PageHeader`, `SectionHeader`, `Facts`, `SignalMark`,
`OutcomeMark`, `LoadingBlock`, and `TerminalPane` at their current ownership seams.
The existing seams are suitable for reuse; none needs a replacement framework.

Use the recorded build-detail screen as the first complete visual checkpoint: it
exercises navigation, title/actions, facts, status signals, module/step selection,
night output panes, long technical values, empty output, loading, and stale errors.
Review that screen before expanding the new treatment throughout the app.

Keep partially migrated states local to the work session; do not release a mixed design.
Replace old rules directly, not through legacy token aliases or a parallel theme skin.

### Carry the visual system through every surface

Apply the established treatment to these coherent groups:

- Repository landing, repository first-build page, registration wizard.
- Installation wizard, signed-out landing, session recovery and shell failures.
- System settings and migrations, including blocked and manual-recovery states.
- Existing build-list, build-wizard, engine-fleet, and engine-instance mock screens.

The mock screens stay at their current capability level; redesigning them is not
authorization to implement new APIs, engine data, polling, retry, or streaming.

Before creating another visual unit, inspect and adapt existing capabilities.
Form styling repeats across installer, onboarding, and build trigger; installation
and onboarding repeat checklist/action/instructions composition. Extract a named shared
form or wizard presentation only where those concrete consumers justify it, keeping
their domain state and actions in their owning screens. Feed similarity alone does not
justify merging repository and engine domains.

Remove every replaced route-local and component-local style, old typography role,
unused asset, and obsolete dependency within the approved scope. Route-local CSS
continues to own domain layout only. Verify the deletion boundary with searches and
review rather than source-text tests.

### Verify, audit, and finish

Inspect every route at desktop and narrow widths, with keyboard navigation, visible
focus, selected/pressed/disabled controls, reduced motion, cold font loading, and long
repository names, SHAs, errors, and output. Check contrast on each actual surface;
clip paths must not cut focus indicators or content.

Verify that layout changes preserve installation/session gates, inert loading,
onboarding's manifest confirmation, no overlapping build requests, selected-step
retention, stale snapshots, migration recovery, and separately labeled escaped output.
These contracts are in `docs/spec/webui.md:81–185`, `docs/spec/installation.md`, and
the existing client components; do not rewrite them to simplify styling.

Run the existing frontend tests and production UI build, then `go test ./...` and
`./test.sh`, retaining current budgets. Run frontend checks during shared-component
work, and the broad suites at each completed implementation slice required by repository
rules. Review smoke drift before any re-recording; UNCHANGED is not visual approval.
Do not add tests that police CSS classes or freeze incidental markup.

Regenerate and include `webui/build/`: Go embeds that committed tree, so a source-only
redesign would not change the shipped UI (`docs/spec/platform-server.md:361–371`).
Check generated CSS/font/logo URLs for both prerendered and dynamic-fallback pages.

Perform independent visual/behavior and code-ownership audits in parallel, then review
the full diff and fix in-scope findings. Keep coherent local commits on the current
branch with spec preceding implementation; leave unrelated edits untouched.
Push, release, publication, and deployment require separate explicit authorization.

## Long-session checkpoints and completion evidence

Keep each checkpoint resumable with the accepted scope, changed areas, verification,
remaining screens/states, and next step. Use the existing ACE state files for continuity,
not a new session journal. Do not re-open settled v2027 choices during a later checkpoint.

Completion requires one consistent v2027 visual system across the approved scope,
no surviving old styling or unused old assets there, preserved behavior except the
explicit theme ruling, a rebuilt embedded UI, passing required checks, and a clean
full-scope audit. Browser review evidence must accompany test results because the
automated suites alone cannot establish visual quality.
