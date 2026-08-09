#!/usr/bin/env sh
set -eu

repository_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd -P)
git -C "$repository_root" config core.hooksPath .githooks
printf 'Git hooks enabled for %s\n' "$repository_root"
