#!/usr/bin/env bash
set -eu

script_parent=${BASH_SOURCE[0]%/*}
if [[ "$script_parent" == "${BASH_SOURCE[0]}" ]]; then
  script_parent=.
fi
repository_root=$(CDPATH= cd -- "$script_parent/.." && pwd -P)

export ARDENTS_BOOTSTRAP_BASH_VERSION=${BASH_VERSION:-unavailable}
export GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOWORK=off
cd "$repository_root"
exec go run ./cmd/carrier-lab bootstrap --repository-root "$repository_root" "$@"
