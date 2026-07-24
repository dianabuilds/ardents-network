# Ardents Operator Runbook

## Routine Checks

- `ardentsctl node status`: lifecycle, Identity, subsystem, and Diagnostics truth;
- `ardentsctl network status`: joined/reachability/privacy/profile truth;
- `/readyz` and bounded Prometheus alerts: fleet-level readiness;
- `ardentsctl diagnostics pending` and `explain`: incomplete or degraded operations.

Never infer readiness solely from a running container or open TCP port.

## Backup And Restore

Use `./ardents.ps1 backup` and `./ardents.ps1 restore` for the supported Compose
target. Backups are stopped-node consistency groups. Restore verifies archive
checksum plus Ardents principal, device, and Waku peer identity. Deployment
secrets are backed up separately; detailed groups and failure rules are in
`docs/security/persistent-state-security.md`.

## Rotation And Revocation

- Operator device Credential: create a fresh finite device Credential from the
  offline Principal root, verify it, then revoke the exact old Device ID;
- Operator Access Grant: issue and verify the replacement grant before revoking
  the exact old Grant ID;
- observability scrape token: replace the monitoring secret and restart only
  that read-only boundary;
- WSS certificate/key: replace matching files together, then controlled restart;
- Channel Grant: revoke the old grant and perform a fresh channel
  generation;
- Ardents/Waku identity: no implicit `v1` rotation; use an explicit new-node
  migration and withdraw old publication.

## Upgrade, Rollback, And Incidents

Follow `docs/operations/upgrade-migration.md` for image/config changes and
`docs/operations/incident-response.md` for containment and recovery. Preserve diagnostics
and state before intervention. Cleanup is an explicit deployment action and
must never be used as an automatic repair policy.

Compose rollout state is recorded in
`<StateDir>/rollout-transaction.json`. Do not delete or edit this file during an
incident. Re-run `upgrade` or `rollback` with the same project and state
directory to resume compensation to the recorded fallback digest; after
compensation completes, run the intended rollout command again.
