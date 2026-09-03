#!/bin/bash
# Disposable live clock-observation owner for the multi-consumer pilot. The
# production State contract reads a regular file's mtime as its independent
# observation and rejects observations more than two seconds from its local
# clock. Keep the shared marker fresh at the same 500 ms interval used by the
# maintained Contributor clock owner; never rewrite a plan that consumers may
# already be reading.

set -eu

OBSERVATION="$1"
if [ -z "$OBSERVATION" ]; then
    echo "usage: clock-tick.sh OBSERVATION_PATH" >&2
    exit 2
fi
if [ -e "$OBSERVATION" ] && { [ ! -f "$OBSERVATION" ] || [ -L "$OBSERVATION" ]; }; then
    echo "clock-tick: observation is not one regular file: $OBSERVATION" >&2
    exit 2
fi

mkdir -p "$(dirname "$OBSERVATION")"
touch "$OBSERVATION"
echo "clock-tick: live observation ready: $OBSERVATION"

stopping=0
trap 'stopping=1' INT TERM
while [ "$stopping" -eq 0 ]; do
    touch "$OBSERVATION"
    sleep 0.5 &
    wait "$!" || true
done
