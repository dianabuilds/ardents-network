#!/bin/sh
set -eu

exec go test -tags=endpoint_portable_qualification ./tests/e2e/endpoint \
  -run '^TestUbuntuPortableUserUnitQualification$' -count=1 "$@"
