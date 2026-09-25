#!/bin/sh
# Build one immutable input set for the installed Ubuntu command journey.
set -eu
export GOENV=off GOTOOLCHAIN=local GOFLAGS=-mod=readonly

fail() { printf '%s\n' "$*" >&2; exit 1; }

: "${ARDENTS_TEXT_COMMAND_CANDIDATE_PARENT:?set an existing absolute output parent outside the repository}"
case "$ARDENTS_TEXT_COMMAND_CANDIDATE_PARENT" in
  /*) ;;
  *) fail 'candidate parent must be absolute' ;;
esac
[ -d "$ARDENTS_TEXT_COMMAND_CANDIDATE_PARENT" ] && [ ! -L "$ARDENTS_TEXT_COMMAND_CANDIDATE_PARENT" ] ||
  fail 'candidate parent must be an existing direct directory'
for program in go mktemp mkdir cp chmod sha256sum; do
  command -v "$program" >/dev/null || fail "candidate build requires $program"
done
[ "$(go version)" = 'go version go1.26.8 linux/amd64' ] ||
  fail 'candidate build requires Go 1.26.8 on Linux x86-64'

repository=$(CDPATH= cd -- "$(dirname -- "$0")/../../.." && pwd -P)
parent=$(CDPATH= cd -- "$ARDENTS_TEXT_COMMAND_CANDIDATE_PARENT" && pwd -P)
case "$parent/" in
  "$repository/"*) fail 'candidate output must be outside the repository' ;;
esac
cd "$repository"

umask 077
candidate=$(mktemp -d "$parent/closed-text-candidate.XXXXXXXX") ||
  fail 'candidate output could not be created'
printf 'candidate-stage=%s\n' "$candidate" >&2
mkdir "$candidate/commands" "$candidate/worker"

go test -c -tags text_worker_installed -trimpath -buildvcs=false \
  -o "$candidate/closed-text-commands.test" ./tests/e2e/node
for name in ardents ardents-custody ardents-node ardents-control ardents-text; do
  go build -trimpath -buildvcs=false -o "$candidate/commands/$name" "./cmd/$name"
done
cp "$candidate/commands/ardents-text" "$candidate/worker/ardents-text"
chmod 0555 "$candidate/closed-text-commands.test" "$candidate/commands/"* "$candidate/worker/ardents-text"

(
  cd "$candidate"
  sha256sum closed-text-commands.test \
    commands/ardents commands/ardents-custody commands/ardents-node \
    commands/ardents-control commands/ardents-text worker/ardents-text > SHA256SUMS
  sha256sum --check --status SHA256SUMS
)
printf '%s\n' 'go version go1.26.8 linux/amd64' > "$candidate/GO-VERSION"
printf '%s\n' 'complete' > "$candidate/READY"
printf 'candidate-ready=%s\n' "$candidate"
