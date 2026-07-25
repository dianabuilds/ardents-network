# PW3-02: Review ADR-0011 single-authority Channel Grant lifecycle

Status: ready-for-human
State: open
Labels: ready-for-human
Research class: decision review

## Parent

`../PRD.md`

## User story

As a maintainer, I can accept, return, or reject the production Channel Grant
authority decision with the trust root, authority procedures, convergence
claim, anti-rollback root, recovery contract, and finite v1 boundary visible in
one review packet.

## Complete review behavior

Review
`docs/adr/0011-single-authority-channel-grant-lifecycle.md` against the accepted
DR-03 packet, Wave 3 synthesis, existing identity/access ADRs, and current
capability truth. Record exactly one outcome:

- `review-ready`, meaning the packet is ready for a separate explicit
  maintainer acceptance decision;
- `returned with blockers`, naming each required correction;
- `rejected with rationale`.

The review does not implement the authority, change capability status, or
interpret research acceptance as ADR acceptance. Only an explicit maintainer
decision may change ADR-0011 from `Proposed`.

## In scope

- one deployment-owned Realm Authority Principal as sole issuer for one v1
  realm;
- separation from the authority Node Principal, every member Principal, Waku
  Peer ID, TLS identity, SSH identity, Credential, Session, Access Grant, and
  Delegation;
- exact protected Operator actions/resources and the non-delegable recovery
  and authority-rotation policy;
- recipient-attested RFC 9180 HPKE `GenerationBundle` delivery;
- installed/active receipt semantics and their approved-host trust boundary;
- fresh generation on membership add/remove, renewal, and routine rotation;
- removal revocations, survivor activation, explicit deployment fencing, and
  truthful partition behavior;
- separate discovery, data, Application, and capability-control channels;
- transactional authority truth, audit outbox, signed checkpoint, and external
  signer seam;
- independent monotonic compare-and-append checkpoint repository;
- backup, restore, rollback/fork handling, v2 migration, authority rotation,
  downgrade, federation/MLS disposition, and finite v1 bounds.

## Out of scope

- accepting ADR-0013 or ADR-0015;
- implementing CGA-01 through CGA-07;
- selecting a checkpoint-repository vendor or signer product;
- Application Messaging semantics or multi-host topology implementation;
- federation, MLS, threshold authority, public authority, Kubernetes,
  suppressed transports, remote Application transport, or non-Go SDKs;
- any `I`, `R`, `O`, or `Q` promotion.

## Dependencies

- Root `CONTEXT.md` and accepted ADR-0001 through ADR-0010 are fixed inputs.
- Research source:
  `docs/engineering/research/channel-grant-authority.md`.
- Cross-packet source:
  `docs/engineering/research/wave3-synthesis.md`.
- Decision register entry W3-D001 remains proposed until this review completes.
- No Authority implementation may start before explicit ADR-0011 acceptance.
- ADR-0013 and ADR-0015 acceptance is downstream of this decision.

## Authority and state ownership checks

- The Realm Authority Principal alone issues Channel Grants, revocations,
  activation checkpoints, and planned successor transitions.
- The authority Node hosts the module but does not become the issuer through
  Node identity, Waku identity, transport identity, or filesystem possession.
- Authority mutations use exact Operator Interface admission. The Application
  Interface exposes no authority procedure or cryptographic material.
- The authority ledger is the sole membership, generation, delivery-phase,
  revocation, operation, sequence, and audit-chain truth.
- Each member Node owns only its installed grants, sender snapshot,
  revocations, delivery key, receipts, replay state, and active checkpoint.
- The checkpoint repository owns monotonic freshness evidence, not membership
  policy or Channel Grant authority.
- Deployment owns host isolation and produces fencing evidence; the authority
  decides whether that evidence is sufficient for membership completion.

## Bounds checklist

The review must preserve:

