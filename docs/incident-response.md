# Operator Incident Response

## First Response

1. Preserve the affected node's Diagnostics snapshot, bounded logs, effective
   redacted config, image digest, and deployment manifest.
2. Contain exposure at the deployment boundary. Do not delete databases, keys,
   blobs, Waku Store, or capability ledgers as a diagnostic shortcut.
3. Determine whether the incident affects operator authority, identity,
   transport, private capability, retained payloads, workload isolation, or
   availability commitments.
4. Stop publication and workload actions whose authority or runtime truth is
   uncertain; keep unaffected encrypted retention only when policy permits.
5. Recover from a verified consistency-group backup and re-prove identity,
   privacy, network, data, and Diagnostics truth.

## Credential And Key Events

- Suspected API token exposure: replace the deployment secret, restart the local
  API, verify the old token is rejected, and review bounded audit events.
- Capability compromise: revoke the grant, issue a fresh channel generation to
  remaining members, rotate selector/channel secret, and reject the old sender.
- WSS key compromise: replace certificate/key as one deployment operation,
  update trust material where required, and restart the listener.
- Ardents or Waku identity key loss: restore the complete matching consistency
  group. Deleting the remaining half or generating a replacement in place is
  not recovery.
- Payload-key loss: retained ciphertext remains unavailable; do not describe
  storage presence as readability.

## Exit Criteria

Close the incident only after the cause and exposure window are recorded,
revoked authority no longer works, restored nodes preserve expected identities,
private operations do not downgrade, committed replica counts are truthful, and
monitoring/alerts have returned to their expected state.

