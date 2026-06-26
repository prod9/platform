# School-edit handoff — afk pre-flight + realign auto-trigger (2026-06-26)

Staging artifact for the **ace-school** agent. These three edits land in the PRODIGY9 Coding
School (skills are symlinked from the school clone; edits go through the school, not the local
file). Relayed over ace-connect. Delete this note once applied.

Source: the law/trend + afk/realign redesign session (chakrit, 2026-06-26). The personal-side
counterparts (`~/.claude/CLAUDE.md` Laws taxonomy, personal `realign`) are already applied
directly; these are the school-shipped mirrors.

Design context — the Law/trend taxonomy these reference:

- **Laws** — binding tier, arm realign. Scoped: *global* (cross-repo), *per-repo* (marked in a
  project CLAUDE.md), *session* (binds for a phase, then expires).
- **Watch-trends** — soft, self-monitor, never arm.

The school skills must stay generic — reference "a loaded surface's Laws designation", never any
one user's personal CLAUDE.md.

---

## Edit 1 — `skills/ace-realign/SKILL.md`

Mirror of the personal `realign` auto-trigger, generic. Two changes.

### 1a. Description — append the auto-trigger clause

Replace the `description:` block with:

```yaml
description: >
  Force re-attention on a rule you keep breaking — repeat it verbatim at the start
  or end of every message until the session ends or the user says stop. TRIGGER on
  "realign" when the user calls out a broken rule that already lives in a loaded
  surface (CLAUDE.md, a skill, an explicit earlier instruction). ALSO AUTO-TRIGGER
  without being asked: on the second violation of the same Law (a rule a loaded
  surface designates a Law) within a session, self-engage on that Law. DO NOT TRIGGER
  for first-time rule capture with no prior violation, when the user merely disagrees
  with an output rather than citing a broken rule, or — on the auto path — for
  watch-trends or a Law's first violation.
```

### 1b. Body — insert before `When triggered:`

```markdown
## Auto-trigger on Laws (no invocation needed)

Some rules are designated **Laws** in a loaded surface — a CLAUDE.md may mark a Laws set
with global, per-repo, or session scope. Laws bind harder than ordinary instructions.

Each turn, self-audit your last action against the active Law set. On the **second**
violation of the **same Law** within a session, **arm this protocol immediately on that
Law** (run the steps below) — do not wait for the user to say "realign", do not make them
re-state the frustration. The first violation: fix it and move on, no arming.

Only **Laws** auto-arm. Watch-trends are self-monitored, never armed. Honor scope: a
session Law arms only within its phase, a per-repo Law only in its repo.
```

---

## Edit 2 — `skills/ace-afk/SKILL.md`

Two changes: add a **pre-flight phase** before the loop, and point the loop at the new
`workflow-afk.md` instead of "the ace workflow with one substitution."

### 2a. Insert a pre-flight section before `## Run the loop`

```markdown
## Pre-flight — before the unattended loop engages

Run this while the human is still reachable. It front-loads every decision so the
unattended body needs none. This phase is the *only* sanctioned asking window.

1. **Restate the understood end-goal.** "Understood: <goal>." Include the definition of
   *done* — the real deliverable in the real target (the repo actually changed, the thing
   actually live), never a /tmp render or staged plumbing. If the goal is ambiguous, this is
   the moment to ask.
2. **Clear blockers — go HARD.** Surface and resolve every fork, missing input, and decision
   now, while the human can answer. This is where all the asking is spent; the body gets
   none. Apply **Earn the blocker** before flagging anything as needing the human.
3. **Establish the decision-basis.** State the philosophy the run resolves forks against,
   derived from the repo's CLAUDE.md + the goal. This is what makes "no questions after Go"
   safe rather than reckless: the body resolves forks against the basis and records the
   choice, instead of stopping to ask.
4. **State the AFK plan, then wait for explicit "Go."** Go is the last gate. After it: no
   questions, no go-gate — drive the loop to the envelope.
```

### 2b. Replace the loop pointer

The current `## Run the loop` says to read the ace `workflow.md` and drive it with one
substitution. Replace that body with:

```markdown
## Run the loop

After Go, read `workflow-afk.md` in the `ace` skill directory and drive it autonomously to
the envelope below. It is the ace workflow with every propose/confirm gate already removed —
no stop-to-ask, no stop-to-plan. Honor `$ARGUMENTS` as the focus if given.
```

### 2c. Add `Earn the blocker` to the blocker section, and `don't pause at milestones`

In `## Don't block — log it`, add:

```markdown
**Earn the blocker.** Before logging any blocker for a missing input — example, fixture,
dependency, test target — exhaust manufacturing it yourself: fetch a public sample, download
a real package, write a dummy/stub, synthesize minimal scaffolding. Only a resource you
genuinely cannot obtain or construct is a real blocker.

**Don't pause at milestones.** A completed goal, a clean checkpoint, or a discretionary fork
is not a reason to stop and report-and-ask. Resolve by the decision-basis, record the choice,
keep the loop driving.
```

