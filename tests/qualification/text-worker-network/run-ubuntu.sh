#!/bin/sh
set -eu
exec sh "$(dirname "$0")/../text-worker-lifecycle/run-ubuntu.sh" network
