# DR-03: Define Production Channel Grant authority

Status: ready-for-agent
State: open
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

- [ ] Current authority, storage, recovery, and operator journeys are evidenced from the frozen baseline.
- [ ] At least two materially different trust/authority designs are compared.
- [ ] The selected design defines actors, state owners, exact lifecycle transitions, revocation, rotation, restart, and recovery.
- [ ] Discovery, data, and application channel authority are explicitly separated or intentionally unified with justification.
- [ ] Backup/restore, federation, migration, privacy, abuse, and audit consequences are explicit.
- [ ] A proposed ADR decision and vertical implementation slices are ready for review.
- [ ] No Messaging API or release qualification is implemented or claimed.

## Blocked by

- W3-00

## Comments

None.
