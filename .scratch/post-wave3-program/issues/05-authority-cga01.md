# PW3-05: CGA-01 create and inspect a production realm authority

Status: ready-for-human
State: open
Labels: ready-for-human
Research class: R0 implementation with security review

## Parent

`../PRD.md`

## User story

As a Realm Operator, I can create or reopen one production realm on a
designated authority Node and inspect bounded, redacted authority and
checkpoint status without exposing Channel Grant material.

## Complete vertical behavior

Implement the smallest production authority tracer bullet from authenticated
Operator admission through durable realm genesis and restart:

```text
Operator request with stable idempotency identity
  -> exact action/resource and Product Policy admission
  -> RealmAuthority transaction
  -> external signer binding
  -> new-realm checkpoint create-if-absent
  -> audit outbox and stable result
  -> redacted Inspect projection
  -> restart reads and validates the same independent head
```

This slice freezes the versioned authority, checkpoint, inspect, audit, and
error contracts needed by later CGA slices. It does not deliver a generation,
change membership, activate a channel, or qualify the capability.

## In scope

- versioned RealmID, authority epoch, authority sequence, schema, and
  control-artifact contracts;
- exact Operator actions/resources required for realm genesis and redacted
  inspection;
- Actor/Effective attribution, Access Grant admission, Product Policy,
  idempotency, and audit outbox transaction;
- one transactional authority ledger with finite member/channel/operation
  containers;
- narrow external Realm Authority signer seam with Principal/public-key
  continuity checks;
- monotonic checkpoint repository seam supporting unique-head read,
  new-random-RealmID create-if-absent, and exact compare-and-append;
- signed genesis checkpoint and immutable audit-chain head;
- redacted `Inspect` status with stable bounded phases, counts, deadlines, and
  reasons;
- startup/restart validation against the independent repository head;
- readiness, diagnostics, metrics, corruption, mismatch, and exhaustion
  behavior for this slice.

## Out of scope

- HPKE delivery and receipts (CGA-02);
- rotation/activation and generation overlap (CGA-03);
- membership change, revocation, or fencing (CGA-04);
- renewal and Application-channel lifecycle (CGA-05);
- backup restore, authority rotation, and v2 migration (CGA-06);
- CGA-07 or any matching-commit qualification;
- a production multi-host topology or Application Messaging;
- federation, MLS, threshold authority, public authority, or multiple realms
  per authority instance.

## Dependencies and admission gate

- PW3-02 is closed and ADR-0011 has explicit maintainer acceptance.
- This issue is `ready-for-agent`; implementation still requires deliberate
  assignment and must preserve the accepted ADR boundary.
- Accepted ADR-0001, ADR-0002, ADR-0003, and existing Identity/Access,
  idempotency, audit-outbox, and protected Operator Interface contracts are
  fixed dependencies.
- The implementation must consume the current capability store and security
  vocabulary without treating `internal/provision` or shared authority files
  as the production state owner.
- A concrete external signer adapter and checkpoint repository adapter may be
  selected during implementation, but they must satisfy ADR-0011 semantics.

## Authority and state ownership

- The Realm Authority Principal is distinct from the hosting authority Node
  Principal, Waku Peer ID, TLS identity, SSH identity, and Operator Principal.
- The authority ledger owns realm identity, authority epoch/sequence,
  bounded member/channel records, operation identities, audit outbox, and
  signed checkpoint references.
- The signing private key remains outside the database behind a narrow signer
  seam. Authority-store encryption keys are separately provisioned.
- The checkpoint repository owns independent monotonic freshness evidence. It
  is not a second authority and cannot derive or mutate membership.
- The Operator request supplies intent and authority through exact grants; it
  does not receive raw authority, channel, grant, selector, or receipt secrets.
- `authority.json` and shared directory possession are migration inputs only,
  never production truth.

## Bounds and public contract

- Exactly one realm per authority instance.
- Capacity is fixed at no more than 256 realm members, 1,024 active channels,
  256 members per channel, one pending and one previous generation, and four
  deliveries per member/channel, even where later containers are initially
  empty.
- Requests, responses, operation IDs, pages, audit records, denial records,
  reasons, and metric labels are versioned and finite.
