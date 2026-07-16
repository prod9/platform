# The scaffolded baseline is provider-neutral

Date: 2026-07-16
Status: **accepted**

## The ruling

**Platform ships no cloud-provider-specific content — no Linode annotations, vars, or
instructions in the skel baseline, ever.** A scaffolded repo must come up the same on any
conformant cluster (Linode, DigitalOcean, anything with a CCM and LoadBalancer Services).

Provider wiring — reserved/static LB IPs, firewall attachment, any
`service.beta.kubernetes.io/*` annotation — is the **infra repo's own edit**: the
operator patches their committed copy of the scaffolded files (e.g. an annotations `set`
directive on the NginxProxy service patch in `apps/nginx-gateway-exp.platform`). The
scaffolded files are the operator's files; provider specifics live and evolve there.

**Scope — what neutrality does NOT mean.** The cert-manager + Gateway + ListenerSet
stack is the **cross-cloud convention** and stays fully in the baseline: the NGF gateway,
the gateway-api/listenerset controller flags, the host-agnostic `Gateway` app, the ACME
cluster-issuer, per-component `ListenerSet`s. Provider-specific is only what names a
cloud: `service.beta.kubernetes.io/<cloud>-*` annotations, provider IDs/IPs, provider
vars. Do not strip the stack in neutrality's name.

Equally: **don't overbuild the neutrality.** No provider abstraction, no plugin system,
no per-cloud component variants, no conditional DSL constructs to make one shipped file
serve every cloud (the rejected `set-unless-empty` verb was exactly this). Neutral means
*absent*, not *parameterized*.

**Why scaffold-to-files is the mechanism:** init writes the baseline to real files in the
infra repo — deliberately not hard-embedded behavior — precisely so the operator/agent can
customize them per cloud. The DSL exists for the same reason: the set/edit machinery is
implemented once in platform; each infra repo declares its own cloud's edits in its
committed `.platform` files.

## Context

The baseline briefly shipped Linode CCM annotations (`linode-loadbalancer-firewall-id`,
then `-reserved-ipv4`) fed by `DefaultVars`. Live bring-up showed the empty-value default
was apply-breaking on the exact provider it targeted, and the fix direction (a
conditional DSL verb) was bending a deliberately branch-free language around one cloud's
annotation. Wrong layer both times.

## Enforcement

`TestEmbeddedNginxGateway` asserts the rendered baseline contains no `linode` string —
the guard against re-scaffolding provider content.
