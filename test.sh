#!/bin/sh

set -o xtrace

go run github.com/chakrit/smoke@latest -v tests.yml "$@"
