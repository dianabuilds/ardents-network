---
id: R-039
title: What is the minimal Stage 6 production boundary for Service Name lifecycle in H3?
status: accepted
owner: product research
started: 2026-08-17
reviewed: 2026-08-17
---

# R-039 - Stage 6 private naming lifecycle

## Decision status

Status: **accepted by the Product Owner as the bounded Stage 6 scope. This record
does not open the coding gate or select implementation mechanisms.**

Stage 6 implementation remains blocked until Stage 5 advances, the corrected
Stage 6 documents are accepted, and the S6.0 decision profile is frozen.
Implementation evidence for every mandatory cell is required before Stage 6 can
be considered complete.

## Decision this unlocks

Prepare a bounded production naming lifecycle based on accepted R-003 semantics.
The scope includes exact-name encoding, lifecycle state, authority and delegation,
Target continuity, private resolution and control-operation role separation,
conflict/fork behavior, bounded abuse surfaces, and independent evidence.

This decision authorizes design preparation only. It does not authorize code to
choose persistence, storage engine, consensus, public wire protocol,
cryptography, claim ordering, Anonymous Cost, or governance mechanisms.

## Fixed product contract

- A Service Name is one exact canonical name, not a list/search/directory entry,
  identity, authorization, or secrecy capability.
- Name Authority is separate from Service Authority and Service Target Authority.
- Lease follows Active, finite Grace, and Released; reclaim creates a new Name
  Generation and revisions are monotonic within a generation.
- Consistency and recovery are separate from Lease state. Conflict/Fork stops
  resolution but cannot release a valid Lease.
- Delegation creates bounded subordinate authority and is not authority transfer.
- Rotation and transfer install one successor inside the same generation.
- Recovery exists only under a precommitted generation-bound policy; policy
  mutation is delayed and visible; Recovery Pending is bounded and fail-closed.
- Exact-name resolution and naming control operations separate endpoint location
  from the exact name/control view for any one ordinary Node under the accepted
  conditions.
- Direct Target connections remain pinned; name-origin connections never silently
  retarget.
- No DNS, HTTP, search, recommendation, alias, alternate Namespace, registrar,
  administrator, or manual fallback is introduced.

## Required pre-implementation decisions

S6.0 must freeze:

1. exact canonical-name label, length, depth, link, and schema-version limits;
2. authenticated claim ordering and proofs distinguishing ordered collision from
   unresolved Conflict, partition, and rule Fork;
3. persistence, restart, rollback, cache freshness, and state ownership;
4. cryptographic/key-management mechanisms or explicit replaceable interfaces,
   with an ADR where selection creates lock-in;
5. R-010-compatible Anonymous Cost and local admission limits; and
6. field-level resolution and control-operation role views, Role Domains,
   known-family exclusions, Isolation Context behavior, and bounded clocks.

Every selection follows repository research discipline. No implementation slice
may treat an open mechanism as an implicit default.

## Hypotheses

- **H1:** accepted R-003 semantics can be implemented through bounded production
  naming modules and a separately owned verifier after S6.0 decisions are frozen.
- **H2:** the Gate C private-resolution seed cannot satisfy production role and
  evidence boundaries without a separately selected mechanism or seam.
- **H0:** the required ordering, privacy, persistence, recovery, accessibility, or
  abuse bounds cannot be implemented without weakening the H3 contract.

## Evaluation criteria

1. exact canonical encoding and finite limits;
2. exact Lease, generation, revision, parent/child, cache, and recovery behavior;
3. Conflict/Fork visibility without forced release or local branch selection;
4. rotation, transfer, delegation, policy mutation, and recovery without stale or
   administrative authority;
5. same-Target migration, replacement Target, name-origin close, and direct-Target
   pinning semantics;
6. strict role separation for resolution and naming control operations;
7. bounded Anonymous Cost/resource handling without identity or fairness claims;
8. no public or less-private fallback; and
9. independent `pass|fail|invalid` verification over immutable artifacts.

## Evidence plan

Primary sources are R-003, R-024/R-010, R-026, the H3 technical design, threat
model, corrected Stage 6 brief, and Stage 6 evidence contract.

The mandatory A0-D6 matrix covers canonical parsing, lifecycle, parent delegation,
rotation/transfer, Recovery Policy mutation, recovery, Target continuity,
connection behavior, concurrent ordering, conflict/fork, front-running,
withholding, pressure, resolution/control privacy, cross-context state, fallback,
cache rollback, and rule forks.

Each cell uses one immutable manifest, ordered evidence, bounded clocks, complete
cleanup, and a separately built verifier. Runner and commands cannot author a
verdict.

## Failure scenarios

- conflict or fork forces Release or silently selects a controller;
- concurrent claims create two accepted controllers;
- stale generation, revision, descendant, policy, or cache evidence is replayed;
- delegation escapes its subtree or survives parent Release/reclaim;
- recovery policy is erased, bypassed, cancelled, or extended by current authority;
- resolution resumes after recovery without a fresh successor record;
- a live name-origin stream silently retargets or a direct Target follows naming;
- one ordinary role observes both endpoint location and exact name/control view;
- query state links Isolation Contexts;
- front-running, withholding, flooding, rollback, or rule forks resolve by local
  arrival or best effort; or
- a public/less-private fallback or administrative recovery path appears.

## Options and recommendation

1. **Reuse Gate C semantics only as a replaceable seed after S6.0 review.**
   Accepted. Production roles, state, evidence, and privacy claims remain separate.
2. **Treat the Gate C lab implementation as production naming.** Rejected because
   it bypasses production state, command, role, and evidence decisions.
3. **Choose open mechanisms while coding.** Rejected because it violates research
   discipline and makes the verifier depend on accidental implementation choices.
4. **Defer all naming work.** Rejected while Stage 6 remains an accepted H3 outcome;
   H0 may still stop/redesign the slice if S6.0 gates cannot be satisfied.

Recommendation: retain option 1 and complete S6.0 before production code resumes.

## Disposition

- R-039 remains accepted as the Stage 6 scope decision.
- Corrected brief, plan, checklist, and evidence contract require Product Owner
  acceptance before coding entry.
- Stage 5 S5.4/S5.5 `advance` remains a separate prerequisite.
- No ADR is created by this scope record. Any S6.0 technology selection that
  creates meaningful lock-in requires its own research record and accepted ADR.
- No experiment folder is required for this document correction; mechanism
  experiments follow their own decision-relevant questions and evidence plans.
