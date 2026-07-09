# Vendor reference

**Third-party lookup material** — facts about tools and services this repo *uses* but does
not own: framework commands, an external API's signatures, another product's CLI flags,
config keys, error codes. Answers "what exactly does *their* thing do?" for surfaces you
keep reaching for.

Our own surface (our CLI, our config, our API) is not vendor — that is `../spec/`. A task
walkthrough is `../guides/`.

**Link-first, crib not mirror.** Point at the upstream source and keep only the slice you
actually reuse plus your own gotchas. Never dump a whole external API — upstream owns it,
and a full copy rots the moment they ship.

**Mark provenance.** Head each file with where it came from and when it was read:

```
<!-- derived from: <source-or-url> @ <version-or-date> -->
```

Upstream is the source of truth; the marker makes staleness legible. When the crib is
wrong, re-read upstream — you cannot fix the rot by editing here.

## Format

One file per subject: `<slug>.md` (no date prefix — describes a thing, not a moment). Favor
tables and lists; keep entries skimmable.

## Index

- [`dagger-engine.md`](dagger-engine.md) — Dagger engine capabilities & deployment: SDK pin,
  the connect call, the single-engine/many-sessions model, runtime requirements, deployment
  topologies, and the load-balancer pitfall.
- [`nginx-gateway-fabric-install.md`](nginx-gateway-fabric-install.md) — the NGF / Gateway API
  install recipe: upstream URLs, the firewall-annotation patch, the serverTokens workaround,
  and the string-forcing constraint.
