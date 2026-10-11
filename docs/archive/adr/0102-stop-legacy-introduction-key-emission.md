---
status: accepted
date: 2026-09-26
supersedes: 0034
---

# ADR-0102 — Stop legacy Service Introduction key emission; Credential v3

## Context

ADR-0034 bound a separate X25519 `IntroductionHPKEPublic` key into the
signed Service Credential v2 as the only recipient eligible for the
`SealedIntroduction` v1 envelope. ADR-0094 retired that envelope's
decryptor and grammar, and ADR-0081 selected the closed private v3 capsule
path, whose recipient key is a fresh volatile `Binding.NewPrivateRecipient`
issuance. After those decisions the persisted legacy key had no
cryptographic consumer:

- `cmd/ardents service-instance initialize` still generated an X25519
  keypair for every new root and persisted both halves;
- the canonical request grammar still committed the public half;
- Custody's `serviceSuccessor` still echoed it into the signed Credential;
- the Instance acceptance check still compared the echo;
- the Endpoint copied it into `setup.IntroductionPublic`, which
  `newEndpoint` length-validated and then discarded — the endpoint struct
  never stored it;
- `Binding.IntroductionPublic()` had no production caller and survived only
  as a guard-asserted data-contract witness.

F-42 set the disposition boundary: stop new writes of the old key only
through a superseding decision for ADR-0034, an explicit Instance
request/Credential contract change, and a disposition for both phase-less
and current stored roots, with an exit gate for the old reader.

## Decision

1. **Credential v3.** `publication.Credential` loses the
   `IntroductionHPKEPublic` field; the signed body drops to version 3 with
   32 fewer bytes. Following ADR-0034's own closed-record rule ("readers
   accept only v2 and reject v1 rather than carrying a compatibility
   decoder"), the decoder accepts only v3. ADR-0034's recipient-binding
   requirement is superseded: the current private capsule path issues its
   own volatile recipient keys, so no persisted X25519 recipient remains in
   the signed delegation.
2. **Request grammar v2.** The canonical request domain becomes
   `ardents-service-instance-request-v2\x00` and drops the 32-byte
   `IntroductionPublic` field; the commitment space is disjoint from v1.
   The command's operator-facing schema label moves to
   `ardents-service-instance-request-v2`. Old request bytes are public
   operator input with no persisted status and parse as invalid.
3. **Response grammar v2.** `ardents-service-instance-response-v2\x00`
   carries the v2 request commitment and the v3 Credential bytes.
4. **Instance root schema v2.** The state schema and directory marker
   become `ardents-service-instance-root-v2`. `generateState` no longer
   generates any X25519 material; the stored JSON loses
   `introduction_public`/`introduction_private`; the phase-less
   rederivation branch (a pre-phase legacy shape that reconstructed both
   public keys and the request commitment from private bytes) is deleted.
5. **Typed refusal of pre-v3 roots.** A v1 marker or a v1/unknown state
   schema returns the new typed `ErrLegacyRoot` before any strict decode;
   no v1 state decoder is retained. The refusal is recognition only — that
   absence of any full v1 read path is the exit gate for the old reader.
   Refused root bytes stay on disk as evidence; the operator path is
   re-initialization under a new root.
6. **Pre-v3 publication records refuse themselves.** `publicationSize` and
   `credentialSize` shrink by 32 bytes, so pre-v3 generation records fail
   the fixed-size record parse with the existing typed "publication record
   is malformed" refusal. Pre-v3 publication roots must be republished
   under a fresh Credential v3; their bytes remain as refused evidence.
   The reachability Descriptor store does not embed Credential bytes, so
   persisted Descriptors are unaffected.
7. **Accessor and dead-field removal.** `Binding.IntroductionPublic()` is
   deleted, and the ADR-0094-era guard clause that preserved it as a
   retained data-contract witness flips to forbidding it. The Endpoint
   `setup.IntroductionPublic` discard field is deleted.

## Consequences

- No new durable or signed data carries the legacy Introduction key; the
  F-42 emission card closes. The historical grammar stays verifiable as
  Git-history and ADR-0034/0094 evidence; nothing in the working tree can
  produce or accept it.
- Existing Instance roots and publication generations become typed-refused
  evidence. The chosen disposition is refusal, not migration: this closed
  test network has no supported operator population, no production consumer
  ever read the key, and refusal preserves the exact bytes.
- F-32 (old Descriptor root policy in the reachability store) remains a
  separate open card; refusing pre-v3 publication generations does not
  dispose of that store's legacy reader.
- The Endpoint's publication composition keeps its current authenticated
  inputs (Authority public key, Instance binding, publication root); only
  the discarded field disappears from its setup.