- one realm per authority instance;
- at most 256 realm members;
- at most 1,024 active channels;
- at most 256 members per channel;
- one pending and one receive-only previous generation per channel;
- four outstanding deliveries per member/channel;
- a 256 KiB maximum delivery envelope;
- grants and delivery attestations valid for at most 30 days;
- renewal beginning at least 24 hours before expiry;
- unactivated operations expiring within 24 hours;
- bounded audit retention that fails mutations closed on exhaustion.

## Restart, recovery, and security review

- Restart resumes one durable phase using the original request, operation, and
  delivery identities; it never infers completion from memory.
- Abort is allowed only before activation commit. After commit, repair rolls
  forward and never reinstates the old publishing generation.
- Repository unavailability after activation leaves the operation active but
  `checkpointing` and blocks another security mutation.
- A missing, lower, forked, non-monotonic, or unexpected repository head moves
  the authority to recovery-required and forbids blind repair.
- A signed old checkpoint is authentic but not fresh. Same-realm restore
  requires exact equality with the independently retained unique head and
  immutable predecessor history.
- Repository loss, ambiguous history, colocated fault domains, lost authority
  key, or unprovable freshness requires a new realm.
- A receipt MAC proves possession of `receipt_key` and the asserted phase
  binding only. It does not prove persistence or runtime behavior against a
  compromised holder.
- A suspect, modified, inconsistent, or unapproved member must be fenced even
  if its receipt MAC verifies.
- Logs, metrics, CLI output, audit, backups, and evidence must not expose
  plaintext grants, secrets, selectors, receipt keys, payloads, private
  endpoints, or Principal identifiers in public labels.

## Acceptance criteria

- [ ] Every item in the authority, state, bounds, and recovery checklists is
      represented without contradiction in ADR-0011.
- [ ] Exact actions/resources, Actor/Effective attribution, allowed
      one-hop Delegation, and non-delegable policy actions are unambiguous.
- [ ] Membership completion does not claim instantaneous revocation across a
      partition.
- [ ] Receipt wording consistently says trusted-host protocol attestation,
      never cryptographic proof of durable installation or honest activation.
- [ ] The independent checkpoint repository has unique-head read,
      create-if-absent only for stopped new-realm genesis, immutable history,
      and exact compare-and-append; delete/replace/truncate/blind-put paths are
      forbidden.
- [ ] Migration and downgrade cannot introduce mixed old/new authority
      management or recreate repository freshness from an archive.
- [ ] Federation and MLS remain explicitly unsupported for v1.
- [ ] The reviewer records `review-ready`, `returned with blockers`, or
      `rejected with rationale`.
- [ ] ADR status changes only after explicit maintainer approval and in a
      separate logical commit.

## Required checks and evidence

- Manual comparison of ADR-0011 with the DR-03 packet and Wave 3 synthesis.
- Documentation contract and architecture-acceptance checks.
- Capability catalogue consistency check showing no premature `I/R/O/Q`
  change.
- `git diff --check`.
- Retained review comments identify the reviewer, reviewed source commit, ADR
  outcome, and any blockers. Existing implementation tests are context only;
  they are not authority qualification evidence.

## Capability impact

- Capability: `realm.channel-grant-authority`.
- Review success authorizes the selected design only.
- Expected status after review remains the canonical
  `I=partial, R=no, O=no, Q=no` until product behavior and evidence change.
- ADR acceptance must not set `Q=yes`.

## Expected files and modules

- Review target:
  `docs/adr/0011-single-authority-channel-grant-lifecycle.md`.
- Evidence sources:
  `docs/engineering/research/channel-grant-authority.md`,
  `docs/engineering/research/wave3-decision-register.md`, and
  `docs/engineering/research/wave3-synthesis.md`.
- Tracker comments may be appended to this issue.
- No production module is changed by this review issue.

## Exit condition

The issue closes only after a maintainer records one explicit review outcome.
If accepted, ADR-0011 status and governance projections are updated separately
and CGA-01 may be triaged for implementation. If returned or rejected, all
dependent implementation and ADR acceptance remain blocked.

## Comments
