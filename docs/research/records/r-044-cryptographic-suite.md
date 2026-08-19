---
id: R-044
title: Which maintained cryptographic suite implements Name Authority, threshold Recovery Authorities, and resolver query hiding?
status: open
owner: Product Owner
started: 2026-08-19
reviewed: 2026-08-20
---

# R-044 — Stage 6 cryptographic suite

## Decision this unlocks

Select reviewed implementations and a replaceable interface for Name Authority
signing, a real `t`-of-`n` Recovery Policy operated by scoped Recovery
Authorities, and resolver query hiding. The selection must precede S6.2, S6.4,
and cryptographic evidence claims.

## Current contract

R-003 and CONTEXT.md separate Name Authority from the set of Recovery
Authorities. A Recovery Policy precommits participant scope, threshold, and a
visible delay; the current Name Authority alone cannot cancel or satisfy a
recovery intended to survive its compromise. No project coordinator, registrar,
or administrator may become an implicit recovery root.

Ed25519 and OHTTP remain plausible reused components. The previous ADR-0013
selection is withdrawn: `golang.org/x/crypto/bls12381` does not exist, and
ordinary BLS signature aggregation is not by itself a threshold-key setup,
share-generation, partial-signature, participant-authentication, or recovery
protocol. No Stage 6 threshold implementation is currently selected.

## Hypotheses

- **H1:** a maintained threshold-signature implementation with documented key
  generation and participant authentication can satisfy Recovery Policy.
- **H2:** a policy can verify `t` individually authenticated signatures without
  threshold aggregation while preserving bounded size and privacy.
- **H0:** no maintained Go implementation fits the trust and operational model;
  V1 recovery must be reduced or deferred rather than implemented locally.

## Evaluation criteria

1. Recovery is authorized by `t` distinct scoped Recovery Authorities, never by
   the current Name Authority alone.
2. Key/share setup, replacement, loss, participant identity, proof of
   possession, rogue-key defense, transcript binding, and domain separation are
   specified and independently testable.
3. Malformed, duplicate, unknown, stale-generation, and cross-policy shares fail
   closed without changing recovery state.
4. The implementation is maintained, versioned, license-compatible, reviewed,
   misuse-resistant, and recorded in `dependencies.md` before import.
5. Signature, proof, state, latency, CPU, and memory fit the R-023 reference
   profile.
6. Query hiding preserves the R-026/R-039 role boundary and has no less-private
   fallback.

## Evidence plan

### Primary sources

- R-003 and CONTEXT.md, accessed 2026-08-20 — Recovery Policy trust model.
- [RFC 8032](https://www.rfc-editor.org/rfc/rfc8032), accessed 2026-08-20 —
  Ed25519.
- [RFC 9591](https://www.rfc-editor.org/rfc/rfc9591), accessed 2026-08-20 —
  FROST threshold Schnorr signatures.
- [CFRG BLS signatures draft](https://datatracker.ietf.org/doc/draft-irtf-cfrg-bls-signature/),
  accessed 2026-08-20 — BLS signatures and aggregation.
- [`golang.org/x/crypto` source](https://github.com/golang/crypto), accessed
  2026-08-20 — confirms the formerly named package is absent.
- [CIRCL BLS API](https://pkg.go.dev/github.com/cloudflare/circl/sign/bls),
  accessed 2026-08-20 — available BLS aggregation API, not a complete Recovery
  Policy protocol.
- [RFC 9458](https://www.rfc-editor.org/rfc/rfc9458), accessed 2026-08-20 —
  Oblivious HTTP.

### Experiment

Create `experiments/r-044-recovery-crypto/` after candidate implementations are
identified. Exercise setup, `t-1`, `t`, and `n` participants; duplicate and
rogue keys; mixed policies/generations; lost participants; delayed completion;
restart; malformed shares; and independent verification. Measure retained bytes,
CPU, memory, and latency on the R-023 host.

### Failure scenarios

- Current Name Authority can satisfy or disable recovery alone.
- Aggregation accepts duplicate keys or fewer than `t` distinct participants.
- A participant/share is replayed across policy, name, or generation.
- Setup requires an undocumented trusted dealer or online coordinator.
- A malformed proof changes Recovery Pending state.
- Query hiding silently falls back to a direct or plaintext resolver.

## Findings

- **Sourced fact:** the accepted product model assigns recovery to distinct
  scoped Recovery Authorities.
- **Sourced fact:** the named `golang.org/x/crypto/bls12381` package is absent.
- **Sourced fact:** available BLS aggregation APIs do not define the complete
  threshold custody and recovery protocol required here.
- **Inference:** the earlier `ThresholdVerifier` interface was too shallow: it
  could not express participant identity, threshold policy, setup, or shares.
- **Assumption:** reusing Ed25519 and OHTTP may still be appropriate, but that
  does not authorize a partial suite before recovery is solved.

## Options

1. **FROST-based threshold recovery.** Candidate; requires maintained Go supply,
   key setup, participant authentication, and measured operational behavior.
2. **Individually authenticated `t`-of-`n` signatures.** Candidate; simpler
   trust semantics, larger proofs, and possible participant-linkage costs.
3. **BLS-based threshold recovery.** Candidate only with a real threshold
   construction and maintained implementation; ordinary aggregation is
   insufficient.
4. **Defer Recovery Policy.** Required fallback if no candidate meets the V1
   threat and maintenance model.

## Recommendation

Choose none yet. Run the experiment after a source review identifies at least
one maintained implementation. Confidence is high that ADR-0013 cannot be used;
confidence in a replacement is currently low. The strongest counterargument to
FROST is its setup and interactive operational burden for a one-person product.

## Disposition

- State: `open`; ADR-0013 is withdrawn and provides no import authorization.
- S6.2 query-hiding integration, S6.4 recovery, and their final evidence remain
  blocked. Existing R-026 OHTTP research may be reused as evidence, not assumed.
- Any selected runtime dependency requires `dependencies.md`, an accepted ADR,
  exact conformance vectors, and failure-path tests.
