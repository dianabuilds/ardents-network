---
id: R-067
title: Which existing naming bytes are preserved during Stage 8 Namespace refactoring?
status: accepted
owner: Product Owner and Codex
started: 2026-08-23
reviewed: 2026-08-23
---

# R-067 — Naming profile retention

## Decision this unlocks

DA-07 must distinguish accepted Name security/profile facts from the current
tracer's accidental persistence, claim, and command encodings. Without that
split, M5 would either treat every current byte as permanent or mutate a
signed/profiled boundary without authority.

## Current contract

R-041 fixes canonical Name V1. R-044 fixes the bounded Recovery Policy and
its signed proof inputs. R-047/ADR-0014 fix Name Record signatures and the
private OHTTP profile. R-057/ADR-0020 fix the threshold-attested current
Namespace statement and compact proof contract. R-060 assigns all Namespace
durability/commitment representations to its owner. G2 F023, F026, F029, and
F031 show that current claim, receipt, durable-chunk, and command encodings
are not proof that their complete implementation behavior is authoritative.

## Hypotheses

- **H1:** retain only the accepted canonical/signed/private-resolution profile;
  treat current internal persistence, claim-tracer, receipt, and command bytes
  as C0 implementation residue with no named external observer.
- **H2:** retain all current naming bytes indefinitely because Stage 6 produced
  them.
- **H0:** replace accepted Name, Record, Recovery, OHTTP, or materialization
  bytes without a migration/authority record.

## Evaluation criteria

The decision must preserve R-041/R-044/R-047/R-057 security properties,
identify an actual observer before retaining an adapter, make no unsupported
wire migration, and leave F023/F026/F029/F031 visible for M5 design rather than
declaring them solved by retention.

## Evidence plan

### Primary sources

- R-041, R-044, R-047, R-057, ADR-0014, ADR-0018, ADR-0020, and R-060,
  accessed 2026-08-23.
- Stage 8 G2 findings F023, F026, F029, and F031, inspected 2026-08-23.
- Target-architecture W03/D04 and M5 disposition, inspected 2026-08-23.

### Experiment

No implementation experiment is needed: this is an observer and authority
inventory. M5 must provide byte-level characterization for each retained
profile and a deletion assertion for each C0 tracer encoding.

### Failure scenarios

- treating unsigned receipts as authenticated Namespace state;
- retaining a claim proof that contradicts its global-close contract;
- changing a signed Record or recovery transcript while calling it refactoring;
- adding a permanent legacy decoder without a named observer; and
- losing fixed-size private-resolution or threshold-materialization checks.

## Findings

- **Sourced fact:** the accepted records above name canonical records,
  signatures, recovery proof semantics, fixed private resolution, and
  threshold materialization; none names the present command result, receipt,
  durable chunk, or local claim-tracer encoding as a public observer contract.
- **Inspection:** the Stage 8 inventory names no external caller for
  `cmd/ardents-name` or the internal `nameclaim`/`namestore` wire containers.
- **Inference:** retaining their behavior by default would preserve known
  defects (notably F023/F026/F029/F031) as accidental compatibility.

## Options

| Option | Fit | Disposition |
|---|---|---|
| H1: profile-only retention | Preserves selected trust/privacy facts while allowing obsolete tracer encodings to disappear in one owner-led cutover. | Accepted. |
| H2: retain every current byte | Creates unbounded legacy support and treats Stage 6 evidence as a permanent public protocol. | Rejected. |
| H0: change selected profile bytes now | Requires a separate format/protocol authority and migration evidence. | Rejected. |

## Recommendation

M5 must retain these authority-bound profile facts until separately superseded:
Name V1 canonical bytes; Record and Recovery signed transcript semantics;
fixed 4,096-byte OHTTP envelopes; and threshold-authenticated current
materialization/proof semantics. Internal claim, receipt, persistence-chunk,
and command-tracer bytes have no named observer and are C0 deletion targets.
F023/F026/F029/F031 remain design/failure obligations, not a license to alter
the retained profile without an explicit subsequent record.

## Disposition

**Accepted H1 on 2026-08-23 under the Product Owner's standing Stage 8
authority.** DA-07 is closed for M5 profile retention only. A future change to
any listed signed, private-resolution, or materialization profile reopens
DA-07 and requires the applicable format/protocol authority.
