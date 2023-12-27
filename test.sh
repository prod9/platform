#!/bin/sh

set -o xtrace

cue eval -c --out=yaml tests.cue > tests.yml
go run github.com/chakrit/smoke@v0.2.2 -v tests.yml "$@"
