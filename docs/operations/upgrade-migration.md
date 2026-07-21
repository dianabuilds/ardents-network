# Upgrade And Migration Guide

## Release Identity

Use only a release artifact whose `ard version` or `ardd --version` matches the
version, commit, build date, OS, and architecture in its provenance statement.
Verify `SHA256SUMS` before installation and retain the previous immutable image
digest until rollback acceptance.

## Rolling Upgrade

1. Read release notes, persisted-format changes, dependency exceptions, and
   platform support.
2. Stop each node in turn and create a verified consistency-group backup.
3. Record the current image digest and cluster manifest.
4. Recreate one node with the new immutable image.
5. Require local API health, canonical network readiness, expected privacy
   state, Diagnostics, and retained identity before continuing.
6. Repeat for remaining nodes; do not upgrade all bootstrap nodes together.
7. Retain backups and the previous image until the observation window closes.

`./ardents.ps1 upgrade` automates the single-host Compose sequence.

## Rollback

Recreate one node at a time with the previous immutable image and re-prove
readiness. Restore data only when release notes say the newer version migrated a
persisted format incompatibly. Never attach an old binary to a partially
migrated database or silently discard newer state.

If automatic rollback fails, stop the affected node, preserve logs and the
complete state directory, and follow the incident runbook. A failed node must
not be reported as healthy merely because another peer remains available.

## Configuration And Protocol Migration

Unknown operator-config versions and fields are rejected. Generate and validate
the new version before restart; preserve the previous document for rollback.
Private protocol versions never downgrade to readable legacy topics. Capability
and selector rotation uses an explicit encrypted cutover as defined by
`docs/protocols/network-privacy-protocol.md`.
