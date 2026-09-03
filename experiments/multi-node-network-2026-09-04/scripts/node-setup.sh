#!/bin/bash
# Disposable per-consumer setup script. Reads the shared source-client
# plan, rewrites two fields that must be unique per consumer
# (local_role_state_root and the consumer's own state root for refresh),
# and writes a consumer-private copy that ardents refresh-sources then
# opens. Without this rewrite, six consumers all reference the same
# local_role_state_root path and contend on the local-roles file lock,
# which the production state store rejects with
# "acquire exclusive local role lease: resource temporarily unavailable".

set -e

NODE_ID="$1"
EVIDENCE="$2"
SHARED_PLAN="$3"
if [ -z "$NODE_ID" ] || [ -z "$EVIDENCE" ] || [ -z "$SHARED_PLAN" ]; then
    echo "usage: node-setup.sh NODE_ID EVIDENCE_DIR SHARED_PLAN" >&2
    exit 2
fi

CONSUMER_STATE="$EVIDENCE/state/$NODE_ID"
LOCAL_ROLES_DIR="$CONSUMER_STATE/local-roles"
# The private plan must live OUTSIDE the consumer state root. The
# production state store refuses to claim a non-empty unowned state root,
# so dropping the rewritten plan into the same directory would block
# every consumer with "refusing to claim a non-empty unowned state root".
# We keep the plans under a sibling directory and pass the explicit path
# to ardents refresh-sources via --source-plan.
PRIVATE_PLANS_DIR="$EVIDENCE/plans"
PRIVATE_PLAN="$PRIVATE_PLANS_DIR/$NODE_ID.json"
mkdir -p "$CONSUMER_STATE" "$PRIVATE_PLANS_DIR"

python3 -c "
import json
import sys
src = sys.argv[1]
dst = sys.argv[2]
local_roles = sys.argv[3]
with open(src, 'r') as handle:
    plan = json.load(handle)
plan['local_role_state_root'] = local_roles
with open(dst, 'w') as handle:
    json.dump(plan, handle, indent=2)
    handle.write('\n')
" "$SHARED_PLAN" "$PRIVATE_PLAN" "$LOCAL_ROLES_DIR"
