---
status: accepted
date: 2026-09-26
---

# ADR-0094 — Retire the sealed Introduction v1 grammar and the v1 Introduction instruction codecs

## Context

ADR-0093 retired the Route v2 execution closure but retained two bounded
survivors under the F-42 disposition boundary: the byte-exact sealed
Introduction v1 grammar in `internal/route/sealed_introduction.go` and the
orphaned publication v1 `IntroductionInstruction` codecs, both awaiting one
superseding ADR-0035 record. The dependency audit after ADR-0093 confirms the
wait is over: `instance.Binding.OpenIntroduction` has no production caller on
any platform, the instruction encoder, decoder, and validator have no caller
at all, and the only positive exercise of the sealed grammar was the
`cmd/ardents` service-instance compatibility test. The selected text runtime
delivers introductions through the private v3 capsule path
(`Binding.NewPrivateRecipient` / `PrivateRecipient.OpenPrivateIntroduction`,
ADR-0081); nothing composes a sealed v1 recipient.

## Decision

1. Supersede ADR-0035: the v1 `ServiceIntroductionInstruction` plaintext
   grammar is no longer verifiable in code. Retire
   `internal/service/publication/introduction_instruction.go` with its test;
   ADR-0035 (whose live Introduction slots and EndpointTransitBinding v1 were
   already retired by ADR-0093) moves to superseded status with this record
   as its replacement.
2. Retire the sealed Introduction v1 grammar: `SealedIntroduction`,
   `IntroductionRecipient`, the Encode/Decode/Seal/Open codecs, the HPKE
   recipient adapter, and the relocated v2 envelope framing in
   `internal/route/sealed_introduction.go` with its canonical-vector test.
   ADR-0026 and ADR-0034 remain the historical provenance of the wire bytes;
   the vectors retire with the grammar.
3. Split the old decryptor out of the current Instance lifecycle: remove
   `Binding.OpenIntroduction` and its sealed-info constant from
   `instance/lifecycle.go`; the file keeps all current Accept, Binding,
   CommitPublished, and withdrawal behavior. Retire the sealed-opening
   compatibility test and its fixture helper in `cmd/ardents`; the stable
   request/response command test is unchanged.
4. Retain the `IntroductionPublic` key data contract: every new Instance
   still generates the X25519 key pair, the canonical request still commits
   the public key, Custody still embeds it as `IntroductionHPKEPublic` in the
   signed Credential v2 (ADR-0034), and acceptance still compares the two.
   Stopping new key emission requires a superseding decision for ADR-0034
   with a compatible Credential/Instance transition, explicit migration or
   typed refusal for existing roots, and an exit gate for the last reader —
   a separate card, not a method deletion. `Binding.IntroductionPublic()`
   survives as the accessor for the recipient-separation invariants asserted
   by the Endpoint and Instance tests.
5. The retired v2 `Profile` refusal identity moves to
   `internal/route/closed_node_carrier.go`; the Node typed refusals in
   `admission.go` and `duty_server.go` are unchanged.
6. Update the guards and documents: the Route v2 closure retirement guard
   extends its absence inventory with the four deleted files and flips the
   sealed-grammar retention checks into absence and data-contract checks; the
   deadcode allowlist drops the sealed-grammar and instruction-codec groups;
   the package map and `network-route-node.md` record the retired grammar.

## Consequences

`internal/route` now contains only the closed v3 Carrier/wire path, the
refusal `Profile` identity, and the closed token issuer; the whole
generation-2 Route wire surface is gone. `internal/service/instance` keeps
its lifecycle and the private v3 recipient; `internal/service/publication`
keeps the durable publication root without the v1 instruction grammar. The
only remaining Route v2-era obligation is the Introduction key data contract,
tracked as one migration-or-refusal card with a bounded exit gate.
