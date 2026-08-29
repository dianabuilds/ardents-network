---
id: R-124
title: What evidence can qualify independently inspectable public control?
status: active
owner: Product Owner and Codex
started: 2026-08-29
reviewed: 2026-08-29
---

# R-124 — What evidence can qualify independently inspectable public control?

## Decision this unlocks

H4-6C may select a concrete public-control candidate only when its custody,
Candidate View construction, builds, and audits can be independently inspected.
It must not relabel a set of project-controlled keys, hosts, CI jobs, or a
Product Owner walkthrough as multiparty public control.

## Current contract

ADR-0004 fixes distinct Control Plane roots, a `3-of-5` public baseline for
Network Epoch and executable authorization, a `4-of-5` expiring emergency,
and two full Candidate View auditors independent of each other, the Epoch
threshold, and audited Candidate operators. The product contract requires five
real independent custodians and reproducible packages with two matching
independent builder attestations. H4-6B deliberately selected neither the
custody organization nor a public Candidate.

The active project team is one Product Owner and Codex. It has no real external
custodians, builders, or auditors. Existing project VPS instances, local
Docker, CI, and local development keys are project-controlled experiment
resources and cannot fill any of those roles.

## Hypotheses

- **H1:** A public candidate can be independently inspected when a bounded,
  content-addressed evidence package binds separately operated custody, full
  Candidate View reconstruction, independently reproduced packages, and
  independent audit findings to one exact candidate.
- **H2:** Multiple keys, rebuilds, or readers operated by the project are
  sufficient evidence of public control.
- **H0:** No available custody/evidence arrangement satisfies the product
  contract, so no public candidate should be selected.

## Evaluation criteria

Before selecting a candidate, its evidence must prove all of the following:

The user outcome is a public participant or auditor who can obtain one exact
candidate bundle and independently determine whether it is safe to inspect or
must stop. The protected information is custody private material and
participant-specific Candidate selections; the adversary includes a malicious
project, signer, distributor, builder, or auditor that tries to forge, omit,
replay, or silently downgrade evidence. The selected candidate must publish its
measured byte/count/storage/CPU/time limits and availability/source budget as
part of the same bundle; no generic latency or capacity number is assumed here.

1. exactly five disclosed custody members operate independently: no shared
   operator, hosting provider, or administrative organization is counted
   twice; each supplies a durable public identity, authority public key, and
   independently reachable custody-operation record;
2. ordinary Epoch and new-executable authorization require `3-of-5`; an
   emergency is `4-of-5`, automatically expires, and can only disable/stop
   unsafe work — never seize a Name, alter a live destination, choose a Route,
   decrypt traffic, or install a package;
3. every successor/rotation record names the exact predecessor and is accepted
   only by both predecessor and successor quorum; loss, compromise, removal,
   replacement, and emergency recovery have public, finite, fail-closed
   procedures;
4. the full, canonically ordered Candidate View input log, cutoff, deterministic
   acceptance/rejection rules, View commitment, global summaries, and
   materialization rule revision are available as exact content-addressed
   artifacts. An Endpoint verifies only a selected indexed proof; two full
   auditors independently reconstruct the complete View and publish their
   inputs, software revision, output digest, and any disagreement;
5. every candidate package binds source revision, resolved dependencies, build
   recipe, SBOM, qualification identity where claimed, artifact digest, and two
   matching independently operated builder attestations. A builder is not a
   Release Targets custodian; reproducibility is checked from retained inputs,
   not asserted by a CI label; and
6. the public reader verifies artifact identities, signatures/quorums where the
   selected format permits, predecessor and expiry/floor rules, declared role
   collisions, and the complete transition/error matrix. It reports missing,
   withheld, conflicting, stale, replayed, forged, revoked, and unavailable
   evidence without fallback. It labels factual operator independence as an
   external-evidence conclusion, never as something a self-authored manifest
   can prove.

The package also needs finite byte/count/resource limits selected for the
candidate's stated capacity. H4-6C must not reuse Functional Alpha's small
private Epoch limits as a public-network limit without a new capacity decision.

## Evidence plan

### Primary sources

