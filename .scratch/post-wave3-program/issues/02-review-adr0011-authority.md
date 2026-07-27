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

### Architecture and security review — 2026-07-27

- Reviewer: Codex `/root`, lead review-agent for PW3-02.
- Reviewed source: exact commit
  `67d7cd7f665c395c2a14612564cd22a07b648453` on `main`; the starting
  worktree was clean and `HEAD` matched the expected commit.
- Outcome: `returned with blockers`.

#### Findings

- P0: none.
- P1 — the authority admission contract is not implementably unambiguous.
  `docs/adr/0011-single-authority-channel-grant-lifecycle.md:37-40` refers to
  exact procedures and existing Actor/Effective, Access Grant and one-hop
  Delegation rules, but does not record the authority-specific
  action/resource/delegation catalogue. The selected packet contains eight
  distinct rows at
  `docs/engineering/research/channel-grant-authority.md:426-439`, yet its
  ordinary one-hop rows also need reconciliation with ADR-0002's
  Principal-to-Application-only Delegation rule at
  `docs/adr/0002-principal-centered-identity-and-access.md:5-7` and the
  Operator/Application interface separation. Minimal correction: put all
  eight exact actions and resources in ADR-0011; explicitly choose a compatible
  admission rule for each ordinary action (direct-only `Actor == Effective`,
  or a separately approved identity/access change for the intended one-hop
  path); retain non-delegable recovery and authority rotation; and state the
  resulting Actor/Effective intersection and dual-attribution audit rule.
- P2 — renewal is not bound to a fresh generation.
  `docs/adr/0011-single-authority-channel-grant-lifecycle.md:58-68,164-166`
  requires fresh generations for membership changes and routine rotation, but
  gives renewal only a timing bound. The selected packet requires new grant IDs
  and a new generation so the complete bounded sender snapshot and receipt
  attestation advance together at
  `docs/engineering/research/channel-grant-authority.md:551-553`. Minimal
  correction: add that invariant to ADR-0011 and route renewal through the same
  durable installation/activation/checkpoint operation. This is a
  specification-completeness blocker; the stronger claim of a current runtime
  exploit was rejected because activation/readiness already fails stale
  members closed.

No other P0-P2 finding survived adversarial validation. In particular, the ADR
preserves the single deployment-owned authority Principal, separates the
authority Node/Waku/transport identities, keeps recovery and authority rotation
non-delegable, excludes the Application authority surface and secret material,
keeps authority/member/checkpoint ownership non-competing, makes receipt MACs
trusted-host protocol attestations only, fences suspect or partitioned members,
uses pre-commit abort and post-commit roll-forward, requires independent exact
freshness for same-realm restore, creates a new realm on lost or ambiguous
freshness, preserves every finite v1 bound, forbids mixed authority management,
requires full stopped-backup downgrade, and leaves federation, MLS and
threshold authority unsupported.

#### Checks

- `git status --short --branch` — clean starting tree on
  `main...origin/main`.
- `git rev-parse HEAD` —
  `67d7cd7f665c395c2a14612564cd22a07b648453`.
- `git diff --check` — passed.
- `go test ./tests/tooling/doccontract ./tests/tooling/archaccept -count=1` —
  passed.
- `go run ./tests/tooling/capabilitycatalog -check` — passed:
  `24 capabilities, 8 domains, 0 qualified`.
- `realm.channel-grant-authority` remains
  `I=partial, R=no, O=no, Q=no`; no capability status changed.

ADR-0011 remains `Proposed`. Research acceptance is not ADR acceptance. This
review package may be given to the maintainer for blocker disposition, but the
ADR is not ready to be presented for acceptance until the two corrections are
made and re-reviewed. CGA-01 remains blocked, and this review authorizes no
production implementation, capability promotion, or push.

### Blocker remediation and follow-up review — 2026-07-27

Both blockers were corrected in the uncommitted working tree based on
`main@67d7cd7f665c395c2a14612564cd22a07b648453`:

- The P1 authority admission gap is closed at
  `docs/adr/0011-single-authority-channel-grant-lifecycle.md:37-67`.
  ADR-0011 now records all eight exact actions and canonical resources,
  requires direct Operator admission with `Actor == Effective`, rejects
  Delegation as Operator call authority, retains Actor/Effective audit
  attribution, and keeps recovery and authority rotation non-delegable. This
  direct-only v1 decision is compatible with ADR-0002 and explicitly supersedes
  the provisional one-hop column in the research packet. Any future delegated
  authority call requires a new identity/access and interface decision.
