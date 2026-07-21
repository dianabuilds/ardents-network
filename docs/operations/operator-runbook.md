# Ardents Operator Runbook

## Routine Checks

- `ard node status`: lifecycle, Identity, subsystem, and Diagnostics truth;
- `ard network status`: joined/reachability/privacy/profile truth;
- `/readyz` and bounded Prometheus alerts: fleet-level readiness;
- `ard diagnostics pending` and `explain`: incomplete or degraded operations.

Never infer readiness solely from a running container or open TCP port.

## Backup And Restore

Use `./ardents.ps1 backup` and `./ardents.ps1 restore` for the supported Compose
target. Backups are stopped-node consistency groups. Restore verifies archive
checksum plus Ardents principal, device, and Waku peer identity. Deployment
secrets are backed up separately; detailed groups and failure rules are in
`docs/security/persistent-state-security.md`.

## Rotation

- API and observability token: replace secret and restart the corresponding
  local boundary;
- WSS certificate/key: replace matching files together, then controlled restart;
- capability: revoke old grant and perform a fresh private channel generation;
- Ardents/Waku identity: no implicit `v1` rotation; use an explicit new-node
  migration and withdraw old publication.

## Upgrade, Rollback, And Incidents

Follow `docs/operations/upgrade-migration.md` for image/config changes and
`docs/operations/incident-response.md` for containment and recovery. Preserve diagnostics
and state before intervention. Cleanup is an explicit deployment action and
must never be used as an automatic repair policy.
