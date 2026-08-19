# S6.0 preparation summary

Status: **not ready. One syntax freeze and one persistence-boundary decision are
decided; four S6.0 research gates are open. ADR-0013 is withdrawn.**

This document is a status pointer, not a decision record. Full decisions and
evidence belong in R-041 through R-046; delivery documents link them instead of
copying their values.

## Gate

| # | Condition | Status |
|---|---|---|
| 1 | Stage 5 maintained development advanced | **Satisfied 2026-08-19.** Deferred R-037 qualification remains S9.6 and is not a Stage 6 predecessor. |
| 2 | Corrected Stage 6 documents accepted by Product Owner | **Open.** Validation corrections are in progress. |
| 3 | R-003 remains authoritative | **Satisfied.** Reopened records now preserve its front-running, Recovery Authority, and measurement requirements. |
| 4 | Required S6.0 decisions accepted | **Open.** R-042, R-044, R-045, and R-046 are unresolved. |
| 5 | Package and command ownership is factual | **Partial.** Foundation packages are feasibility work, not Stage 6 evidence; no verifier package is registered. |
| 6 | Product Owner records coding start | **Open.** May occur only after every readiness item in A-D passes. |

## S6.0 research status

| ID | Boundary | Blocks | Status | Next evidence |
|---|---|---|---|---|
| R-041 | Canonical textual and wire name profile | S6.1 | **decided** | Maintained round-trip/rejection vectors after coding is authorized |
| R-042 | Claim ordering and authenticated inclusion | S6.5, S6.6 | **open** | Claim-ordering simulator and eight-scenario proof map |
| R-043 | Durable/restart-derived/cache-bounded state | S6.4 | **decided** | Naming-owned interface plus `internal/network/store` adapter conformance test |
| R-044 | Cryptographic suite and recovery trust | S6.2, S6.4, S6.6 | **open** | Source review, threshold recovery experiment, replacement ADR |
| R-045 | Anonymous Cost and admission | S6.5, S6.6 | **open** | Predeclared benchmark on R-023 host and weaker client |
| R-046 | Field-level role matrix | S6.2, S6.6 | **open** | Complete operation/field table and forbidden-field mutation tracer |

## ADR status

- [ADR-0013](../adr/0013-stage-6-cryptographic-suite.md) is **withdrawn**. It
  authorizes no primitive, package, interface, or implementation.
- A replacement cryptographic decision requires R-044 findings and a new ADR.

## Achievable next state

1. Complete R-046's exact matrix; it defines what R-044 query hiding and S6.6
   evidence must protect.
2. Research and measure R-044 and R-045 without implementing production paths.
3. Run R-042's ordering experiment and freeze the eligible-set proof.
4. Freeze the Stage 6 manifest/evidence/verdict serialization and profile after
   those decisions exist.
5. Revalidate this brief, plan, checklist, and evidence contract; then the
   Product Owner may record coding start.

No implementation slice may convert an open item into an implicit default.
Existing `internal/naming` and `internal/namelease` work remains feasibility code
and cannot be cited as Stage 6 progress or evidence while the gate is closed.