- Unknown fields, unsupported versions, oversized input, duplicate RealmID,
  non-empty genesis head, or capacity exhaustion fail closed with stable
  errors.
- `Inspect` exposes realm/channel class, generation/sequence, phase, counts,
  deadlines, readiness, and stable reasons only. Membership lists require a
  separate exact audit/read action and are not part of ordinary status.

## Restart, failure, and recovery behavior

- Genesis request, idempotency result, ledger state, audit outbox record,
  sequence, and checkpoint digest commit as one authoritative transaction.
- Repository create-if-absent is allowed only for a new random RealmID during
  stopped genesis. A pre-existing or non-empty head fails without overwrite.
- Startup reads the external head before becoming mutation-ready and verifies
  RealmID, authority Principal/key binding, epoch, sequence, digest, schema,
  and audit-chain continuity.
- Store/signing/repository unavailability yields stable degraded or unavailable
  readiness and never regenerates keys, sequence, or a repository head.
- Missing, lower, forked, non-monotonic, or unexpected head enters
  recovery-required and blocks mutations.
- A crash at every transaction/checkpoint boundary either returns the original
  stable result or resumes the one durable operation identity; it never emits
  a second genesis or audit identity.
- This slice does not implement same-realm archive restore. It must preserve
  enough exact state for CGA-06 to verify, and must fail closed rather than
  inventing a repair path.

## Acceptance criteria

- [x] Realm creation and reopen are reachable only through the protected
      Operator Interface with exact action/resource admission.
- [x] No equivalent Application Interface procedure exists.
- [x] Duplicate same request returns the original result; changed request
      content under the same idempotency identity conflicts.
- [x] One durable realm/epoch/sequence and audit-chain genesis survives
      restart.
- [x] The signer seam proves the active Realm Authority Principal binding
      without storing the signing key in the authority database.
- [x] Repository create-if-absent and exact compare-and-append contracts reject
      overwrite, skip, fork, stale expected sequence, and blind put.
- [x] Corrupt state, key/Principal mismatch, head mismatch, unsupported schema,
      or unavailable independent head fail closed.
- [x] `Inspect`, logs, metrics, audit, CLI JSON, backups, and test artifacts
      contain no secret, selector, plaintext grant, receipt key, private
      endpoint, or unbounded Principal label.
- [x] All declared bounds and stable error classes have positive and negative
      tests.
- [x] Crash injection at each persistence/repository boundary deterministically
      resumes or fails closed.
- [x] Documentation states that this is an implementation slice, not
      qualification.

## Required tests and evidence

- Unit tests for RealmID/epoch/sequence, state transitions, authorization,
  idempotency, bounds, redaction, and corrupt-state handling with injected
  clock, random source, signer, store, and repository.
- Contract tests for version/unknown-field/size rejection, exact
  action/resource mapping, Application Interface absence, repository
  create-if-absent/read/CAS behavior, and stable public errors.
- Integration tests with real durable authority storage, signer adapter test
  double, repository contract fixture, audit outbox, and complete restart.
- Security negatives for wrong Principal/key, stale/forked head, replayed
  request, unauthorized/delegation-policy mismatch, malformed signatures,
  cardinality exhaustion, and secret/log scans.
- Documentation contract, architecture acceptance, capability catalogue check,
  relevant package tests, race tests where supported, and `git diff --check`.
- Evidence remains development evidence tied to the slice commit; it is not
  CGA-07/DR-06 qualification.

## Capability impact and no-Q rule

- Capability: `realm.channel-grant-authority`.
- This slice may improve implementation/reachability only when its supported
  caller journey actually exists and canonical governance is updated from
  evidence.
- It does not establish membership changes, generation delivery, fencing,
  recovery, production operability, or qualification.
- `Q` must remain `no`; only the later complete matching-commit qualification
  program may promote it.

## Expected files and modules

- New deep authority module and transactional persistence under the Identity
  and Security ownership boundary.
- Protected Operator protocol/handler/CLI types for create/open and inspect.
- Exact Identity/Access action and resource registrations.
- External signer and monotonic checkpoint-repository ports plus contract
  fixtures.
- Authority audit/outbox, readiness, diagnostics, and bounded metrics.
- Versioned protocol vectors, tests, and authority operations/security
  documentation.
