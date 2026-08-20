---
id: R-057
title: How does private resolution authenticate the current Namespace without returning an unbounded parent chain?
status: decided
owner: Product Owner
started: 2026-08-20
reviewed: 2026-08-20
---

# R-057 — Current Namespace materialization

## Decision this unlocks

Authenticate the current root claim, every later authority/recovery transition,
and parent lineage at the Resolver while keeping the fixed private-resolution
response at 4096 bytes.

## Current contract

R-003 requires resolution to authenticate current Namespace state rather than
accept any self-signed Name Record. ADR-0004 already makes the Network Epoch
threshold the authenticated shared-state root and explicitly retains its
censorship/fork limitation. R-041 permits 127 labels, so returning the complete
signed child-to-root chain cannot meet the fixed R-047 response bound.

## Hypotheses

- **H1:** a threshold-signed current Namespace statement plus one Merkle
  membership proof authenticates a compact resolution leaf.
- **H2:** returning every parent Record remains bounded for every legal Name.
- **H0:** neither option fits; the legal name depth or fixed response must be
  redesigned.

## Evaluation criteria

1. A self-consistent Namespace signed only by substituted Name Authorities is
   rejected under the installed Network Epoch authority set.
2. The statement binds network, rule, epoch number and authenticated Epoch
   digest, cutoff, current Record root/count, accepted transition root/count,
   and deterministic rejection root/count.
3. One leaf binds the exact signed current Record and a threshold-materialized
   effective parent-lineage summary; S6E1 independently recomputes it from the
   full corpus.
4. The maximum legal 127-label Name fits the 4096-byte response.
5. Missing, stale, malformed, below-threshold, forked, or mutated evidence fails
   closed without a second Namespace or plaintext fallback.

## Evidence plan

### Primary sources

- RFC 9162, Merkle Tree Hashes and inclusion proofs, accessed 2026-08-20.
- ADR-0004, authenticated Network Epochs and captured-threshold limitation,
  accessed 2026-08-20.
- R-003, R-041, R-043, R-047, and R-055, accessed 2026-08-20.

### Experiment

The maintained S6.6 corpus is the bounded experiment. It retains the complete
Record/transition/rejection inputs, independently recomputes roots and lineage,
mutates every statement/proof field, and measures the largest legal proof in
the real fixed-size resolution codec.

### Failure scenarios

- malicious Gateway substitutes a new self-signed root and Target;
- one signer, duplicate signer, unknown signer, wrong network/rule/epoch;
- changed leaf, ordinal, tree length, sibling, root, transition, or rejection;
- stale materialization after rotation, recovery, Release, or parent reclaim;
- two valid threshold statements for one network/rule/epoch;
- legal deep Name exceeds the fixed response.

## Findings

- **Sourced fact:** RFC 9162 inclusion proves membership in a committed tree;
  it does not prove honest completeness or prevent signer censorship.
- **Sourced fact:** ADR-0004 already accepts the Network Epoch threshold as the
  visible shared-state authority and requires authenticated forks to fail closed.
- **Measurement:** the maintained 127-label fixture produces a `1667`-byte
  compact proof, independently recomputed from all `127` signed current Records
  in S6E1, below the fixed `4096`-byte resolution response bound.
- **Inference:** signing only each Name Record authenticates its current key but
  cannot authenticate that the root claim or later transition is the accepted
  current Namespace state.
- **Assumption:** the complete S6E1 corpus is sufficient development evidence;
  independent signer governance and censorship resistance remain later gates.

## Options

1. **Threshold-signed Namespace root plus compact proof.** Chosen. Reuses the
   accepted Network Epoch trust root and keeps response work logarithmic.
2. **Complete child-to-root signed chain.** Rejected: response grows with the
   legal 127-label depth and exceeds the fixed profile.
3. **Trust the Gateway or current Name Authority.** Rejected: either can replace
   the accepted Namespace state with a self-consistent substitute.
4. **Add another registrar/consensus root.** Rejected: duplicates the accepted
   Network Epoch authority and introduces an unresearched foundation.

## Recommendation

Choose option 1. Confidence is high for bounded authentication and medium for
the eventual governance outcome. The strongest counterargument is decisive: a
captured threshold can censor or fork the entire Namespace. The design exposes
and fails closed on observable forks; it does not eliminate threshold capture.

## Disposition

- State: `decided`; the Product Owner accepted option 1 on 2026-08-20.
- ADR-0020 records the durable trust and response decision.
- S6.3-S6.6 must use the threshold statement and compact proof; a bare
  self-signed Record chain cannot receive Stage 6 completion credit.
- S6E1 owns complete-corpus recomputation, mutation coverage, the exact
  `1667`-byte maximum-depth proof measurement, and the honest
  captured-threshold limitation.
