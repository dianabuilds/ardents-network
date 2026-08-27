#!/bin/sh
set -eu

exec go test -tags=h41aqualification ./tests/e2e/endpoint \
  -run '^TestUbuntuPortableUserUnitQualification$' -count=1 "$@"