- The P2 renewal gap is closed at
  `docs/adr/0011-single-authority-channel-grant-lifecycle.md:90-96`.
  Renewal now uses new grant IDs, a fresh secret and next generation, and
  advances the complete sender snapshot, receipts, activation checkpoint and
  external checkpoint through one durable operation. Same-generation
  sender-snapshot updates are unsupported.

Follow-up outcome: `review-ready`. The two required corrections are represented
without a remaining P0-P2 finding, and the technical packet is ready for a
separate explicit maintainer decision.

Post-remediation checks passed:

- `git diff --check`;
- `go test ./tests/tooling/doccontract ./tests/tooling/archaccept -count=1`;
- `go run ./tests/tooling/capabilitycatalog -check`:
  `24 capabilities, 8 domains, 0 qualified`.

ADR-0011 remains `Proposed`; `realm.channel-grant-authority` remains
`I=partial, R=no, O=no, Q=no`. Follow-up `review-ready` does not accept the ADR,
authorize CGA-01, promote a capability, or authorize a push. CGA-01 remains
blocked until the maintainer explicitly accepts ADR-0011.

### Maintainer disposition — 2026-07-27

- Maintainer decision-agent: Codex `/root`.
- Reviewed source: remediation commit
  `20e87867799bf160bde59107555bc1df6f5db906`, whose exact reviewed diff is
  `67d7cd7f665c395c2a14612564cd22a07b648453..20e87867799bf160bde59107555bc1df6f5db906`.
- Outcome: `returned with blockers`.

#### Finding

- P0: none.
- P1: none.
- P2 — restart and delivery retry identity remain underspecified.
  `docs/adr/0011-single-authority-channel-grant-lifecycle.md:128-133` preserves
  one request/operation identity and the durable phase, but does not preserve
  the original delivery identity and envelope bytes. The required restart
  contract at
  `.scratch/post-wave3-program/issues/02-review-adr0011-authority.md:112-113`
  and the selected DR-03 packet at
  `docs/engineering/research/channel-grant-authority.md:523-527` require normal
  retry/restart to reuse the original request, operation, delivery ID and
  envelope bytes. Only explicit reissue may increment the delivery retry
  generation and invalidate the prior receipt verifier while retaining the
  operation identity. Without that rule, delivery deduplication and receipt
  verification across restart are not decision-complete.

Minimal correction: add the original request, operation, delivery ID and
envelope-byte reuse invariant to ADR-0011; state that explicit reissue alone
increments delivery retry generation, invalidates the old receipt verifier and
retains the operation ID. Then rerun the maintainer review and required gates.

ADR-0011 remains `Proposed`. `realm.channel-grant-authority` remains
`I=partial, R=no, O=no, Q=no`; no `I`, `R`, `O` or `Q` projection changes.
CGA-01 remains `needs-info` and blocked; its implementation triage is not
authorized. No production implementation or push is authorized by this
disposition.

### P2 delivery-retry remediation — 2026-07-27

The maintainer P2 is corrected in the uncommitted working tree based on
`main@74aee897277b27212d99d250de423e9e1e4a0ce6`.

`docs/adr/0011-single-authority-channel-grant-lifecycle.md:128-146` now:

- persists the original request ID, operation ID, delivery ID, canonical sealed
  envelope bytes/digest, receipt verifier and delivery retry generation;
- requires ordinary retry and restart to reuse the original request,
  operation and delivery IDs and exact stored envelope bytes without resealing,
  minting another receipt key or allocating a replacement delivery identity;
- makes explicit reissue the only transition allowed to replace a delivery
  identity or envelope bytes, while retaining request/operation IDs,
  incrementing delivery retry generation, atomically persisting the replacement
  record and invalidating the previous receipt verifier.

Post-remediation checks passed:

- `git diff --check`;
- `go test ./tests/tooling/doccontract ./tests/tooling/archaccept -count=1`;
- `go run ./tests/tooling/capabilitycatalog -check`:
  `24 capabilities, 8 domains, 0 qualified`.

The corrected packet is ready for maintainer re-review, not implicitly
accepted. ADR-0011 remains `Proposed`; `realm.channel-grant-authority` remains
`I=partial, R=no, O=no, Q=no`; CGA-01 remains blocked until explicit maintainer
acceptance. No production code or capability projection was changed.
