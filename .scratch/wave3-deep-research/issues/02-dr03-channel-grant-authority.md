# DR-03: Define Production Channel Grant authority

Status: ready-for-agent
State: closed
Labels: ready-for-agent
Research class: R2

## Parent

`../PRD.md`

## What to build

Produce a complete authority-lifecycle decision for production private
channels. Select a trust root and realm model, then define issuance, protected
delivery, acknowledgement, recovery, membership changes, revocation,
generation rotation, channel-class separation, audit attribution,
backup/restore consistency, federation, and migration.

The selected module must keep capability material out of ordinary operator
output and must provide a stable authority contract that Application Messaging
and private multi-host operations can consume without inventing their own realm
semantics.

## Acceptance criteria

- [x] Current authority, storage, recovery, and operator journeys are evidenced from the frozen baseline.
- [x] At least two materially different trust/authority designs are compared.
- [x] The selected design defines actors, state owners, exact lifecycle transitions, revocation, rotation, restart, and recovery.
- [x] Discovery, data, and application channel authority are explicitly separated or intentionally unified with justification.
- [x] Backup/restore, federation, migration, privacy, abuse, and audit consequences are explicit.
- [x] A proposed ADR decision and vertical implementation slices are ready for review.
- [x] No Messaging API or release qualification is implemented or claimed.

## Blocked by

- W3-00

## Comments

- Accepted on 2026-07-25 against frozen product baseline
  `8b9f8ad87fb78fccd7a73d445f2d72dbf2e51b4c`.
- Packet:
  `docs/engineering/research/channel-grant-authority.md`.
- Selected one deployment-owned Realm Authority Principal on a designated
  authority Node, distinct from the Node Principal and Waku Peer ID.
  Operator-only HPKE generation bundles and approved-host installed/active
  attestations drive fresh-generation cutover; a receipt MAC proves bundle
  possession, not honest host execution, so suspect/noncompliant members are
  fenced.
- Selected an independent monotonic compare-and-append checkpoint repository
  outside the authority-backup fault domain as the anti-rollback trust root.
  Ambiguous, missing, forked, or stale head prevents security mutations and
  same-realm restore.
- Rejected shared authority files, Node-local issuers, federation, and replacing
  the current Channel Grant protocol with MLS for v1.
- Proposed ADR:
  `docs/adr/0011-single-authority-channel-grant-lifecycle.md`; it remains
  Proposed pending maintainer approval.
- Reviewed CGA-01 through CGA-07 as dependency-ordered vertical slices. DR-01
  consumes the authority lifecycle; DR-04 must provide fencing, authority/
  checkpoint placement, bounded clocks, and rollout order.
- Checks passed: complete packet review; source-path and Markdown checks;
  `go test ./internal/identity/capability ./internal/provision
  ./internal/messaging`; and `git diff --check`. Multi-host deployment,
  adversarial security, backup/restore, migration, and release qualification
  were not run because the selected design is not implemented.
- Canonical I/R/O/Q remains `partial/no/no/no`; no release qualification is
  claimed.