- [The Update Framework specification, threshold and root-rotation rules](https://theupdateframework.github.io/specification/v1.0.28/), accessed 2026-08-29.
- [SLSA Build Provenance v1.0](https://slsa.dev/spec/v1.0/provenance), accessed 2026-08-29.
- [Sigstore Bundle Format](https://docs.sigstore.dev/about/bundle/), accessed 2026-08-29.
- ADR-0004, the product operating model, functional map NET-07L/NET-15/NET-18,
  and the threat model, accessed 2026-08-29.

### Experiment

Do not manufacture a qualifying public-candidate experiment. The retained
`simulate-public-control` mechanics simulation uses freshly generated,
in-memory project-controlled identities to exercise threshold, rotation, full
Candidate View reconstruction, package-attestation, and intentional-failure
paths. It has no independent-operator result and is labelled `qualified:
false`. Once real
participants exist, each independent custodian, builder, and auditor produces
their evidence from their own administration and build environment; the public
reader is run from at least two non-project auditor environments against the
same content-addressed package. A disagreement, missing input, withheld object,
or failed reproduction is retained in the candidate's evidence denominator and
blocks promotion until an explicit successor candidate is selected.

The repository may test only the deterministic reader and malformed/equivocal
evidence handling. Those tests prove neither independent operation nor public
control.

### Failure scenarios

- One person or organization controls several nominal custodians, builders,
  auditors, or their hosting/administration.
- A custodian is lost, compromised, coerced, unavailable, or signs a conflicting
  successor or emergency action.
- A signer omits, accepts, or rejects Candidate inputs inconsistently; a
  distributor withholds a materialization; auditors disagree.
- A package/build attestation is forged, source/dependency inputs change,
  reproduction differs, or an emergency is used as an install/downgrade path.
- A stale/replayed/forked roster, Epoch, package, receipt, or rotation is
  offered after expiry or a retained floor.

## Findings

- **Sourced fact:** TUF threshold verification counts each key at most once and
  rotates a root through both predecessor and successor thresholds. It also
  treats expiry as the boundary on a withholding/freeze attack.
- **Sourced fact:** SLSA provenance identifies the build definition, resolved
  dependencies, builder identity, and output subjects; its builder identity
  describes a trust boundary rather than proving independent human operation.
- **Sourced fact:** a Sigstore bundle can carry verification material and a
  signature/attestation, but its presence alone does not establish that two
  nominal signers are independently operated.
- **Inference:** cryptographic signatures and content addresses can make
  statements and artifacts inspectable, but independence remains a claim about
  real people, organizations, administration, and infrastructure that needs
  external corroboration.
- **Measurement:** as of 2026-08-29 the repository contains Functional Alpha
  H4-6A/B evidence only. No real independent H4-6C custodian, builder, or
  auditor evidence has been supplied.
- **Measurement:** the H4-6C mechanics simulation covers `3-of-5` routine,
  `4-of-5` emergency, bidirectional rotation, two full deterministic Candidate
  View reconstructions, and two matching builder attestations, lifecycle and
  reader-matrix rejection cells. All identities are local ephemeral simulation
  identities.

## Options

1. Select five project-controlled keys and label them public control — rejected:
   contradicts the product/threat-model independence gates.
2. Reuse the Alpha reader and its local/VPS/Docker outputs as a public evidence
   reader — rejected: it has alpha scope and cannot prove global completeness,
   public capacity, or independent operation.
3. Specify a bounded public evidence contract and reader seam, then select a
   candidate only after real independent parties supply evidence — retained for
   implementation and external execution.
4. Select no public candidate now — current outcome until option 3 has real
   evidence.

## Recommendation

Choose option 4 now, while implementing only option 3's non-authorizing reader
and documentation seam. Confidence is high: the current one-to-one project
cannot independently furnish the required people or organizations. The
strongest limitation is that this preserves the Public Beta block; it does not
create public control on its own.

## Disposition

R-124 remains active. The reader and mechanics simulation are retained as
reproducible non-authorizing evidence work, but no custody selection,
public-control ADR, or Public Beta claim follows from them. Selection requires
the six criteria above to be independently evidenced by actual participants.
