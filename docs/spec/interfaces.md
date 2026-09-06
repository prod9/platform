# Interfaces

Status: **draft.** This is the shared presentation contract for platform's web UI and
CLI. It governs the meaning carried across those interfaces; their layouts and interaction
mechanics remain in their own specs.

Platform is a technical product for operators working directly with its configuration,
records, builds, and delivery machinery. Its interfaces expose that system rather than
translate it into a second, friendlier vocabulary.

## Guiding principle

Minimize translation layers between values held or used by the system and values shown to
an operator.

## Concrete rules

### Web UI and CLI

- Web UI and CLI name their labels, actions, and values as close to the system terms they
  govern as possible.
- Web UI and CLI present derived values as explanations or hints on their source value,
  never as independent values.
- Keep terminology consistent throughout the application and across every interface;
  consult [terminology.md](terminology.md) before introducing or presenting a term. Avoid
  aliases for the same concept, one term carrying multiple meanings, and
  interface-specific renaming.
- Errors must include hints toward a corrective action when available.

### Web UI

- Labels use human-readable forms kept close to canonical field names; values remain
  canonical. For example, present `publish_policy` as `Publish policy`, but keep its
  `always` value rather than translating it to `Always publish`.
- One source field does not become several translated rows that appear to be independently
  stored values.
- A mark, abbreviation, relative time, duration, or other derived presentation may clarify
  a canonical value, but never replace it. Keep the source value visible and attach the
  derivation to it as a hint.
- Vertical rhythm is structural. The web interface sets 16px prose on a 24px content
  line and derives its spacing lattice from that line. Every block-flow margin, padding,
  gap, and line height lands on a whole lattice unit. Half-units appear only as equal
  block-start and block-end padding that add up to one unit. Components own complete
  rhythmic blocks, and parent layouts compose those blocks without compensating margins.
- Control text keeps the same 24px content line as adjacent prose. Symmetric block
  padding, not a taller content line, enlarges buttons and form controls into comfortable
  action surfaces; mixed text-and-control rows center those boxes in the block direction.
  A selectable row owns its padding so its whole visible surface is interactive. Inline
  links inside prose remain inline because they are reading affordances, not standalone
  controls.
- Never present flashing content. While data loads, render the content's full structural
  chrome — page and section headers, actions, panels, columns, lists, and controls — in
  its final geometry, with static placeholders only for unknown values. Never hide a
  large component or layout behind a `Loading` label.

Repository onboarding therefore presents the resolved value that registration will
persist, with a relevant derived tag kept on the same field as a hint:

```text
Publish policy  always  (tag: latest)
```

Do not replace it with disconnected presentation aliases:

```text
publish policy   Always publish
publish cadence  Every successful build
image tag        latest
```

### CLI

- Terminal formatting and progress narration may differ from web UI layout; the domain
  vocabulary and values do not.
