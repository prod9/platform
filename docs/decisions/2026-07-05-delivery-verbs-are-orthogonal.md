# Delivery verbs are orthogonal; one publish engine, two drivers

Date: 2026-07-05
Status: **accepted**

## The ruling

`release` and `publish` are **orthogonal** app-image verbs — neither implies the other. There
is **no `deploy` verb**: in the pull model "deploy" is the operator committing the app-image ref
into the infra repo, then `ops publish` (with a platform server + Flux) or `ops render` +
`kubectl apply` (no server). The build+push logic is a single **publish engine** that runs under
two drivers — the local CLI now, a platform server later.

| Verb      | Its one job                                          | Produces                     |
| --------- | ---------------------------------------------------- | ---------------------------- |
| `release` | cut a version                                        | a git tag (immutable marker) |
| `publish` | build + push the image *for* a version               | a registry image + digest    |

There is no `deploy` row: deploying is the operator committing the ref (platform never rewrites
their CUE) + `ops publish`. Infra config is its own `ops render` / `ops publish` concern.

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
- **deploy is not a verb.** Getting a new image running is the operator committing its ref into
  the infra repo — platform never rewrites the operator's CUE (see
  [render-is-pure-function-of-committed-git](2026-06-26-render-is-pure-function-of-committed-git.md)) —
  then `ops publish` + Flux. The legacy build+promote `deploy` command has been removed.

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

## Deploy without a platform server (the manual path)

Until an app/site runs the platform server to drive its reconcile loop, the deploy step is
**manual and serverless**: `platform ops render` (→ `render`, once `ops` is flattened) emits the
`k8s/` manifests, and the operator `kubectl apply`s them directly — no OCI publish, no Flux, no
`deploy` verb. The server model automates the same intent (commit → `ops publish` → Flux pull);
the serverless path is just a human running render + apply by hand.
