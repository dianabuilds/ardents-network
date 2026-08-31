#!/bin/sh
set -eu

exec go test -tags=endpoint_replacement_qualification ./tests/e2e/endpoint \
  -run '^TestUbuntuPortableReplacement(Qualification|RollbackQualification)$' -count=1 "$@"
