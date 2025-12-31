#!/bin/sh

CUE_VERSION=v0.15.3
SMOKE_VERSION=v0.2.4

set -o xtrace
set -o errexit

go run cuelang.org/go/cmd/cue@${CUE_VERSION} eval -c --out=yaml tests.cue > tests.yml
go run github.com/chakrit/smoke@${SMOKE_VERSION} -v tests.yml "$@"
