# PW3-12: CGA-02 deliver and acknowledge one initial generation

Status: ready-for-agent
State: open
Labels: ready-for-agent
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

- [ ] All four procedures are reachable only through protected Operator
      admission with protocol-owned exact actions/resources and direct
      Actor/Effective attribution; no Application equivalent exists.
- [ ] Delivery-key attestation and HPKE bundle use RFC 9180-compatible,
      versioned, bounded, deterministic contracts with retained known-answer
      vectors.
- [ ] Bundle and receipt bind RealmID, authority epoch/sequence, channel ID and
      class, generation, recipient Principal and delivery-key identity.
- [ ] Member install is atomic and idempotent; a crash after commit regenerates
      the same receipt identity and never installs a second generation.
- [ ] Authority delivery, audit outbox, idempotency result, checkpoint
      sequence/digest and acknowledgement state have explicit transactional
      boundaries and restart reconciliation.
- [ ] Wrong recipient/key, malformed/tampered/replayed/expired artifact,
      conflicting idempotency reuse, unknown fields and every declared bound
      fail closed with stable public errors.
- [ ] Inspect, CLI, logs, metrics, diagnostics, audits, backups and test
      artifacts expose no plaintext Channel Grant, selector, receipt key,
      private endpoint or unbounded Principal label.
- [ ] Tests explicitly demonstrate that a malicious key holder can forge an
      asserted installed phase; the contract describes this only as a
      trusted-host possession attestation.
- [ ] Unit, contract, real-store restart/crash integration, RFC vector,
      security-negative, race, documentation, architecture and capability
      catalogue checks pass on one exact implementation commit.
- [ ] Capability qualification remains unchanged and `Q=no`.

## Blocked by

- Accepted PW3-05 / CGA-01 implementation
  `3551a1e5f486d34711416cacfe21fc420d393c46`.

## Comments

- Published after the 2026-07-28 maintainer instruction to proceed with
  CGA-02 through CGA-07. CGA-01 was accepted and closed; no later CGA behavior
  is implied.
