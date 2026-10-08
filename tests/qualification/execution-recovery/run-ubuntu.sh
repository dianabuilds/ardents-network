#!/bin/sh
set -eu
[ "$#" -le 1 ] || { printf '%s\n' 'invalid Endpoint-death profile arguments' >&2; exit 2; }
exec sh "$(dirname "$0")/../text-worker-lifecycle/run-ubuntu.sh" execution-recovery "${1--timeout=2m}"