---

## Edit 3 — new file `skills/ace/workflow-afk.md`

The ace workflow with gates stripped and the kue "keep going" body rules folded in. Full
content:

```markdown
# ACE Workflow — Unattended (AFK)

This is the ace workflow with every propose/confirm gate removed, for unattended runs under
`ace-afk`. The gates are replaced by the afk **envelope** (no push/publish/deploy, no
global-state mutation, no working-tree destruction, commit-don't-push) and the pre-flight
**decision-basis** established before the run. Forward motion is the default; stopping is the
exception.

Standing rules for the whole run:

- **Resolve forks by the basis, don't ask.** Apply the decision-basis from pre-flight, record
  the choice in the durable record, move on. Surface a fork only when the basis is genuinely
  silent *and* the choice is expensive to reverse — and even then, in afk that is a logged
  blocker, not a stall.
- **Record decisions as you make them, not as questions** — into the breadcrumb / durable
  record, which is the crash-safe restore + fork point.
- **Don't pause at milestones.** A completed goal or clean checkpoint is not a stop-and-ask.
- **Earn the blocker.** Exhaust manufacturing a missing input (fetch/stub/synthesize) before
  logging it as needing the human.
- **Thin orchestrator.** You drive; one subagent per slice does the work in fresh context.
  You accumulate only slice summaries, so the loop survives long runs.

## Orientation

Same as the attended workflow: figure out where you already are from conversation, git state,
loaded skills, and in-progress tasks before starting at step 1.

## Task discovery

1. **Cleanup** — check `git status`/`git diff`. Uncommitted coherent work from a prior slice:
   commit it on the current branch (envelope: commit, don't push). Don't proceed on a dirty
   tree.
2. **Surface** — read the storage cascade; collect pending tasks, open questions, blockers.
3. **Select** — pick the next task by the decision-basis and record it. No propose-and-wait;
   the basis decides. Identify which skills the slice needs.

## Planning

4. **Specs** — read the project's source of truth; extract acceptance criteria; note gaps.
5. **Draft plan** — list every change (specs first, then tests, then code), file by file.
6. **Simplify plan** — cut to an elegant just-enough fit; prefer deletions; don't cut
   spec/called-out edge cases.
7. **Test plan** — define validation before implementing; TDD by default (failing test
   first); name the substitute verification where TDD doesn't apply.
8. **Record the plan** in the durable record and proceed. No confirm gate — the basis and the
   envelope replace it. If the plan exceeds the decision-basis (a genuinely silent, expensive,
   irreversible fork), log a blocker and pick up the next unblocked slice.

## TDD execution

Size before editing: single-file work stays in-context; multi-file or cross-module work goes
to an isolated subagent per non-overlapping file group.

9. **Red** — add/update tests first; confirm they fail for the expected reason. State the
   exception + substitute verification if TDD doesn't apply.
10. **Green** — smallest change that satisfies the tests; stay within the recorded plan.
11. **Refactor** — clean up without behavior change; prefer deletions; elegant just-enough.
12. **Verify** — run the planned narrow + broad checks; loop red/green/refactor on a missing
    case; substitute the closest useful check if one can't run.

## Review and close

13. **Audit** — re-read every changed file (not just diffs). Categorize findings (Violation /
    Borderline / Out-of-scope). Fix every Violation and re-audit; the audit converges. Run
    tests + lints.
14. **Commit** — commit on the current branch using the repo's commit convention.
    **Envelope: do not push, publish, release, or deploy** — those wait for the human.
15. **Checkpoint** — update the breadcrumb / durable record (what landed, what's next, open
    blockers) so a crash or compaction leaves a clean restore point. No `/ace-save` or
    `/clear` between slices — the subagent boundary gives fresh context, the breadcrumb gives
    continuity.

## Two-phase audit every 2–3 slices

Spawn audit subagents: (A) code-quality (correctness, DRY, test strength, skill compliance over
the batch), then (B) architecture/cleanup (boundaries, layering, dead code, simplification over
the module graph). Fold findings into the plan as fix-slices; don't let them stall forward
motion.

## Loop or stop

Verify passes + slices remain → spawn the next. Stop only at a genuine blocker (basis-silent +
expensive + irreversible, logged), a failed verify the subagent couldn't fix, or an empty plan
— leaving the breadcrumb pointing at the next step. On stop, write the run summary to `.afk.log`.

## Storage cascade

Same as the attended workflow ($ARGUMENTS → built-in tasks → agent inbox → task tracker →
scratch files → git state).
```

---

## Relay

Send to the ace-school agent over ace-connect once it's online; it owns applying these to the
school clone + the school's PR workflow. The personal-side edits are already live.
