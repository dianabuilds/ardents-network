---
id: R-044
title: Which maintained cryptographic mechanism implements threshold Recovery Authorities?
status: open
owner: Product Owner
started: 2026-08-19
reviewed: 2026-08-20
---

# R-044 — Stage 6 cryptographic suite

## Decision this unlocks

Select a reviewed implementation and complete protocol for a real `t`-of-`n`
Recovery Policy operated by scoped Recovery Authorities. R-047 separately owns
the decision-ready narrow S6.2 Name Authority/Record authentication and query
hiding profile; this record remains the S6.4 threshold-recovery gate.

## Current contract

R-003 and CONTEXT.md separate Name Authority from the set of Recovery
Authorities. A Recovery Policy precommits participant scope, threshold, and a
visible delay; the current Name Authority alone cannot cancel or satisfy a
recovery intended to survive its compromise. No project coordinator, registrar,
or administrator may become an implicit recovery root.

The previous ADR-0013 selection is withdrawn: `golang.org/x/crypto/bls12381` does not exist, and
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
6. Recovery signatures cannot be replayed as R-047 Name Record or control
   signatures; domains and canonical transcripts are distinct.

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
- [Go `crypto/ed25519`](https://pkg.go.dev/crypto/ed25519), accessed
  2026-08-20 — maintained standard-library RFC 8032 signing and verification;
  private-key operations are documented as constant time.
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

The frozen O2 experiment uses the worst supported `5-of-8` policy with all
eight signatures, caps logical policy-plus-proof bytes at `2 KiB`, verification
at `2 ms` p95 on the declared weaker `1 vCPU/512 MiB` Linux container, and Go
heap allocation at `8 KiB` per verification. Every hostile vector must have
zero false authorizations. These are experiment gates, not a reliability claim.

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
- **Inference:** S6.2 Ed25519/OHTTP and S6.4 threshold recovery are separable
  decisions because the current Name Authority must not hold recovery shares or
  satisfy the recovery threshold alone.

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

## Decision-ready candidate O2 — individually authenticated threshold

This candidate is not accepted until its experiment passes and the Product
Owner chooses it. It interprets `t`-of-`n` literally: one proof carries at least
`t` distinct RFC 8032 Ed25519 signatures rather than hiding them behind an
aggregate or distributed key.

- One generation-scoped Recovery Policy contains `2 <= t <= n <= 8`, exactly
  `n` strictly ordered canonical Ed25519 public keys, one visible pending delay
  from `72 hours` through `30 days`, a monotonic policy revision, and its
  domain-separated digest.
- No Recovery Authority key may equal the current Name Authority. Duplicate or
  unknown participants, duplicate signatures, and non-canonical order fail.
- Every participant signs the same canonical transcript: domain literal,
  network, canonical Name wire bytes, generation, effective policy digest,
  recovery operation identifier, successor Name Authority, start boundary, and
  completion boundary.
- At least `t` valid distinct signatures initiate Recovery Pending. The current
  Name Authority cannot initiate, cancel, shorten, or extend it alone.
- Cancellation requires another `t`-of-`n` proof under the same effective policy
  and a distinct cancellation domain. Without cancellation, the precommitted
  successor becomes eligible exactly at the fixed boundary and must publish a
  fresh monotonic Name Record before resolution resumes.
- Add/replace/disable of a Recovery Policy is delayed by the same visible policy
  delay. The preceding policy remains effective until the change completes, so
  a compromised current authority cannot erase already usable recovery.

O2 needs no dealer, DKG, aggregation, proof of possession, or new runtime
dependency. Its intentional costs are linear proof size and public participant
keys/signatures. At `n = 8`, raw keys plus signatures are `768` bytes before
bounded framing; participant privacy is not claimed.

## Measurements

- **Measurement:** the positive `2-of-3` case and hostile `t-1`, duplicate,
  unknown, current-authority, cross-network/name/generation/policy, successor,
  deadline, operation-domain, and malformed-signature vectors passed ten
  consecutive runs with zero false authorization and no input mutation.
- **Measurement:** the worst supported `5-of-8` proof used `1,248` logical
  policy-plus-proof bytes. On the Windows endpoint it allocated about `1,721`
  heap bytes per verification and completed within the clock's approximately
  `1 ms` resolution at p95.
- **Measurement:** in the pinned Ubuntu image, offline/read-only, capped to
  `1 vCPU`, `512 MiB`, `64` PIDs, no capabilities, and no network, `10,000`
  worst-case verifications used `1,720` heap bytes per run and completed in
  `0.404001 ms` p95 (`0.242362 ms` p50, `6.250771 ms` maximum). O2 passed every
  frozen resource gate.
- **Inference:** individual signatures are materially simpler than FROST/BLS
  for this bounded policy and fit the tracer budget. The experiment does not
  prove participant independence, custody quality, or recovery availability.

## Recommendation

Choose O2 after Product Owner review and record a replacement recovery ADR.
Confidence is high that ADR-0013 cannot be used and high that individually
authenticated Ed25519 signatures implement the bounded cryptographic threshold.
The strongest counterargument is that policy membership and every signer become
visible and proof size grows linearly; neither participant privacy nor recovery
availability is provided.

## Disposition

- State: `open`, O2 decision-ready; ADR-0013 is withdrawn and provides no import
  authorization.
- S6.4 recovery and its final evidence remain blocked. R-047 is the separate
  decision-ready gate for S6.2 authentication and query hiding.
- Any selected runtime dependency requires `dependencies.md`, an accepted ADR,
  exact conformance vectors, and failure-path tests.
