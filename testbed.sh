#!/bin/sh

set -o errexit

go build -v -o ./bin/platform .

DIR="$1"
if [ -z "$DIR" ]; then
				echo "Specify testbed to work as first arg"
				exit 1
fi

shift
cd "./testbeds/$DIR"
../../bin/platform "$@"
