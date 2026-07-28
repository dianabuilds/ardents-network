# PW3-16: CGA-06 restore, recover and migrate authority truth

Status: ready-for-agent
State: open
Labels: ready-for-agent
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
- 2026-07-28 predecessor and deployment seams satisfied:
  - the maintainer explicitly accepted CGA-05 implementation commit
    `c8f87f69ff27a14902a628822b49bab60fa0dd38`;
  - the stopped native backup/restore path already archives the separately
    protected authority directory into an empty stopped target and verifies an
    immutable archive/manifest digest;
  - the configured production checkpoint adapter requires a preprovisioned
    independent WORM assertion, immutable predecessor files, unique head and
    exact compare-and-append, while configuration rejects repository placement
    inside authority/store/signer fault domains;
  - CGA-06 owns the missing same-realm restore verification, recovery-only
    fence, transition and local-v2 import protocol. Admitting this slice does
    not qualify those deployment procedures, change `Q`, deploy or push.
