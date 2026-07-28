# PW3-12: CGA-02 deliver and acknowledge one initial generation

Status: ready-for-human
State: open
Labels: ready-for-human
Research class: R0/R1 cryptographic integration with security review

## Parent

`../PRD.md`

## What to build

Deliver the smallest complete recipient-bound initial-generation journey:

```text
protected Operator delivery intent
  -> exact realm/channel/member admission and Product Policy
  -> member delivery-key attestation
  -> Authority HPKE generation bundle
  -> member atomic install
  -> stable installed receipt
  -> Authority acknowledgement and redacted status
  -> restart/retry returns the same operation and receipt identities
```

The authority owns `Deliver` and `Acknowledge`; the member Node owns
`PrepareDelivery` and `InstallDelivery`. The bundle contains a complete sender
snapshot and is bound to Realm, authority epoch/sequence, channel, generation,
recipient Principal and delivery-key identity. Receipt MACs attest possession
on an approved host; they do not prove honest persistence or runtime behavior.

The slice extends the existing bounded ledger/checkpoint transaction without
adding rotation, membership mutation, renewal, restore, Application Messaging,
or qualification.

## Acceptance criteria

- [x] All four procedures are reachable only through protected Operator
      admission with protocol-owned exact actions/resources and direct
      Actor/Effective attribution; no Application equivalent exists.
- [x] Delivery-key attestation and HPKE bundle use RFC 9180-compatible,
      versioned, bounded, deterministic contracts with retained known-answer
      vectors.
- [x] Bundle and receipt bind RealmID, authority epoch/sequence, channel ID and
      class, generation, recipient Principal and delivery-key identity.
- [x] Member install is atomic and idempotent; a crash after commit regenerates
      the same receipt identity and never installs a second generation.
- [x] Authority delivery, audit outbox, idempotency result, checkpoint
      sequence/digest and acknowledgement state have explicit transactional
      boundaries and restart reconciliation.
- [x] Wrong recipient/key, malformed/tampered/replayed/expired artifact,
      conflicting idempotency reuse, unknown fields and every declared bound
      fail closed with stable public errors.
- [x] Inspect, CLI, logs, metrics, diagnostics, audits, backups and test
      artifacts expose no plaintext Channel Grant, selector, receipt key,
      private endpoint or unbounded Principal label.
- [x] Tests explicitly demonstrate that a malicious key holder can forge an
      asserted installed phase; the contract describes this only as a
      trusted-host possession attestation.
- [x] Unit, contract, real-store restart/crash integration, RFC vector,
      security-negative, race, documentation, architecture and capability
      catalogue checks pass on one exact implementation commit.
- [x] Capability qualification remains unchanged and `Q=no`.

## Blocked by

- Accepted PW3-05 / CGA-01 implementation
  `3551a1e5f486d34711416cacfe21fc420d393c46`.

## Comments

- Published after the 2026-07-28 maintainer instruction to proceed with
  CGA-02 through CGA-07. CGA-01 was accepted and closed; no later CGA behavior
  is implied.
- 2026-07-28 CGA-02 implementation handoff:
  - exact starting commit:
    `d88adbc2fef56a85dd0b4adbaf55c88ad4da2bb5`;
  - exact logical implementation commit:
    `693ac7cb0e88661dccce8a97482ae14d53a5afd9`;
  - implemented the protected Operator-only
    `PrepareGenerationDelivery -> IssueInitialGeneration ->
    InstallGenerationDelivery -> AcknowledgeInitialGeneration` journey with
    protocol-owned exact actions/resources, direct Actor/Effective
    attribution, Product Policy admission and no Application equivalent;
  - recipient delivery-only bootstrap uses
    `privacy.delivery_enabled=true`, an encrypted capability store, a
    separately protected store key, local Principal identity and
    purpose-scoped `channel.issue` trust without requiring a pre-existing
    discovery/data grant;
  - the Authority emits a recipient-bound RFC 9180 HPKE bundle, the member
    installs the complete grant/sender/revocation snapshot atomically and
    returns a stable MACed receipt, and the Authority commits acknowledgement
    through one shared ledger/checkpoint transition;
  - idempotent replay checks the complete retained binding including expiry,
    and expired, malformed, tampered, wrong-key, wrong-recipient and
    conflicting artifacts fail closed. Tests explicitly prove that possession
    of the receipt key can forge an `installed` assertion and therefore is not
    proof of honest persistence;
  - exact validation on the implementation commit:
    - `go test ./...`;
    - `go vet ./...`;
    - `go test -race ./internal/authority
      ./internal/identity/capability ./internal/channeldelivery
      ./internal/localapi/authority ./internal/localapi/channeldelivery`;
    - `go test -tags integration ./tests/integration/localapi -run
      'TestRealmAuthorityGenesisInspectAndRestartThroughProtectedOperatorInterface|TestInitialGenerationDeliveryThroughProtectedOperatorInterface'
      -count=1`;
    - `scripts/generate-api.ps1 -Check`;
    - `go run ./tests/tooling/capabilitycatalog -check`;
    - `git diff --check`;
  - retained RFC 9180 Appendix A.2 known-answer coverage, real-store crash
    injection at both Authority checkpoint boundaries, member crash-after-
    commit replay and the protected four-RPC integration scenario are part of
    the committed test suites;
  - independent repeat Standards and Spec reviews both returned `PASS` with no
    remaining actionable P1-P3 findings;
  - the capability catalogue remains `24 capabilities, 8 domains, 0
    qualified`; no canonical qualification promotion occurred and `Q`
    remains `no`;
  - CGA-03 remains gated on explicit acceptance of this exact implementation
    commit. No production deployment was changed and nothing was pushed.
