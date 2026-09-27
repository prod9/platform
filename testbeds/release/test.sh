#!/bin/sh
set -eu

root=$(CDPATH='' cd -- "$(dirname -- "$0")/../.." && pwd)
binary="$root/bin/platform"
mkdir -p "$root/build"
fixture=$(mktemp -d "$root/build/release.XXXXXX")
printf 'Release fixture: %s\n' "$fixture" >&2

# Local Git configuration and fixed identities keep the fixture independent of the
# operator's signing, hooks, and commit timestamps.
export GIT_CONFIG_NOSYSTEM=1 GIT_CONFIG_GLOBAL=/dev/null
export GIT_AUTHOR_NAME='Release Fixture' GIT_COMMITTER_NAME='Release Fixture'
export GIT_AUTHOR_EMAIL='fixture@example.com' GIT_COMMITTER_EMAIL='fixture@example.com'
export GIT_AUTHOR_DATE='2026-01-01T00:00:00Z' GIT_COMMITTER_DATE='2026-01-01T00:00:00Z'
export ALWAYS_YES=1

git init -q --bare "$fixture/remote.git"
git init -q -b main "$fixture/work"
cd "$fixture/work"
cat > platform.toml <<'EOF'
repository = "github.com/example/release-fixture"
strategy = "semver"
EOF
git add platform.toml
git commit -q -m 'fixture: Initial project'
git remote add gh "$fixture/remote.git"
git push -q -u gh main

reject_before_fetch() {
    label=$1
    shift
    if "$binary" -q release "$@" > "$fixture/$label.log" 2>&1; then
        printf 'Unexpected success: %s\n' "$label" >&2
        cat "$fixture/$label.log" >&2
        exit 1
    fi
    if [ -e .git/FETCH_HEAD ]; then
        printf 'Rejected request fetched remote refs: %s\n' "$label" >&2
        cat "$fixture/$label.log" >&2
        exit 1
    fi
    local_tags=$(git tag --list)
    remote_tags=$(git --git-dir="$fixture/remote.git" tag --list)
    test -z "$local_tags"
    test -z "$remote_tags"
    printf '%s: rejected before fetch\n' "$label"
}

reject_before_fetch malformed v1.2.3-alpha..1
reject_before_fetch shorthand v1.2
reject_before_fetch metadata v1.2.3+build
reject_before_fetch extra-argument v1.2.3-alpha.1 extra
reject_before_fetch name-and-patch v1.2.3-alpha.1 --patch
reject_before_fetch name-and-false-patch v1.2.3-alpha.1 --patch=false
reject_before_fetch conflicting-false-bumps --minor=false --major=false

cat > platform.toml <<'EOF'
repository = "github.com/example/release-fixture"
strategy = "datestamp"
EOF
git add platform.toml
git commit -q -m 'fixture: Date strategy'
reject_before_fetch unsupported-strategy v1.2.3-alpha.1
cat > platform.toml <<'EOF'
repository = "github.com/example/release-fixture"
strategy = "semver"
EOF
git add platform.toml
git commit -q -m 'fixture: SemVer strategy'

create_release() {
    expected=$1
    shift
    if ! "$binary" -q release "$@" > "$fixture/$expected.log" 2>&1; then
        cat "$fixture/$expected.log" >&2
        exit 1
    fi
    object_type=$(git cat-file -t "$expected")
    local_tag=$(git rev-parse "$expected")
    remote_tag=$(git --git-dir="$fixture/remote.git" rev-parse "$expected")
    test "$object_type" = tag
    test "$local_tag" = "$remote_tag"
    printf '%s: annotated tag pushed to local remote\n' "$expected"
}

create_release v1.2.3-alpha.1 v1.2.3-alpha.1
create_release v1.2.3-beta.2 v1.2.3-beta.2
create_release v1.2.3
create_release v1.2.4 v1.2.4

before=$(git rev-parse v1.2.4)
if "$binary" -q release v1.2.4 > "$fixture/collision.log" 2>&1; then
    printf 'Existing tag unexpectedly replaced\n' >&2
    exit 1
fi
local_tag=$(git rev-parse v1.2.4)
remote_tag=$(git --git-dir="$fixture/remote.git" rev-parse v1.2.4)
test "$local_tag" = "$before"
test "$remote_tag" = "$before"
printf 'collision: rejected; local and remote tags unchanged\n'
printf '\nAlpha annotation:\n'
git tag -l --format='%(contents)' v1.2.3-alpha.1
printf 'Remote tags:\n'
git --git-dir="$fixture/remote.git" for-each-ref \
    --format='%(refname:short) %(objecttype)' refs/tags
