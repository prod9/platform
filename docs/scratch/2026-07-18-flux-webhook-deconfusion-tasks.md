<!-- not spec/decision because: an actionable TO-DO list (documentation-hygiene edits + one
ADR deletion), not a ruling or a design. Committed (not in machine-local .ace/) so a next
session on any machine can pick it up and execute. Delete this file once all tasks are done. -->

# TO-DO — de-confuse "flux webhook" (created 2026-07-18)

**STATUS: ALL TASKS DONE (2026-07-18).** T1–T4 complete; the `platform-server.md` item
resolved — the per-repo flux-webhook endpoint is **VOID** (org-wide, install-time webhook),
struck from the spec, not gated. Durable disambiguation now lives in CLAUDE.md "Build &
delivery facts". This file's purpose is served — deletable.

---

## Read this first — what "de-confuse flux webhook" actually means

The bare phrase **"flux webhook" is ambiguous**, and a prior (AFK) session — and Claude,
repeatedly across a live session — conflated **two completely different things that point in
opposite directions**:

- **GitHub → Flux** (the infra delivery *mechanism*). On a GHCR publish, GitHub fires a
  `registry_package` webhook; a Flux **`Receiver` inside the cluster** validates the HMAC and
  pokes the infra `OCIRepository` to reconcile immediately instead of waiting for the poll.
  This is the *reconcile trigger*. It is an **infra-side** concern.

- **Flux → srv** (platform srv *observing* delivery). The question "does Flux push
  notifications INTO platform srv?" chakrit's ruling: **no** — srv **reads** Flux CR state
  (pull, via the pod ServiceAccount); there is **no** Flux→srv webhook. This is the
  **srv-side** concern (srv 1-by-1 **item 8**).

**The conflation:** when chakrit said "no flux webhook" he meant **Flux→srv**. A prior session
applied it to the **GitHub→Flux** mechanism and proposed deleting the `Receiver`, superseding
the ADR, etc. — all aimed at the wrong direction. chakrit (verbatim): *"when i say no flux
webhook, this meant no webhook FROM flux TO platform SRV. This is completely not related to
anything FROM github TO flux-in-infra."*

**Canonical wording to enforce everywhere:**
- the mechanism → write **"GitHub→Flux Receiver"** (never bare "flux webhook").
- the observability direction → **"Flux→srv"**, which *does not exist* (srv reads Flux state).

**Scope guard:** de-confusing is about **making the direction explicit in the docs**. It does
**not** mean changing the GitHub→Flux mechanism itself. Whether that mechanism (push Receiver
vs poll-only) is right is a **separate infra decision**, out of scope here.

---

## Tasks

### T1 — fix the conflated srv item-8 notes  →  corrected shape everywhere
Corrected shape: *srv observes Flux by **reading** CR state (pull, pod SA); there is **no**
Flux→srv webhook; this is **unrelated** to the GitHub→Flux Receiver.*
- [x] `.ace/save.ledger.md` item 8 body — **already rewritten 2026-07-18** (machine-local).
- [x] `docs/scratch/2026-07-17-srv-1by1.md` item 8 (~line 82) — body still reads "drop the
  webhook + delete baseline Receiver/HMAC/`FLUX_HOSTNAME` + supersede the flux-webhook ADR."
  That's the conflation. Rewrite it to the corrected shape above. (scratch = editable.)
- [x] `docs/scratch/2026-07-17-trail-fix-plan.md` (~line 208) — table row "Flux webhook drop
  + observ." → relabel to "Flux→srv observability (read only)".

### T2 — DELETE the wrong ADR
chakrit: a wrong ADR misleads future runs, so delete rather than annotate.
- [x] Delete `docs/decisions/2026-07-13-flux-webhook-delivery.md` entirely.
- [x] Remove its index line in `docs/decisions/README.md` (~line 70).
- **Caveat:** this deletes the *doc*, not the GitHub→Flux *mechanism* — the baseline
  `Receiver` and the `TestEmbeddedFluxReceiver` guard still exist in the framework/baseline.
  Whether that mechanism is itself correct is a **separate infra decision**, NOT part of this
  deletion. (The ADR was also largely *spec content mis-filed as a decision* — if the
  mechanism is later kept, its design belongs in `spec/`, not a re-created ADR.)

### T3 — disambiguate the trap term in the GitHub→Flux docs
Replace bare "flux webhook" with "GitHub→Flux Receiver" where the mechanism is meant, in:
- [x] `docs/spec/scaffolding.md` (~lines 90, 100)
- [x] `docs/spec/config-allocation.md` (~lines 86, 88)
- [x] `docs/spec/platform-server.md` — broken ADR link struck; the `POST .../flux-webhook`
  endpoint row + prose struck as **VOID** (see below).
- [x] `docs/guides/cluster-bringup.md` (`FLUX_HOSTNAME` / Receiver references)
- **Resolved (2026-07-18), not gated:** the per-repo `POST /api/repos/{owner}/{repo}/flux-webhook`
  endpoint (`srv/flux/webhook.go`) is **VOID** — the GitHub→Flux `registry_package` webhook is
  **org-wide, provisioned once in the install flow**, never minted per-repo. Struck from
  `platform-server.md`; the srv rebuild drops the code.

### T4 — pin the two directions durably so this can never re-trap
- [x] Add the canonical GitHub→Flux vs Flux→srv distinction (the two bullets under "Read this
  first") to a **durable, always-loaded** home. **Home is chakrit's to pick** — recommend
  CLAUDE.md "Build & delivery facts" (loaded every session); NOT the terminology-lexicon ADR
  (that's a snapshot of a past pass). This is spec/reference, not a new decision.

---

## Related (separate, tracked elsewhere — do not fold in here)
- **srv item-8 read-endpoint design** (`GET /api/repos/{owner}/{repo}/flux` via pod SA)
  graduates to `spec/` with the rest of the walk rulings — that's walk *execution*, not
  de-confusion.
- **srv item-11 (RBAC/observability authz)** graduation → see
  `docs/scratch/2026-07-18-srv-rbac-observability.md`.
