# Infra manifests publish as a plain Dagger image; retire oras-go + Flux media types

- **Date:** 2026-07-05
- **PR:** manual
- **Status:** accepted
- **Recorded:** 2026-07-07 — belatedly. This ruling was made 2026-07-05 and lived only in
  rolling notes ([`../scratch/2026-07-05-resume.md`](../scratch/2026-07-05-resume.md) §Next-2,
  [builders reshape design](../scratch/prior-art.md#builders-reshape-design-pass-4-2026-06-29)),
  so it kept getting re-derived. Promoted to a decision to stop the re-litigation.

## Decision

Publish the rendered infra manifest tree as a **plain Dagger-built image** (`FROM scratch`
carrying the YAML). Flux consumes it via `OCIRepository` + a **`layerSelector`** that extracts
the `application/vnd.oci.image.layer.v1.tar+gzip` layer; kustomize-controller applies the
extracted docs. **Retire `oras-go` and the hand-built Flux-media-type path** in
`gitops.Publish` (and `gitops/registry.go`).

Consequences that follow directly:

- **Infra becomes a real builder module** (a `platform/infra` builder). Its output image
  carries the rendered manifests, so **infra publish IS the normal `publish` verb** — no
  separate publish mechanism, no `ops publish`. (Today `infra/`'s `[modules]` is empty, so
  `platform publish` there is a no-op; the builder fills that in.)
- **No `deploy` verb for infra.** Flux tracks the moving `latest` OCI tag, so *publishing is
  the deploy*. A separate env-promotion verb for manifests makes no sense.
- **`ops render` → top-level `render`.** Once the bespoke publish path is gone, `ops` is down
  to one verb and isn't worth a namespace group.

## Rationale

The obvious approach — and what the code does today — is to make the **producer** emit
exactly what Flux wants: `gitops.Publish` hand-builds an OCI artifact with Flux's media types
(`application/vnd.cncf.flux.config.v1+json`, `…flux.content.v1.tar+gzip`) using `oras-go`.

That path was rejected because **Dagger cannot emit those media types.** Dagger's only
media-type knob is `ImageMediaTypes` (`OCI` | `Docker`) — image manifests only. There is no
way to set a config media type, a Flux content-layer type, or an `artifactType`;
`WithAnnotation` adds annotations, not media types. So matching the Flux shape *requires* a
bespoke non-Dagger pusher — which is why `oras-go` exists in the tree at all.

The resolution **moves the compatibility from the producer to the consumer.** Flux's
`OCIRepository` already supports consuming a **plain image** via `layerSelector` (`mediaType:
application/vnd.oci.image.layer.v1.tar+gzip`, `operation: extract`) — the canonical, well-
supported Flux OCI path, **already in production on stage9** (the existing `OCIRepository` +
`Kustomization` pair). The new model keeps that same consumer pair and only swaps the
artifact shape: plain image + `layerSelector` instead of the flux-media-type artifact. So
Dagger's native image output is exactly enough — no bespoke pusher needed.

Why this matters beyond tidiness: the bespoke oras/flux-media-type path is the **root cause**
of three separate warts —

1. the separate `ops` command namespace (it needed its own publish that the app path couldn't
   provide),
2. the **registry-cred asymmetry** — app image publish authenticates via local docker
   credentials (osxkeychain) through Dagger, while `ops publish` needs
   `REGISTRY_USERNAME`/`REGISTRY_PASSWORD` env for oras (`187f20d` is an interim keychain
   patch on exactly this), and
3. `oras-go` sitting in `go.mod` at all.

Collapsing infra onto the Dagger builder path erases all three at once.

**Reproducibility is deliberately a non-issue.** kustomize-controller's apply is idempotent
server-side: identical content ⇒ no-op apply regardless of digest. A Dagger layer tarball may
carry timestamps and re-push a new digest on an unchanged render, but that only costs a
harmless re-pull + re-apply. Nothing here depends on artifact-level digest stability. (Files
synthesized via `WithNewFile` tend to get fixed timestamps anyway.)

## Scope note

`gitops` keeps the **render** half (CUE/`.platform` → manifest `Tree`). Only the **publish**
half (the oras packer + Flux media types + `RemoteRepository` auth) retires; the tree is
carried into the scratch image by the infra builder instead of oras-packed. The serverless
path (`render` → `kubectl apply`) is unaffected.

## Relationship to other decisions

- Refines [2026-07-05 — Delivery verbs are orthogonal](2026-07-05-delivery-verbs-are-orthogonal.md):
  that ADR framed infra config as "its own `ops render` / `ops publish` concern." This ruling
  collapses the *publish* half onto the normal `publish` verb (infra = a builder). The
  `release`-vs-`publish` orthogonality it establishes is untouched; the app-vs-infra *publish*
  split is what goes.
- Supersedes the oras-go mechanic noted in
  [2026-06-14 — Pull-based GitOps](2026-06-14-pull-based-gitops-timoni-flux.md).
