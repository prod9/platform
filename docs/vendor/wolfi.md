<!-- derived from: cgr.dev/chainguard/wolfi-base:latest (apk queried directly) +
https://edu.chainguard.dev/open-source/wolfi/overview/ @ 2026-07-26 -->

# Wolfi (our base image)

Every framework but `Dockerfile` and `Infra` builds on `cgr.dev/chainguard/wolfi-base:latest`
(`framework/base.go:32`). Wolfi is Chainguard's rolling, glibc-based distro. Our side of the
contract — the FHS tree, the package sets, the never-pin rule, the apk cache mount — is
[`../spec/frameworks.md`](../spec/frameworks.md); this file is only for looking *their*
packaging up.

## 🚨 Wolfi uses apk. Wolfi is not Alpine.

The package manager is the same tool and a great many names coincide, so an Alpine answer
is right often enough to pass review — and wrong without warning. `pkgs.alpinelinux.org` is
**not evidence about this image**, and neither is recall of what Alpine ships. Resolve
against Wolfi or don't claim it.

Wolfi also splits packages more aggressively than Alpine and ships a near-empty base, so
"the distro already has it" is a weaker assumption here than anywhere else.

## Resolving a package

Ask the image. It costs one pull and it is the only answer that binds:

```sh
docker run --rm cgr.dev/chainguard/wolfi-base:latest \
  sh -c 'apk add <pkg> && apk info -L <pkg>'
```

`apk info -L` lists what the package lays down — use it to confirm the *file* you actually
wanted, not just that a name resolved. To check whether the bare base already carries
something, `ls` for it before installing anything.

Browsable alternative: the [`wolfi-dev/os`](https://github.com/wolfi-dev/os) repo, one
build YAML per package. Good for reading how a package is assembled; still confirm against
the image.

## Verified data points

| Question | Answer | Read |
|--------------------------------------|--------------------------------------------------|------------|
| Base ships `/etc/mime.types`?        | No — nor `/etc/mailcap`                           | 2026-07-26 |
| What provides the MIME table?        | `mailcap` (2.1.54-r7)                             | 2026-07-26 |
| What does `mailcap` lay down?        | `/etc/mailcap`, `/etc/mime.types`, `/etc/nginx/mime.types` | 2026-07-26 |
| Is `font/woff2` in that table?       | Yes — 2258 entries, 74869 bytes                   | 2026-07-26 |
| What Caddy does `caddy` install?     | 2.11.4-r0                                         | 2026-07-27 |
| What does `caddy` lay down?          | `/usr/bin/caddy`, `/etc/caddy/Caddyfile`, `/usr/share/caddy/index.html` | 2026-07-27 |
