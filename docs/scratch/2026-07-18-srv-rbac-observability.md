<!-- not spec/decision because: srv 1-by-1 walk derivation for item 11 (RBAC + cluster-
observability authz). SETTLED in shape (2026-07-18) but not yet executed — graduates to
spec/ (the srv observability/authz surface) + an annotation on the zero-RBAC ADR in the
post-walk execution pass. Kept in full because the short ledger summary loses the reasoning
and re-confuses a fresh session. -->

# srv item 11 — RBAC & cluster-observability authz (SETTLED 2026-07-18)

**Outcome: zero-RBAC confirmed — no reversal.** The walk stress-tested whether a pure
GitHub-token model supports *every* srv action (especially viewing cluster/flux rollout
state) and found it does, given two enablers (provenance discovery + repo-first IA). No
platform-side RBAC, roles, or permission tables are introduced.

Provenance: worked live with chakrit 2026-07-18; verbatim rulings quoted where they land.

## Starting model (from ADR `2026-06-29-platform-server-github-app-zero-rbac`)

1. srv authenticates as a **GitHub App**; **zero platform-side RBAC** — no roles, no
   permission tables. Authorization is fully delegated to GitHub.
2. Can access the repo → can trigger its builds. **Deploy authority = git-push on the infra
   repo.** Never a single god credential.
   - **Note (2026-07-18): there is no app-repo → infra-repo linkage, and there shouldn't
     be.** The deploy flow is a manual human handoff across two permission domains: publish
     a release on the **app** repo (build + push image) → a user with **infra**-repo access
     updates the tag/image-ref in the infra repo → push there triggers the build/reconcile.
     The app-repo publisher and the infra-repo committer may be different people; platform
     doesn't bridge them — the user knows the relationship. ⇒ **cluster-view authz gates on
     the infra-repo rights.**
3. The DB holds **near-nothing RBAC-wise** — only authN/identity (user + session), the
   install record (item 5), and domain data (builds / `BuildEvent`s). Authz is computed
   live, never stored.
4. **Webhook-triggered builds use the installation token** (`platform[bot]`) — the
   autonomous path, and the one that needs an explicit access check. The user-to-server
   token path gets authz implicitly, bounded to the user's own reach.

## The stress-test: does a pure GitHub token cover every srv action?

5. **Cluster-view (flux rollout state) authz — yes, and with no RBAC.** The user token
   gates WHO (a GitHub repo-access check, `checkRepoPush`-style); the **pod ServiceAccount**
   does the privileged cluster read. The backend holds the cluster creds; the user's GitHub
   identity only *authorizes the request*.
6. **repo → namespace mapping is the real gap — and it is resource routing, not authz.**
   Must not be conflated with an RBAC need (domain-vs-mechanism).
7. It is **not derivable from the repo name.** Counterexample (chakrit): `bluepages-infra`
   houses many instances of the bluepages codebase; one is named `haachang.com` running in
   namespace `s9-haachang`. Instance + namespace names are chosen in the infra CUE. One
   infra repo → N arbitrarily-named namespaces. You cannot compute `s9-haachang` from
   `bluepages-infra`.
8. **But discoverable from the cluster, not stored.** A flux `Kustomization` →
   `sourceRef` → `OCIRepository` → infra image → repo. So `namespace → infra-repo` is
   recoverable by reading cluster state via the pod SA.
   - **Note (2026-07-18): don't touch the baseline for this yet.** (A baseline-stamped
     provenance label was proposed for a deterministic reverse lookup — deferred.) For now,
     discover provenance from **existing** cluster/flux metadata (the `sourceRef` →
     `OCIRepository` → image chain); no new baseline labels added just for srv.
   - **The discovered provenance is cached into the session data** (see Caching).
9. **Authz flow:** namespace → provenance (SA read, from existing metadata) → infra-repo →
   GitHub rights check → serve via SA.

## Performance

10. The **hot path (one namespace → repo) is a targeted read, not O(n)** — read that
    namespace's `Kustomization` / `sourceRef`. The GitHub rights check likely *dominates*
    latency over the cluster read.
11. The **reverse (repo → all its namespaces) is a server-side label/field-filtered list** —
    one call, only matches returned. Fine at realistic scale (hundreds–low-thousands of
    objects). (k8s labels aren't truly indexed in etcd, so the apiserver still does an
    internal list+filter, but it's one filtered call, not a platform-side scan.)
12. **No k8s watch/informer index.** It was proposed as the O(1) escalation and **rejected
    as over-engineered** (chakrit:verbatim: "no thanks, that seemed convoluted and
    over-engineered. we'll just do some smart caching if this ever is needed."). If perf
    ever bites, do smart caching in the session — not an informer.

## UX / information architecture

13. **Reject namespace-first selection** — listing namespaces discloses deployment
    names/existence to unauthorized viewers. A leak.
14. **Repo-first** is the model *and* the natural IA of a GitHub-gated tool: you only ever
    see your installation repos, and you navigate repo → its namespaces. The authz gate and
    the information architecture are the same shape.
15. **Repo list at login = one GitHub call** — the user's repos *within the installation*
    (platform's managed set ∩ the user's access). Exact endpoint = `docs/vendor/` confirm
    (GitHub has a list-installation-repos-for-user call; not asserting the path).

## Caching — the "special" platform session

- **The platform session is not a bare token — it holds more data** (chakrit:verbatim:
  "Perhaps session token needs to be quite special for platform. Holding more data than just
  token."): the cached repo list, the discovered repo↔namespace provenance, and rights.
  Server-side and controllable. It stays a **cache** of GitHub/cluster truth, so zero-RBAC
  holds — the session is *not* an authoritative grant store.
- This is the **single caching layer** (repo list + provenance), discovered once and reused
  for the session. It is the "smart caching" that removes any need for an informer.
- **Rights-derived cache needs a TTL / refresh** so a revoked GitHub access can't linger in
  a stale session — the cache speeds reads but must not outlive the grant it mirrors.
- **Authz is always re-checked server-side per action.** A stale or tampered client-side
  copy of the repo list can't grant access — the server re-verifies GitHub rights on the
  specific repo before serving its cluster state.

End-to-end flow:

> login → list *your* installation repos (cached in the session) → pick a repo → server
> re-checks your rights on it → provenance lookup (existing cluster metadata, cached) →
> that repo's namespaces + flux rollout state.

## Net conclusion

16. **Zero-RBAC holds, confirmed — no reversal, no platform RBAC.** Cluster observability is
    carried by: infra-repo GitHub rights (authz, server-side, re-checked per action) +
    provenance discovered from existing cluster data (no baseline change) + a fat platform
    session for caching (repo list + provenance; rights with a TTL) + repo-first IA (no
    namespace leak). No informer. The app→infra handoff stays manual.

## Graduation (post-walk execution)

- The **authz model** in this doc is settled and graduates to `spec/`. The full
  observability **surface** (endpoint set, response shapes, webui) is **not designed here** —
  item 8 is one endpoint skeleton; a surface spec is a later design pass, not a graduation of
  this doc.
- The **zero-RBAC confirmation** (no reversal) → an annotation on ADR
  `2026-06-29-platform-server-github-app-zero-rbac` — it already covers build/deploy authz;
  extend it (or the spec) to the observability read path.
- **Deferred dependencies:** baseline provenance labels (only if existing-metadata discovery
  proves insufficient); the exact GitHub list-installation-repos endpoint (vendor confirm).
