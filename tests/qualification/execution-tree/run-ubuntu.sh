#!/bin/sh
set -eu
exec sh "$(dirname "$0")/../worker-lifecycle/run-ubuntu.sh" execution-tree
