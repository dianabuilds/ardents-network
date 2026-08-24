---
status: accepted
date: 2026-08-24
supersedes: none
---

# ADR-0034 — Bind a separate Service Introduction HPKE key in Credential v2

## Context

The closed `SealedIntroduction` v1 envelope in ADR-0026 uses X25519 HPKE.
The Authority-signed Service Credential previously bound only an Ed25519
`InstancePublic` key, which is used for Service Connection proof. It supplied
no authenticated HPKE recipient, and deriving one from an Ed25519 key would
create an unreviewed cross-protocol conversion rule.

## Decision

Credential v2 contains a required, separate 32-byte
`IntroductionHPKEPublic` X25519 public key. The existing Service Authority
signs it with the target, Ed25519 Instance key, generation, time bounds,
Network ID, and capabilities. It is the only recipient public key eligible
for the selected `SealedIntroduction` construction.

`InstancePublic` remains an Ed25519 Service Connection key; implementations
must neither derive nor substitute the X25519 key. Credential is a closed
canonical binary record: alpha readers accept only v2 and reject v1 rather
than carrying a compatibility decoder.

## Consequences

- Every live alpha Service publication must issue Credential v2 and rotate or
  republish on an introduction-key change.
- The signed recipient binding permits a later C-2 delivery transcript to
  encrypt Service-only material without adding a new key-distribution
  authority.
- This decision does not define SealedIntroduction plaintext, JoinHandle
  spending, remote delivery, Responder admission, or local Publisher handoff;
  R-105 remains open for those semantics.

## Compliance

R-105 records the schema gap and the Product Owner accepted this narrow
binding on 2026-08-24. Publication codec tests prove that a missing or
substituted recipient key is refused and that v1 is not decoded.
