# defs wishlist — Dagger engine (for the infra-defs agent)

Platform is authoring an in-cluster Dagger build engine as a CUE app (`apps/dagger-engine.cue`)
on `prodigy9.co/defs`. Topology + rationale:
`platform/docs/decisions/2026-06-21-dagger-engine-statefulset-tcp.md`. One change is a hard
blocker; one is a nice-to-have. Ping platform when a version carrying these is published — it
pins the new `defs@vX.Y.Z` and finishes render-verify.

## 1. `#Service` headless support — REQUIRED (blocks the engine)

A StatefulSet of engine pods is discovered by platform resolving the **headless** Service's
A-records into per-pod IPs (no k8s API / RBAC). That needs `clusterIP: None`, but
`defs.#Service`'s `spec` is **closed** (no `...`), so it can't be set from the call site.

Proposed contract (platform codes against this):

```cue
#Service: Self={
    // …existing…
    #headless: bool | *false

    spec: {
        // …existing…
        if Self.#headless { clusterIP: "None" }
    }
}
```

- Default `false` → byte-identical to today (no `clusterIP` key emitted).
- `#headless: true` → `spec.clusterIP: "None"`, everything else unchanged.

## 2. `#StatefulSet` per-replica volume claims — NICE-TO-HAVE

The engine needs a per-ordinal cache PVC (`volumeClaimTemplates`) + a container `volumeMount`
at `/var/lib/dagger`. Platform **inlines** this today via the open `spec` — no dependency on
you. But it's a general StatefulSet need (postgres/redis would benefit), so a first-class
feature would be the proper home. Rough shape:

```cue
#StatefulSet: Self={
    // …existing…
    #volume_claims: [Name=string]: {#storage: string, #storage_class?: string, #path: string}
    // emits spec.volumeClaimTemplates[] + the matching container volumeMounts[]
}
```

No rush — if/when this lands, platform switches the inline block over. Until then platform's
`engine.cue` carries the raw `volumeClaimTemplates`.

## Already sufficient in current defs

`#privileged` (sets `securityContext.privileged: true`, subsumes `CAP_SYS_ADMIN`), `#args`,
`#port`/`#ports`, `#service_name`, and `parts.#PodSpread` (node spread) all cover the rest —
no changes needed there.
</content>
