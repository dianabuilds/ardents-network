# PW3-16: CGA-06 restore, recover and migrate authority truth

Status: needs-info
State: open
Labels: needs-info
Research class: R1 migration/recovery with security review

## Parent

`../PRD.md`

## What to build

Restore the latest authority consistency group or migrate local-v2 state
without resurrecting revoked authority. Verify the signed external checkpoint,
independent monotonic history, exact head/CAS and every member before a fresh
post-migration rotation. Planned authority transition is dual-signed; lost
signer or repository truth creates a new Realm rather than repairing the old
one.

## Acceptance criteria

- [ ] Latest restore preserves sequence, audit and revocation truth.
- [ ] Stale, missing, rolled-back, forked or ambiguous repository head fails
      closed; an old signed checkpoint alone is insufficient.
- [ ] Partial restore never regenerates signer/store keys, authority sequence
      or repository history.
- [ ] Authority and repository archives have independently enforced failure
      domains and immutable retention.
- [ ] Recovery-only startup exposes stable redacted reasons and cannot mutate.
- [ ] Planned authority transition is dual-signed and exact; lost-key or
      lost-repository recovery creates a new Realm.
- [ ] The local-v2 importer rejects unknown/downgrade state, reconciles every
      member, requires fresh rotation/CAS and fences the old manager.
- [ ] Restore, migration and downgrade drills retain commit-bound evidence
      without changing `Q`.

## Blocked by

- PW3-15 / CGA-05 accepted.
- Supported deployment backup/checkpoint adapter and restore procedure.

## Comments

- Published as a blocked canonical slice. CGA-01's fail-closed no-restore rule
  remains authoritative until this issue is accepted.
