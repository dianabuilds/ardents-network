# PW3-16: CGA-06 restore, recover and migrate authority truth

Status: ready-for-human
State: open
Labels: ready-for-human
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

- [x] Latest restore preserves sequence, audit and revocation truth.
- [x] Stale, missing, rolled-back, forked or ambiguous repository head fails
      closed; an old signed checkpoint alone is insufficient.
- [x] Partial restore never regenerates signer/store keys, authority sequence
      or repository history.
- [x] Authority and repository archives have independently enforced failure
      domains and immutable retention.
- [x] Recovery-only startup exposes stable redacted reasons and cannot mutate.
- [x] Planned authority transition is dual-signed and exact; lost-key or
      lost-repository recovery creates a new Realm.
- [x] The local-v2 importer rejects unknown/downgrade state, reconciles every
      member, requires fresh rotation/CAS and fences the old manager.
- [x] Restore, migration and downgrade drills retain commit-bound evidence
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
- 2026-07-28 CGA-06 implementation handoff:
  - exact starting commit:
    `ff8643c0811cd4746401b69d1940b8292269cbcb`;
  - exact implementation tip:
    `1136def860f30bc452e1b5352c537cbd44a163f6`;
  - implementation commits:
    `9993528`, `8633e51`, `381b091`, `91d3c09`, `ccdaebf`, `5d612b1`,
    `423bfae`, `34a557a`, `1136def`;
  - recovery-only startup verifies the exact Realm, authority sequence,
    checkpoint, repository head and signer identity against the independently
    monotonic repository. Missing, stale, rolled-back, forked, ambiguous or
    mismatched truth fails closed without mutation or audit draining;
  - planned authority transition is one persisted, restart-idempotent
    operation. Its successor checkpoint carries a dual-signed transition
    proof; repository and ledger CAS advance the epoch, signer and sequence.
    Temporary repository loss remains recoverable with the preprovisioned
    successor, while corrupt or conflicting truth enters recovery;
  - every existing Channel must complete a fresh rotation before transition
    completion. Member adoption and finalization use distinct protected
    actions; successor trust is persisted in the encrypted capability store,
    and the successor-signed completion artifact is bound to the terminal
    checkpoint and exact sorted required/rotated Channel sets. Completion
    durably retires predecessor issuance trust;
  - local-v2 migration strictly decodes supported state, rejects unknown or
    downgrade input, reconciles exact members and grants, performs fresh
    rotations and honors the legacy manager's shared state-directory lock.
    The downgrade drill restores a complete stopped local-v2 backup rather
    than reconstructing partial state;
  - the protected Operator API, capability catalogue, ADR contracts and
    architecture ownership graph consistently expose
    `realm.authority.rotate`, `realm.authority.transition.adopt`,
    `realm.authority.transition.finalize`,
    `realm.authority.migrate.local_v2` and
    `realm.channel.recovery.execute`;
  - commit-bound validation passed on the exact implementation tip:
    - `go test ./...`;
    - `go vet ./...`;
    - `go test -race ./internal/authority ./internal/identity/capability
      ./internal/channeldelivery ./internal/messaging ./internal/provision
      ./internal/localapi/authority ./internal/localapi/channeldelivery`;
    - `go test -tags=integration ./tests/integration/localapi
      ./tests/integration/messaging`;
    - `scripts/generate-api.ps1 -Check`;
    - `go run ./tests/tooling/capabilitycatalog -root . -check`;
    - `govulncheck ./...` reported no called vulnerabilities;
    - `git diff --check`;
  - `tests/ci/cga06-recovery-migration-gate.ps1` required a clean checkout
    and exact full commit, then retained JSONL and SHA-256 evidence for
    restore, migration and downgrade drills. The evidence hashes were:
    - restore:
      `aa6b5b97bb8a010b7a61060c69b0789af389c77305e1621a782a26a833a0abc0`;
    - migration:
      `f054f9d5bce195053b37258ac32aa44399136bfdcba67ec030cbc48238d17a51`;
    - downgrade:
      `aa0c24cd848f8aa861954c83967211032dbbf1e28a643a8f85aaf9ed63cdaa60`;
  - independent repeat Standards and Spec reviews on the exact implementation
    tip both returned `PASS` with no remaining actionable findings;
  - the capability catalogue remains `24 capabilities, 8 domains, 0
    qualified`; canonical qualification is unchanged and `Q=no`;
  - CGA-07 remains gated on explicit maintainer acceptance of the exact
    CGA-06 implementation tip. No production deployment was changed and
    nothing was pushed.
