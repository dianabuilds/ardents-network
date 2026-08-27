#!/bin/sh
set -eu

exec go test -tags=h41bqualification ./tests/e2e/endpoint \
  -run '^TestUbuntuPortableReplacement(Qualification|RollbackQualification)$' -count=1 "$@"
