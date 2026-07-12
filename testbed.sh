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

# Platform requires the target to be a git repo root (it never runs `git init` itself).
# The testbeds are subdirs of this repo, so mark each as its own root — idempotent, and
# git does not treat this nested .git as pollution of the parent's status.
git init -q .

../../bin/platform "$@"
