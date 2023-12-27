#!/bin/sh

set -o xtrace

go run github.com/chakrit/smoke@v0.2.2 -v tests.yml "$@"