- Exact paths are selected during implementation review; this issue does not
  authorize changes to Application Messaging or topology modules.

## Exit condition

The issue closes when one logical implementation commit provides the complete
Operator-to-ledger-to-checkpoint-to-restart tracer bullet, all required checks
pass, retained development evidence is attached, and independent review finds
no authority, state-ownership, bounds, redaction, or restart blocker. ADR-0011
must already be accepted. Capability qualification and downstream CGA slices
remain open.

## Comments

- 2026-07-27 governance transition: ADR-0011 was explicitly accepted from
  source `34bccdeef830fde0cd17d99dec14c9bc4cd8929c` after commit-bound review
  evidence `cb9cdb0903594885cb44090876be9659f7781b4d`. PW3-02 is closed, so the
  ADR admission gate is satisfied and CGA-01 moves from `needs-info` to
  `ready-for-agent`. No implementation, capability promotion, qualification or
  push is implied by this triage transition. Acceptance governance commit:
  `2030d35f1df0a11f8d701ea12e19537a6b4d1c69`.
- 2026-07-28 CGA-01 implementation handoff:
  - exact starting commit:
    `21715a2280ce998f524fc7cdad21af31421d4ee7`;
  - exact logical implementation commit:
    `3551a1e5f486d34711416cacfe21fc420d393c46`;
  - implemented the protected Operator-only create/reopen and redacted inspect
    tracer bullet, protocol-owned exact access metadata, Product Policy on
    initial and idempotent admission, one encrypted transactional authority
    ledger, separately provisioned store key and external signer, immutable
    retained audit chain plus delivery outbox, stable idempotency result,
    signed genesis checkpoint, and restart reconciliation;
  - the production checkpoint adapter now requires a pre-provisioned,
    independently administered deletion-protected/WORM repository and
    provisioning assertion, never creates a missing root, has no local
    fallback, and rejects symlink roots, realm directories, overwrite, skip,
    fork, stale expected sequence, and blind put;
  - Inspect/CLI expose only the bounded redacted v1 projection, including
    generation `0`, authority sequence, genesis-operation deadline, phase,
    readiness/reason, and counts. No Application Authority service or secret
    route exists;
  - exact validation on the implementation commit:
    - `go test ./... -count=1`;
    - `go test -race ./internal/authority ./internal/localapi/authority
      ./internal/daemon ./internal/observability ./internal/cli/authority
      ./internal/cli/catalog -count=1`;
    - `go test -tags=integration ./tests/integration/localapi -run
      'TestRealmAuthority' -count=1`;
    - `go test ./tests/tooling/doccontract ./tests/tooling/archaccept
      -count=1`;
    - `go run ./tests/tooling/capabilitycatalog -check`;
    - `scripts/generate-api.ps1 -Check`;
    - `git diff --check
      21715a2280ce998f524fc7cdad21af31421d4ee7..3551a1e5f486d34711416cacfe21fc420d393c46`;
  - retained development evidence is the real-store/crash/restart integration
    suite in `tests/integration/localapi/authority_test.go`, unit/contract and
    security-negative suites under `internal/authority` and
    `internal/localapi/authority`, the canonical checkpoint vector at
    `internal/authority/testdata/checkpoint-genesis-v1.json`, the operations
    contract at `docs/security/realm-authority-cga01.md`, and the separate
    governance proposal at
    `docs/engineering/realm-authority-cga01-capability-proposal.md`;
  - independent fixed-range Spec and Standards reviews reported no actionable
    P0-P3 findings after remediation;
  - known limits are deliberate: CGA-01 has no generation delivery/receipts,
    rotation/activation, membership mutation, revocation/fencing, renewal,
    same-realm restore/migration, multi-host placement, or qualification.
    Storage-enforced WORM retention and independent credentials remain a
    deployment prerequisite, not something a local process can manufacture;
  - canonical `docs/engineering/capabilities.json` and the evidence register
    are unchanged. Catalogue validation remains `24 capabilities, 8 domains,
    0 qualified`; there is no canonical I/R/O/Q change and `Q` remains `no`;
  - CGA-02 through CGA-07 were not started. No production deployment was
    changed and nothing was pushed.
