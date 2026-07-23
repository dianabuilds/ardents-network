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

- Suspected device-key exposure: revoke the exact `DeviceID`, create a fresh
  device Credential from the offline Principal root, and verify the revoked key
  is rejected on its next call even if it presents a renewed Credential.
- Suspected session exposure: terminate the process-local session, contain the
  transport peer, and revoke the underlying device or grant when its authority
  may be compromised. Sessions are never converted into reusable credentials.
- Suspected Principal root-key exposure: take the affected Principal out of
  service and follow the explicit replacement/re-enrollment procedure. A root
  key defines the Principal and cannot be silently rotated in place.
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
