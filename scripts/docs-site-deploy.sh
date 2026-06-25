#!/bin/sh
# Publish www/ to the gh-pages branch. Run from the repo root.
#
# The site is derived: regenerate www/ from docs/ and commit before deploying.
# One-time setup: enable GitHub Pages -> gh-pages branch in repo settings.
# No GitHub Actions involved — the branch hosts directly.
set -eu

remote="${1:-gh}"   # service-named remote (gh = GitHub), not origin

# Push the current www/ tree as the root of gh-pages.
git push "$remote" "$(git subtree split --prefix www HEAD)":refs/heads/gh-pages --force
