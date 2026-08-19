---
id: R-044
title: Which cryptographic mechanism or replaceable interface for Name Authority signing, Recovery Policy threshold, and resolver query hiding is accepted for S6.0?
status: decided
owner: Product Owner
started: 2026-08-19
reviewed: 2026-08-19
---

# R-044 — Stage 6 cryptographic suite

## Decision this unlocks

Freeze the Stage 6 cryptographic suite: the signature scheme used by
Name Authority, the threshold scheme used by Recovery Policy, the
query-hiding scheme used by the resolver, the hash and KDF primitives,
and the recommended Go interface boundary that lets a future ADR swap
an implementation without rewriting S6.2, S6.4, or S6.5. Without this
freeze, no Stage 6 implementation slice could import a primitive
without violating the research discipline rule against silent
technology selection. With this freeze, every Stage 6 cryptographic
import is justified by the ADR that records the selection.

## Current contract

R-039 § Fixed product contract fixes:

- Name Authority rotation or transfer is an authenticated successor
  transition;
- optional recovery requires a precommitted delayed threshold
  Recovery Policy;
- exact-name resolution and naming control operations separate endpoint
  location from the exact name/lookup view for any one ordinary Node.

R-009 and R-035 already use Ed25519 for Bridge Invite signing and
Bridge state. R-026 selected `openpcc/ohttp v0.0.80` at commit
`79bec89d8042` for the Gate C Private Resolution Adapter. R-041 fixes
`schema_version = 1` and the canonical encoding. R-042 fixes the
order key. R-046 fixes the role matrix. ADR-0009 fixes the Go project
foundation. ADR-0012 fixed the Stage 5 Camouflage Adapter.

What remains open before S6.2, S6.4, and S6.5 can start is the
concrete cryptographic suite and the interface boundary that lets a
future ADR replace an implementation.

## Hypotheses

- **H1:** the existing H3 supply (Ed25519, SHA-256, OHTTP) is
  sufficient for Name Authority and resolver query hiding, and a
  single new dependency (`golang.org/x/crypto/bls12381`) is sufficient
  for Recovery Policy threshold.
- **H2:** a memory-hard post-quantum suite is required.
- **H0:** the existing H3 supply cannot satisfy the R-039 contract
  for Recovery Policy threshold.

## Evaluation criteria

1. **No local primitive.** Every Stage 6 cryptographic import is a
   reviewed Go standard library symbol or a reviewed external
   dependency listed in the ADR.
2. **Suite coverage:** Name Authority signing, Recovery Policy
   threshold, resolver query hiding, hash, and KDF are all fixed.
3. **No new trust root:** the threshold scheme is operated by the
   existing Name Authority holder, not by a new coordinator.
4. **Compatibility:** the suite does not break R-013 (Carrier Lab
   Ed25519), R-026 (OHTTP), R-035 (Bridge state), or R-009 (Bridge
   Invite) supply identities.
5. **Threshold correctness:** the threshold scheme provides
   unforgeability under the accepted `t`-of-`n` policy and
   fail-closed behavior on missing signatures.
6. **Query-hiding isolation:** the query-hiding scheme prevents the
   endpoint-adjacent role from learning the exact Service Name or
   lookup value.
7. **Falsification:** a Stage 6 slice imports a primitive not in the
   ADR; a Stage 6 slice builds a custom primitive; a Stage 6 slice
   bypasses the interface boundary.

## Evidence plan

Primary sources, accessed 2026-08-19:

- R-013 — Carrier Lab technology candidates (Ed25519 baseline).
- R-009 — hostile bootstrap and Bridge entry (Bridge Invite signing).
- R-026 — Private Resolution Adapter selection (OHTTP at
  `openpcc/ohttp v0.0.80` commit `79bec89d8042`).
- R-035 — H3 Bridge state (Bridge auth).
- ADR-0009 — Go project foundation.
- ADR-0012 — Stage 5 camouflage selection.
- RFC 8032 — Edwards-curve Digital Signature Algorithm (Ed25519).
- IETF draft `draft-irtf-cfrg-bls-signature-04` — BLS12-381 signature
  scheme.
- RFC 5869 — HMAC-based Extract-and-Expand Key Derivation Function
  (HKDF).
- FIPS 180-4 — Secure Hash Standard (SHA-256).
- Go standard library `crypto/ed25519`, `crypto/sha256`, `crypto/hkdf`.
- `golang.org/x/crypto/bls12381` — Go team maintained BLS12-381
  implementation (BSD-3-Clause).

The selected suite is recorded in ADR-0013. Stage 6 code is not
authorized until the ADR is accepted and the readiness checklist §B.4
is checked.

## Failure scenarios

- A Name Record is signed with anything other than the selected
  signature scheme.
- A Recovery Policy proof is checked as a single signature, not as a
  threshold aggregation.
- A resolver query is visible to the endpoint-adjacent role in plain
  bytes.
- A Stage 6 slice imports a hash that is not SHA-256 or a KDF that is
  not HKDF-SHA-256.
- A Stage 6 slice builds a custom cryptographic primitive.
- A Stage 6 slice imports a cryptographic dependency that is not
  listed in the ADR.
- A future replacement of an implementation bypasses the documented
  Go interface boundary.

## Options and recommendation

1. **Option A — Ed25519 (name) + BLS12-381 (threshold) + OHTTP (reused)
   + SHA-256 + HKDF-SHA-256 (recommended).** The suite reuses the
   verified H3 supply where it exists and adds one new Go-team
   dependency (`golang.org/x/crypto/bls12381`) for the threshold
   scheme.
2. **Option B — Ed25519 only.** No native threshold. Rejected: the
   `t`-of-`n` Recovery Policy cannot be expressed natively.
3. **Option C — BLS12-381 for everything.** Single curve. Rejected:
   signature size and verification cost are larger than needed for
   single-signer Name Records.
4. **Option D — Post-quantum (Dilithium + Kyber).** Forward-looking.
   Rejected: no H3 precedent; doubles the dependency surface without
   a threat-model driver in R-001 / R-009 / R-035.

Recommendation: **Option A**, accepted by the Product Owner on
2026-08-19. The recommended Go interface boundary (`Signer`,
`ThresholdVerifier`, `QueryHider` in `internal/nameauthority/sig.go`)
is the implementation pattern that any Stage 6 slice is expected to
use so a future ADR can swap an implementation without rewriting
S6.2, S6.4, or S6.5.

## Disposition

- R-044 becomes `decided`. The open row in `docs/research/questions.md`
  is updated to point at this record and the ADR.
- **ADR-0013** records the suite and the interface boundary.
- §B.4 of `stage-6-readiness-checklist.md` is checked.
- S6.2, S6.4, and S6.5 may import the selected primitives. Any other
  import is a new research question and a new ADR.
- A future replacement of an implementation (e.g., a new signature
  scheme, a new KDF, a new query-hiding primitive) requires a new
  research record and a superseding ADR.
- The suite is not a production cryptographic audit. Every Stage 6
  cryptographic import remains subject to the repository's existing
  dependency and advisory policy in `docs/development/dependencies.md`.
