#!/bin/sh

SMOKE_VERSION=v0.5.0

set -o xtrace
set -o errexit

go run github.com/chakrit/smoke@${SMOKE_VERSION} -v tests.cue "$@"
