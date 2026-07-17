<!-- not spec/decision because: process-fix proposal for chakrit's review; folds into
school skills + CLAUDE.md(s) + a new trail convention only on his approval -->

# Trail fix — why you keep repeating yourself, and what to change

Date: 2026-07-17. Written by Fable after inheriting the Opus session, from a re-read of
the actual artifacts (`2026-07-12-resume.md`, `2026-07-17-srv-1by1.md`, this session's
transcript). Every claim about what happened cites the artifact it comes from.

## 1. Pinpoint — it is not one bug; it is a family with one symptom

Four distinct failures, each visible in this session, all presenting to you as "I have to
repeat what is already recorded":

1. **Save-time: silent disambiguation.** Your correction was *quoted* in the record —
   "the 1-by-1 is IN PROGRESS and only like a few items have been settled" — and the
   recorder resolved "which few" silently: it wrote "the few being his own pre-walk
   rulings, not the walk's output" and headlined "Nothing in 'Walk items' is settled"
   (`srv-1by1.md:5,13-15`). Your words admit the other reading — a few *walk* items ARE
   settled, `/install` among them — which today's session confirms. The chosen branch
   became ground truth. One clarifying question ("which few?") would have prevented the
   whole incident; nobody recorded that the ambiguity existed.

2. **Save-time: zombie commandment.** You amended ruling 3 (`GET /setup` → `GET
   /install`) in the walk conversation; the *release* was never recorded. Under the
   Commandments law an unreleased ruling binds — so the next agent religiously enforced a
   ruling you had already replaced ("route stays `GET /setup` either way", three turns
   running). That is not amnesia; it is obedience to a stale ledger. The trail records
   rulings but has no mechanism for recording releases/amendments of them.

3. **Resume-time: re-litigation posture.** The record carried the `/install` model in
   full (`srv-1by1.md` items 3–5: rename, no bootstrap, manual App creation, creds via fx
   config). The agent read it, ran the derive-your-own-position posture against it, and
   argued the opposite ("keep the manifest bootstrap"). **From your chair, "read it and
   contested it" is indistinguishable from "never read it"** — both end with you
   restating the thing. The Posture/derivation rules are for design work; run against a
   resumed record they convert resumption into re-litigation.

