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
- [`wolfi.md`](wolfi.md) — our base image's distro: why an Alpine package lookup is not
  evidence here, how to resolve a package against the image, and the packages we've verified.
- [`caddy.md`](caddy.md) — the static family's webserver: where `Content-Type` comes from,
  the `encode` defaults, and what `handle_errors` does to the response status.
- [`fx-worker.md`](fx-worker.md) — the job machinery our jobs run on: the struct-is-the-payload
  contract, what `Name()` keys, which schedule call deduplicates, and why a process runs one
  job at a time.
- [`github-app-api.md`](github-app-api.md) — the GitHub endpoints `srv/github` drives: App
  JWT claims and header form, token minting, installation/org/repo/ref lookups, and what
  "org owner" means on the wire.
- [`nginx-gateway-fabric-install.md`](nginx-gateway-fabric-install.md) — the NGF / Gateway API
  install recipe: upstream URLs, the firewall-annotation patch, the serverTokens workaround,
  and the string-forcing constraint.
