#!/bin/sh
set -eu

root=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
launcher="$root/testbeds/infra-init/platform"
version=$("$root/bin/platform" --version)
pin=$(sed -n 's/^PLATFORM_VERSION="\(.*\)"$/\1/p' "$launcher")
if [ "$pin" != "$version" ]; then
    printf 'Launcher pin %s differs from binary version %s\n' "$pin" "$version" >&2
    exit 1
fi

# Validate before projecting; the real launcher retains the exact build metadata.
sed 's/^PLATFORM_VERSION=".*"$/PLATFORM_VERSION="<verified-build-version>"/' "$launcher"
