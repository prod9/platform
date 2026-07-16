# Flux webhook delivery: push-driven reconcile, poll as fallback

Date: 2026-07-13
Status: **accepted**

## The ruling

The `Infra` baseline ships **push-driven delivery** as the primary reconcile trigger. On a
GHCR publish GitHub fires the `registry_package` webhook; a Flux `Receiver` (`type: github`)
validates the HMAC signature and pokes the `infra` `OCIRepository`. The `OCIRepository`
poll interval drops from `1m` to `10m` — it is now only the **dropped-webhook fallback**,
not the delivery path.

> **Latency correction (2026-07-17, measured on prod9-main):** "near-instant" holds only
> from event to cluster (receiver → fetch → apply: seconds). GitHub delivers
> `registry_package` on a throttled cadence — ~6m observed steady-state, 10–24m under
> publish surges — so end-to-end publish→cluster is **minutes**. The webhook's value is
> beating the poll's worst case and decoupling from poll misses, not instancy.

Three resources join `apps/flux-sync.cue`, all in `flux-system`:

- the `Receiver` (events `[registry_package]`, resources → the `infra` `OCIRepository`);
- its HMAC token `Secret` (`flux-webhook-token`), an **empty committed placeholder** the
  operator hand-fills — same convention as the registry creds, never prompted or injected;
- an `HTTPRoute` exposing notification-controller's `webhook-receiver` service (`:80`) at
  `FLUX_HOSTNAME`, attached to the operator's gateway.

The two ingress hosts become render-time vars in `DefaultVars`: `FLUX_HOSTNAME` (the receiver
route) and `PLATFORM_HOSTNAME` (the platform app's existing `#host`, previously a literal).
Gateway coordinates (`nginx` / `gateway`) stay **literal** — one standard gateway per cluster;
an operator who deviates edits the committed CUE.

## Scope: cluster-side only for now

This slice ships everything **cluster-side**. The GitHub-side webhook — pointing the repo at
`https://<FLUX_HOSTNAME>/hook/<path>` with the token and `registry_package` event — is
**configured by the operator by hand**, the same manual convention as installing Flux and
committing the infra repo at this stage. Automating it needs the GitHub App and the `srv`
layer, neither of which exists; per [platform-server](../spec/platform-server.md)'s sequencing
(prove the CLI path first), that automation lands when `srv` does. Shipping the cluster half
now is not a workaround — it is the pull-model half platform owns; the push-config half is
GitHub's, gated on the App.

> **2026-07-17:** the GitHub-side automation now exists — `srv` mints an installation
> token from the stored App and `POST /api/repos/{owner}/{repo}/flux-webhook`
> (`srv/flux_webhook.go`) creates the repo's `registry_package` webhook pointing at the
> Receiver URL with its HMAC secret. Manual wiring remains the fallback.

## Why written down

Two conflations to head off:

- **Webhook vs poll is not either/or.** The poll interval survives deliberately as the
  dropped-webhook safety net. A future edit that removes the `Receiver` "because Flux already
  polls" reintroduces per-interval delivery latency — the exact regression this heads off. A
  blocking `framework/` test (`TestEmbeddedFluxReceiver`) plus an at-site comment guard the
  `Receiver` against that relapse.
- **The hosts are config, not the gateway.** Only the per-deployment hostnames varify; the
  gateway identity stays a literal. Making the gateway name a var buys nothing — clusters run
  one gateway — and spreads a non-standard name across the baseline.
