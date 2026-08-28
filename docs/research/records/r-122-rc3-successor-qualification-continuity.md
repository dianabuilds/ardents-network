---
id: R-122
title: RC3 successor qualification continuity
status: open
owner: Product Owner and Codex
started: 2026-08-28
reviewed: 2026-08-28
---

# R-122 — Which bounded RC2-to-RC3 successor operation can bind a corrected A11 harness to one new immutable functional-alpha candidate without widening local custody?

## Decision this unlocks

An immutable `h4-alpha-1-rc-3` candidate, if the Product Owner accepts it,
whose source tag, release/control inputs, H4-6A evidence, and A11 campaign all
name the same exact revision. The decision does not accept the candidate or
authorize a long A11 campaign by itself.

## Current contract

ADR-0052 deliberately permits only the recorded RC1-to-RC2 successor. Its
`BuildAlphaSuccessor` policy fixes RC2 source and Linux program digests,
requires the RC1 root/catalog predecessor, produces only generation-2 inputs,
and exposes no generic signing interface. RC2 is immutably tagged at
`2c18bdf92f11f84075915576f595202f48eb05bc`.

The A11 normal-path fault was in the qualification fixture rather than the
Endpoint binary: Publisher Application applied its five-second cycle limit
before the local Browser/Proxy origin was ready. Commit
`9ef9ebc4946915b161e5df792d885a376ac10633` adds a bounded 15-second initial
assembly window while preserving the five-second limit for each observed HTTP
cycle. Its 10-cycle Windows-to-Ubuntu canary passed with 10/10 cycles, one
proxy dial, no redial, and retained complete User and remote evidence.

The official A11 runner refuses to combine that corrected harness with RC2:
the candidate worktree HEAD, immutable release tag, source revision, and
harness revision must agree. Reusing RC2 evidence after the correction would
contradict that contract; retagging RC2 would violate immutability.

## Hypotheses

- **H1:** one new fixed RC2-to-RC3 custody operation can accept only the
  recorded RC2 static directory, one precommitted RC3 source revision and
  exact two-builder Endpoint/control bytes, emit only generation-3 metadata,
  verifier-preflight it, and preserve all secret and publication boundaries.
- **H2:** the corrected A11 harness can be separately versioned without a new
  candidate while retaining an auditable exact candidate-to-harness binding.
- **H0:** neither operation is acceptable; H4-3/H4-8 remain open and the
  passing short canary stays diagnostic evidence only.

## Evaluation criteria

- The successor accepts no caller-selected signer, target, role, metadata
  generation, root, path, URL, upload destination, or opaque signed bytes.
- It reads and validates the exact accepted RC2 directory before requesting a
  secret, keeps the root role unchanged, and accepts only the recorded RC2
  catalog/component lineage.
- It fixes the RC3 source revision, candidate release identity, Endpoint and
  control SHA-256 values, target descriptor, validity/safety bounds, and
  network identity in maintained code and tests before the Product Owner
  enters a passphrase.
- The operation preflights the complete generation-3 TUF/ACA1/ACS1 material
  with maintained readers, atomically publishes only a previously absent
  external directory, and returns a non-secret receipt.
- A changed byte, predecessor, request field, release identity, clock bound,
  or output path is rejected before publication. No generic signing or
  authorization capability is introduced.
- The candidate must repeat all affected gates: deterministic checks, A1-A10,
  two-fresh-Endpoint H4-6A, and the official A11 6/6 campaign. Earlier RC2
  evidence remains provenance and cannot be promoted as RC3 evidence.

## Evidence plan

### Primary sources

- ADR-0050, ADR-0051, ADR-0052, R-100, and R-120, inspected 2026-08-28.
- `internal/release/custody/alpha_inputs_contract.go` and
  `tests/qualification/h4-8-a11/{invoke-windows,run-windows}.ps1`, inspected
  2026-08-28.
- Retained short-canary evidence at
  `C:\Users\vitek\AppData\Local\Temp\ardents-a11-after-initial-grace`,
  observed 2026-08-28. It is diagnostic evidence, not a candidate receipt.

### Experiment

After an accepted ADR and exact precommitted RC3 identity exist, build the
Linux Endpoint/control pair twice from the tagged clean source. Verify byte
identity, exercise altered-byte/predecessor/request/output refusals, invoke
the fixed local successor once into a previously absent external directory,
and independently preflight and archive the resulting bundle. Then run the
required RC3 H4-6A and A11 profiles with unique external evidence roots. A
failed attempt remains retained and cannot be erased by a later pass.

### Failure scenarios

- The runner permits corrected harness bytes to qualify an older candidate.
- An RC2 predecessor is replaced, weakened, or mixed with another catalog or
  component lineage.
- A caller turns the successor into a generic metadata signer or chooses an
  arbitrary future generation.
- Artifact/source/tag identities disagree, expiry crosses during construction,
  a partial output remains, or a private secret reaches logs, evidence, VPS,
  repository, archive, or command arguments.
- A later success masks an early A11 failure or an unavailable selected
  environment is reported as a pass.

## Findings

- **Current-contract fact:** the existing successor function checks RC1
  predecessor digests and accepts only `h4-alpha-1-rc-2`, generation 2, and
  the fixed RC2 source/program identities.
- **Measurement:** the prior normal-path failure was a remote
  `publisher-app` I/O timeout before the first HTTP byte; the corrected
  10-cycle canary completed 10/10 cycles in 10.13 seconds and preserved
  end-to-end byte counters and clean terminal evidence.
- **Inference:** changing the A11 harness after RC2 was tagged creates a real
  evidence-integrity discontinuity, even if the prior Endpoint executable
  digest happens to remain unchanged. The official runner correctly rejects
  that discontinuity.
- **Inference:** H1 preserves the narrowest available custody boundary. H2
  would weaken the runner's source-to-evidence guarantee and is therefore not
  acceptable for H4 closure.

## Options

1. **Fixed RC2-to-RC3 successor.** Add one separate, policy-pinned operation
   for exactly one RC3 identity, generation-3 metadata, and the recorded RC2
   predecessor. This preserves immutable candidates and makes the full A11
   receipt meaningful. It requires a new accepted ADR, code review, fresh
   artifact construction, and requalification.
2. **Permit a harness/candidate revision mismatch.** This avoids a successor
   candidate but severs the runner's exact source binding. Reject: it permits
   a post-tag test change to create evidence for older release bytes without a
   defined audit boundary.
3. **Retag or overwrite RC2.** Reject: it destroys immutability and invalidates
   retained evidence.
4. **Do not issue a successor.** Retain the canary and failure disposition,
   leave H4-3/H4-8 open, and make no broader functional-alpha claim.

## Recommendation

If the Product Owner wishes to close H4-3/H4-8 now, choose option 1 through a
new ADR. The ADR must describe one exact RC2-to-RC3 policy, not a reusable
release generator. **Confidence:** high that options 2 and 3 are inconsistent
with the accepted evidence contract; medium that the RC3 implementation will
remain small until its fixed identity and refusal tests are reviewed. The
strongest argument against option 1 is that the A11 fixture change, rather
than an Endpoint-byte change, forces a new candidate ceremony and dependent
gate repeat.

## Disposition

Open. Product Owner acceptance of a new ADR is required before implementation,
candidate construction, or use of the encrypted custody record. Until then
the short canary is retained diagnostic evidence and H4-3/H4-8 remain open.
