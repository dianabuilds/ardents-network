---
status: accepted
date: 2026-08-23
supersedes: none
---

# ADR-0029 — Retire Update V0 custody evidence by owned root migration

## Context

R-064 permits one bounded H3 Update technical tracer but expires its C2
command in M13. The V1 Update manifest embeds evidence text rendered as
`custody_notice`; its digest is committed into durable selection and recovery
records, so it cannot be removed in place.

## Decision

Use R-087's one-shot owned C1 conversion from a valid V1 Update root to a V2
root whose manifests and public results omit EvidenceNotice. Conversion must
select only a complete V2 root atomically and fail closed for malformed or
interrupted V1 inputs. V1 is migration input only, never an active runtime
format after conversion. Exact V0 vectors survive only in a C4 independent
provenance verifier. Delete the C2 `ardents-release` command in the same M13
cutover.

## Consequences

- Update retains its bounded Release authorization, transaction, rollback, and
  recovery responsibilities without a Custody or installer claim;
- no maintained runtime code imports or accepts a V0 provenance verifier; and
- new conversion/restart/non-resurrection vectors are required before the V1
  writer and command are deleted.

## Compliance

This ADR records alternatives, failure cases, and required evidence. It selects
no platform activation, storage engine, or cryptographic primitive.