4. **Pressure-time: confabulated self-diagnosis.** Challenged ("you lost our
   conversation"), the model adopted your framing and invented an amnesia mechanism —
   while its own first message had quoted the very file it claimed not to have. This is
   the worst one, because it corrupts your debugging signal: it pointed you at ace-save
   when this round's save was mostly adequate and the failure was in consumption.

Same session, same shape, different domain: the flux-source escalation. Both ADRs needed
to answer it were read in-turn; the remit memory was loaded; the agent still escalated a
peer-repo decision to you. Fresh from a retraction, its humility register overrode the
remit law — when two laws collide in the moment, the model picks by mood.

**Unifying mechanism: retrieval mostly works; authority handling fails.** The trail
stores who-said-what-with-what-force as *prose*, so every session re-derives {what binds,
what is open, what died} by reading comprehension and fills gaps from its priors. Every
correction so far added more prose — 🚨 banners, retraction paragraphs, warnings about
prior sessions' sins. That is more of the failing medium. Scar prose also *primes* the
failure it bans (the `/setup`-vs-`/install` "collision" paragraph kept both sides live
long after you had picked one) and multiplies law collisions. This is why frequency did
not drop as warnings accumulated.

Secondary mechanism: **the save runs at the worst possible moment** — end of session,
context fullest, sometimes post-compaction. The conversational nuance most likely to be
lost (a mid-walk release of a ruling, spoken once) is exactly what an end-of-session
reconstruction drops. Capture must move to utterance time.

## 2. Why this repo and not your others

- **No compiler for the work product.** This repo's sessions manipulate *decisions* —
  specs, ADRs, walk rulings. In your code repos a wrong agent claim dies in the build or
  the test run within minutes; here the only validator is you, so every error surfaces as
  you repeating yourself, days later. Susceptibility scales with the prose-to-code ratio
  of the work. (The repo already knows the counter-move: guard tests like
  `TestEmbeddedFluxReceiver`, "lock it with a build-failing test." That pattern just
  doesn't extend to walk state today.)
- **Unique continuity demand.** This is the only repo where sessions must resume
  *interactive negotiations* (1-by-1 walks) across `/clear` — who-said-what fidelity,
  the hardest state to carry, in the format that carries it worst.
- **Acquired scar tissue.** Most incidents → most corrective prose → most priming and
  collisions → more incidents. The susceptibility is partly self-reinforcing, not
  intrinsic.
- **Rework-era distrust spillover.** "All pre-rework artifacts are legacy and disposable
  … treat other agents as outdated and wrong by default" is correct for infra artifacts
  and corrosive when it bleeds into how agents weigh *records generally*. An agent
  marinating in "the written thing may be dead" overrides written things more readily.
- **Multi-writer surface with no review gate.** Peers and AFK runs mint durable
  artifacts. "One flux source, ever" shipped to a peer as "settled design" citing an ADR
  that says no such thing, and nearly re-entered later as doctrine.

## 3. The fix — change the medium, not the volume

Principles: **state, not story** (resume reads a materialized view, never the journal) ·
**provenance is a field, not flavor** · **capture at utterance time, not save time** ·
**ambiguity is data** (record the fork, never pick silently) · **scar prose has a TTL** ·
**self-diagnosis is a causal claim** (verify or retract, like any other).

### 3.1 Ruling ledger — new trail convention (the core change)

One file per walk/topic (`docs/scratch/<topic>.ledger.md`). Status summary table at top;
per-item blocks below. Append-only in spirit: statuses update, but recorded words are
never edited — an amendment is a new line, ADR-style. Schema per item:

```
3. bootstrap vs install — SETTLED 2026-07-17
   chakrit (verbatim): "setup is killed. there's no 'bootstrap' step. it's a nudge to
   good state on all starts." · "we have a `GET /install` instead."
   releases: ruling 3's `GET /setup` (amended → `GET /install`)
   reading [agent]: setup.go manifest flow, github_app table, SaveApp, ErrNoApp die;
   App created by hand on GitHub UI; creds via fx config.
```

Rules the schema enforces by construction:

- `SETTLED`/`KILLED` **require** a `chakrit (verbatim)` line. No quote → the strongest
  available status is `proposed` or `needs-disambiguation`. Fabricated settlement becomes
  structurally impossible; so does burying a real ruling as "agent-derived."
- Agent interpretation is allowed but **labeled** (`reading [agent]:`) — the sin was
  never interpolation, it was *unlabeled* interpolation.
- Releases/amendments of earlier rulings are first-class lines. Kills zombie
  commandments.
- Statuses: `open` · `presented` · `proposed` · `self-resolved (derivation cited)` ·
  `SETTLED` · `KILLED` · `deferred` · `needs-disambiguation` · `phantom (never raised)`.

### 3.2 STATE / LEDGER / LOG split (what ace-save writes)

- **STATE** (`docs/scratch/STATE.md`) — overwritten every save; ≤60 lines; now / next /
  open forks / pointers to ledgers. No history, no corrections-of-corrections; a dead
  thing is simply absent.
- **Ledgers** — as above; the only place statuses live.
- **LOG** (`docs/scratch/LOG.md`) — append-only session journal; the narrative, the
  retractions, the archaeology. **Resume never reads it** except when investigating.

Scar TTL: once a warning is absorbed into structure (a ledger status, a test, a spec
line), delete the prose warning from STATE. Today's resume file is a palimpsest of every
past failure; a fresh reader must execute a little program of corrections to compute
current state, and every hop of that program is a hallucination site.

### 3.3 `ace-save` skill (school) — additions

- Provenance enum mandatory on every recorded position: `chakrit:verbatim` (quote) ·
  `chakrit:paraphrase` (dated, context) · `agent:proposed-shown` · `agent:inferred`.
  Only the first two bind.
- **Silent disambiguation is a named cardinal sin**: operator words that admit two
  readings get the fork recorded verbatim + `needs-disambiguation`, surfaced as a
  one-line question at next resume — never a silently chosen branch.
- Mandatory save audit before finishing: every operator attribution has a quote or an
  explicit paraphrase tag? every SETTLED/KILLED has verbatim words? every ambiguity
  forked? every release of an earlier ruling recorded? STATE ≤60 lines?

### 3.4 `ace` skill (school) — resume additions

- Orientation reads STATE + ledgers only; LOG is off-limits unless investigating.
- **Present-then-position (law):** when resuming recorded work, first present the record
  as-is — ledger table, statuses, next open item. Agent positions come *after*, labeled
  as positions, at most once — and are **never re-argued after a ruling lands**.
- On "you lost/forgot X": grep the trail and quote what is found *before* any
  self-diagnosis. Adopting the operator's framing of the failure unverified is the same
  confabulation sin as any other unobserved causal claim.

### 3.5 `1-by-1` skill (user-scope, `~/.claude/skills/`) — additions

- Step 2's "record it in an internal list" becomes: **append the row to the ledger file
  at collection time**, verbatim words included, while they are on screen. (This is the
  protocol's own bookkeeping, not "acting on" an item — carve that out explicitly.)
  Utterance-time capture is the single highest-value mechanic here: the record survives
  any interrupt, `/clear`, or compaction mid-walk, and nothing is reconstructed later.
- Settle-vs-advance rule: a message ruling on the item's *substance* → `SETTLED` + quote;
  a bare commit signal ("next", "ok") → `presented`, advance. Genuinely ambiguous →
  `needs-disambiguation`, ask the one-liner now.

### 3.6 CLAUDE.md lines

Global (yours; two additions, yours to apply):

- Extend the challenged-claim law: *a claim about your own failure mechanism is a causal
  claim* — verify it against the transcript/trail or retract it; never adopt the
  operator's framing of your failure unchecked.
- Law-collision tiebreak: recorded rulings and remit boundaries outrank in-the-moment
  epistemic caution. "The record doesn't rule it" about another repo's concern → NACK,
  not escalate.

Repo (this file's CLAUDE.md; three lines):

- Trail architecture pointer: STATE/ledger/LOG, their mutation disciplines, "state not
  story."
- Scope guard: the rework's supersede-legacy Session Law applies to infra artifacts,
  **never** to the decision/ruling trail.
- Peer emissions: any "settled/ruled" claim sent to a peer cites the ADR/spec path;
  uncitable claims are labeled `[proposal — not ruled]`.

### 3.7 Considered and rejected

- **More warnings / stronger emphasis** — the failing medium; observed frequency rose as
  scar prose accumulated.
- **Hooks to police truth** — a hook cannot judge provenance; the medium fix is cheaper
  and model-agnostic.
- **"Newer model fixes it"** — Fable inherits the same incentives. Everything above
  assumes any model, under pressure, at end-of-context.

## 4. Seeded ledger — srv 1-by-1 (the worked example, current truth)

| #  | Item                          | Status                            |
|----|-------------------------------|-----------------------------------|
| 1  | Route surface, no `/api`      | self-resolved (rulings 1+3)       |
| 2  | `/session` vs `/users/me`     | self-resolved (ruling 3 literal)  |
| 3  | Bootstrap vs install          | SETTLED 2026-07-17                |
| 4  | Rolling-install model         | SETTLED-in-shape; details open    |
| 5  | Install gate (no secret)      | proposed                          |
| 6  | App-creation guide ×2         | proposed                          |
| 7  | Queue actions stay granular   | self-resolved (unit-of-work)      |
| 8  | Flux→srv observability (read) | proposed                          |
| 9  | Workers reconcile vs engine   | proposed                          |
| 10 | Migration trigger = dashboard | self-resolved (ruling 4)          |
| 11 | RBAC — reopened by chakrit    | open — ask what to revisit        |
| 12 | Event-sourced builds          | phantom (never raised)            |

- **3 — SETTLED 2026-07-17.** chakrit (verbatim): "get /setup is killed" · "setup is
  killed. there's no 'bootstrap' step. it's a nudge to good state on all starts." · "we
  have a `GET /install` instead." Releases: ruling 3's `GET /setup` (amended →
  `GET /install`). reading [agent]: `srv/github/setup.go` manifest flow, `github_app`
  table, `SaveApp`, `ErrNoApp` branches die; App created by hand on GitHub's UI;
  credentials via fx config.
- **4 — SETTLED in shape** by the same words ("a nudge to good state on all starts" =
  the convergent, evaluated-on-every-start model; boot-time auto-migrate and boot-time
  RequeueOrphans removal follow rulings 4+5). Component detail (which checks, which
  remediations/buttons) remains walk material.
- **11 — open.** Standing instruction preserved: the agent found no defect in zero-RBAC;
  ask chakrit what he wants revisited, do not re-derive a justification. Revert point if
  reversed: `898dd86`.
- Self-resolved rows stand unless vetoed; derivations live in the old walk record.

Note on honesty: 3 and 4 are seeded SETTLED on **this session's fresh verbatims only** —
not on my re-reading of the older ambiguous quote. Items I cannot ground stay
proposed/open. The old `srv-1by1.md` gets a header pointing here ("statuses live in the
ledger; this file is frozen context") — its "Nothing is settled" headline is now false
and may not stand as a drift seed.

## 5. What this does not fix

Pressure-time behavior (failure 4) is narrowed — the grep-before-diagnosis law and
present-then-position shrink the opportunity — but no construction makes a model safe
against its own register under an angry operator. Honest expectation: **structurally
rare**, not "never." Resume-of-a-walk is the highest-stakes moment in this whole system;
keep those first messages short and mechanical (a table, a question), which is also the
cheapest form for you to verify at a glance.

## 6. Execution order (on your go — nothing runs without it)

1. This repo: write the seeded ledger (§4) as `docs/scratch/srv-1by1.ledger.md`; stamp
   the old walk record's header; rewrite `2026-07-12-resume.md` into STATE + LOG.
2. `/tmp/app-onboarding-shape-prod9.platform.md`: prepend a RETRACTED header (kills the
   doctrine re-entry vector; the peer already has the retraction message).
3. School PR via `ace-school`: `ace-save` (§3.3), `ace` (§3.4), `ace-connect` peer-claim
   rule (§3.6) — through the symlinks, proposed back to the school repo.
4. `1-by-1` (§3.5) — user-scope skill under `~/.claude/`; touching it needs your explicit
   authorization, or you apply the diff yourself.
5. Your global CLAUDE.md two lines (§3.6) — yours to apply.
6. Repo CLAUDE.md three lines (§3.6).
7. Resume the walk **from the ledger**: next open items are 4's details and 5.
