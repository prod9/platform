# Delivery verbs are orthogonal; one publish engine, two drivers

Date: 2026-07-05
Status: **accepted**

## The ruling

`release`, `publish`, and `deploy` are **three orthogonal verbs**. Each does exactly one
thing; none implies another. The build+push logic is a single **publish engine** that runs
under two drivers — the local CLI now, a platform server later.

| Verb      | Its one job                                          | Produces                     |
| --------- | ---------------------------------------------------- | ---------------------------- |
| `release` | cut a version                                        | a git tag (immutable marker) |
| `publish` | build + push the image *for* a version               | a registry image + digest    |
| `deploy`  | point an environment at an **already-published** image | a committed ref → Flux pulls |

## Why this is written down (the conflation it heads off)

An agent reading the code will keep trying to **fuse these**, because under a CI model they
co-occur: you cut a tag and "the release" appears in the registry, so `release` looks like
it publishes. That fusion is an **artifact of the trigger mechanism**, not the domain. The
CI server watches tags and runs publish *for* you — the tag didn't publish anything; a
separate publish did, triggered by the tag.

Strip the trigger and the concerns separate cleanly:

- **release** is pure git — a marker in history. It builds nothing, pushes no image.
- **publish** is the only verb that produces an image. It is the local stand-in for what the
  CI server does; run it yourself when there is no server (right now) or for a project with
  no server-side host yet.
- **deploy** references an image that publish already made. It does **not** build. (Today's
  `deploy` still fuses build+promote; that fusion is legacy and gets de-fused toward the
  committed-literal ref — see
  [render-is-pure-function-of-committed-git](2026-06-26-render-is-pure-function-of-committed-git.md).)

## One publish engine, two drivers

The build+push sequence (open engine → `AttemptFrom` → `Build` → `Publish`) is a **reusable
unit in the `engine` package**, not logic trapped in a `cmd/` file. Two front-ends embed the
same unit:

- **local CLI `publish`** — runs the engine on your machine. You are standing in for the CI
  server.
- **platform CI server** (future) — watches version tags and invokes the *same* engine on a
  new tag. Explicit tag→image. **The trigger lives only in the server, never the CLI.**

Once a server exists, `publish` becomes automatic (the server does it on the release tag);
until then the local `release` → `publish` two-step *is* the flow.

## Rejected

- **Fuse `release` into `publish` (or vice-versa).** Imports the CI trigger-coupling into the
  domain. Rejected.
- **Local `publish` sends work to a remote server (RPC/remote build).** Wrong cut — the CLI
  should *be* a driver of the engine, not a client of a remote one. The server and the CLI
  are peers embedding the same engine; only the server adds tag-watch.
- **Tag-watch in the CLI.** The CLI never watches; watching is a server-only trigger. A tag
  growing an invisible "and now build" side effect is the implicit-CI magic this project
  bans elsewhere (Helm, GitHub Actions).
- **A guard against release-but-unpublished.** Accepted by convention — it is a fine state,
  observed harmless in practice. No code to prevent it.

## The design move this is an instance of

Cut to the one high-ROI concern per verb; handle the remainder **by convention, not code**
(no unpublished-guard, no remote-build plumbing, no fused command). When two things co-occur,
ask whether that is the *domain* or a *mechanism coupling* before merging them — default to
the narrower cut.
