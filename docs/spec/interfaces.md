# Interfaces

Status: **draft.** This is the presentation contract for platform's CLI. It governs the
meaning the CLI carries to an operator; terminal formatting and progress narration are
specified with the commands and the engine that produce them.

Platform is a technical product for operators working directly with its configuration,
builds, and delivery machinery. Its interface exposes that system rather than translate
it into a second, friendlier vocabulary.

## Guiding principle

Minimize translation layers between values held or used by the system and values shown to
an operator.

## Concrete rules

- The CLI names its labels, actions, and values as close to the system terms they govern
  as possible.
- The CLI presents derived values as explanations or hints on their source value, never
  as independent values.
- Keep terminology consistent throughout the application and its documentation; consult
  [terminology.md](terminology.md) before introducing or presenting a term. Avoid aliases
  for the same concept, one term carrying multiple meanings, and command-specific
  renaming.
- Errors must include hints toward a corrective action when available.
- Terminal formatting and progress narration are presentation; the domain vocabulary and
  values they carry are not.
