# NGINX Gateway Fabric / Gateway API install recipe

Lookup facts for the NGINX Gateway Fabric (NGF) + Gateway API baseline install: upstream
URLs, the firewall-annotation patch, the `serverTokens` workaround, and the
string-forcing constraint. Live baseline directive:
[`apps-nginx-gateway.platform`](../../framework/skel/apps-nginx-gateway.platform)
(standard channel — the experimental variant was dropped 2026-07-17: its extras,
TCPRoute/UDPRoute, had no working consumer; a repo needing them edits its committed
component).

**Provenance.** Recipe captured 2026-06-19 from `prod9/infra-cli`
(`cmd/nginx_gateway_cmd.go`, the CLI platform replaces). Upstream ships plain
pre-baked YAML — no Helm at render time. Versions below are the pinned defaults; they may
have moved upstream. Version pins live in `framework/baseline.go` `DefaultVars`
(interpolated as `\(var)` into the `download` URLs); they are not selection knobs.

## Upstream sources

Three `download`→`emit` steps land in `k8s/nginx-gateway/`. `\(gateway_api_version)` and
`\(nginx_gateway_version)` interpolate from `[vars]`.

| Step             | URL                                                                                                                    | emit                      |
|------------------|----------------------------------------------------------------------------------------------------------------------|---------------------------|
| Gateway API CRDs | `github.com/kubernetes-sigs/gateway-api/releases/download/\(gateway_api_version)/{standard,experimental}-install.yaml` | `gateway-api-crds.yaml`   |
| NGF CRDs         | `raw.githubusercontent.com/nginx/nginx-gateway-fabric/\(nginx_gateway_version)/deploy/crds.yaml`                       | `nginx-gateway-crds.yaml` |
| NGF controller   | `raw.githubusercontent.com/nginx/nginx-gateway-fabric/\(nginx_gateway_version)/deploy/default/deploy.yaml`             | `nginx-gateway.yaml`      |

- **Gateway API channel**: the baseline installs `standard-install.yaml` only.
  `experimental-install.yaml` adds TCPRoute/UDPRoute (both experimental in NGF 2.6.x) —
  swap the URL in your repo's committed component if you need them; note stage9 observed
  NGF 2.6.0 never programming a Gateway-attached TCPRoute (unresolved as of 2.6.7).

## Pinned versions (`DefaultVars`, as of 2026-06-19 capture)

| Var                          | Default      |
|------------------------------|--------------|
| `GATEWAY_API_VERSION`        | `v1.5.1`     |
| `NGINX_GATEWAY_VERSION`      | `v2.6.0`     |
| `NGINX_GATEWAY_FIREWALL_ID`  | `"11222746"` |

## Controller manifest patches

Applied to the `NginxProxy` doc in `deploy.yaml`. Both baseline files carry the same
four lines:

```
focus .[].kind "NginxProxy"
set .spec.serverTokens "off"
set .spec.kubernetes.service.patches[0].type "StrategicMerge"
set .spec.kubernetes.service.patches[0].value.metadata.annotations."service.beta.kubernetes.io/linode-loadbalancer-firewall-id" "\(nginx_gateway_firewall_id)"
```

- **`serverTokens=off`** — NGF 2.5.1 bug workaround.
- **Cloud LB annotations** — appended as a StrategicMerge patch under
  `NginxProxy.spec.kubernetes.service.patches`. NGF v2's CRD exposes no
  `service.annotations` field; the `patches` list is the only hatch to set provider
  Service annotations (e.g. Linode's `…/linode-loadbalancer-firewall-id` or
  `…-reserved-ipv4`). `set` auto-vivifies `patches[0]…` from nothing. These lines are
  the **infra repo's own edit**, never scaffolded — see the
  [provider-neutral ADR](../decisions/2026-07-16-baseline-is-provider-neutral.md).

## String-forcing constraint

A k8s annotation value must be a **string** (`annotations` is `map[string]string`); a
bare int is invalid. The firewall id (`11222746`) is numeric-looking, so it must be
forced to a string.

Mechanism: in `set`, a **quoted** token is a string literal — it stringifies the
interpolated value. A **bare** token is a variable reference that preserves its native
type. So `"\(nginx_gateway_firewall_id)"` (quoted) keeps the annotation a string;
`\(nginx_gateway_firewall_id)` (bare) would coerce to int. No `set-string` verb exists —
quoting is the whole answer. Same applies to `serverTokens "off"` (quoted → string).

## Divergence from the 2026-06-19 source note

The recipe still matches what platform installs. Differences from the note's prose:

- Syntax modernized (already flagged stale in the note): `select` → `focus`;
  `[field=value]` selectors gone.
- **Two baseline files, not one.** The note asserted experimental-only with no choice
  group; platform now ships both a stable (`standard-install.yaml`) and an experimental
  (`experimental-install.yaml`) variant. `init` installs the experimental one (the
  default working set); the stable file ships in the binary for an operator who swaps
  it in by hand.
- **String-forcing blocker resolved** via the quoted-literal rule above (no new verb).
- `[vars]` keys are now uppercase env-style (`NGINX_GATEWAY_FIREWALL_ID`),
  normalized to lowercase for both `\(var)` directives and CUE `@tag` holes.
- `flux_version` is pinned (`v2.8.8`); unrelated to NGF but was `?` in the note.
