#!/bin/sh

set -o xtrace
set -o errexit

go run cuelang.org/go/cmd/cue@v0.6.0 eval -c --out=yaml tests.cue > tests.yml
go run github.com/chakrit/smoke@v0.2.2 -v tests.yml "$@"
