<!-- derived from: prod9/infra-cli cmd/nginx_gateway_cmd.go @ 2026-06-19;
     local baseline consistency checked against framework/skel/apps-nginx-gateway.platform
     and framework/platform_infra.go @ 2026-10-04; upstream not re-verified -->

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
have moved upstream. Version pins live in `framework/platform_infra.go` `infraVars`
(interpolated as `\(var)` into the `download` URLs); they are not selection knobs.

## Upstream sources

Three `download`→`emit` steps land in `k8s/nginx-gateway/`. `\(gateway_api_version)` and
`\(nginx_gateway_version)` interpolate from `[vars]`.

| Step               | URL                                                                                                                    | emit                        |
| ------------------ | ---------------------------------------------------------------------------------------------------------------------- | --------------------------- |
| Gateway API CRDs   | `github.com/kubernetes-sigs/gateway-api/releases/download/\(gateway_api_version)/standard-install.yaml`                | `gateway-api-crds.yaml`     |
| NGF CRDs           | `raw.githubusercontent.com/nginx/nginx-gateway-fabric/\(nginx_gateway_version)/deploy/crds.yaml`                       | `nginx-gateway-crds.yaml`   |
| NGF controller     | `raw.githubusercontent.com/nginx/nginx-gateway-fabric/\(nginx_gateway_version)/deploy/default/deploy.yaml`             | `nginx-gateway.yaml`        |

- **Gateway API channel**: the baseline installs `standard-install.yaml` only. Using
  TCPRoute/UDPRoute needs **two** edits in your repo's committed component: the
  `experimental-install.yaml` CRDs **and** the NGF controller started with
  `--gateway-api-experimental-features` (upstream's `deploy/experimental/deploy.yaml` —
  its only flag delta vs `deploy/default/`). Missing the flag is silent: NGF ignores the
  route entirely (empty status, `attachedRoutes=0`) — this was the actual cause of
  stage9's "dead" pmrelay TCPRoute; the CRDs alone were never enough.

## Scaffolded defaults (local consistency check, 2026-10-04)

| Var                            | Default        |
| ------------------------------ | -------------- |
| `GATEWAY_API_VERSION`          | `v1.5.1`       |
| `NGINX_GATEWAY_VERSION`        | `v2.6.7`       |

## Controller manifest patches

The scaffold applies only the provider-neutral `serverTokens` patch to the `NginxProxy`
document in `deploy.yaml`:

```
focus .[].kind "NginxProxy"
set .spec.serverTokens "off"
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

A Kubernetes annotation value must be a **string** (`annotations` is `map[string]string`).
For operator-authored provider patches, use the existing DSL's quoted string form and
follow its [value grammar](../spec/manifest-patch-dsl.md); this reference defines no DSL
syntax of its own. The baseline's `serverTokens "off"` is a quoted string.

## Divergence from the 2026-06-19 source note

The original capture describes historical provider wiring. The current local baseline:

- Syntax modernized (already flagged stale in the note): `select` → `focus`;
  `[field=value]` selectors gone.
- **One standard-channel component.** The experimental variant is not scaffolded; an
  operator needing experimental routes edits the committed component as described above.
- **String-forcing blocker resolved** via the quoted-literal rule above (no new verb).
- `[vars]` keys are uppercase env-style (`NGINX_GATEWAY_VERSION`),
  normalized to lowercase for both `\(var)` directives and CUE `@tag` holes.
- `flux_version` is pinned (`v2.8.8`); unrelated to NGF but was `?` in the note.
