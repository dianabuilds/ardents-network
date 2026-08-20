# S6.0 preparation summary

Status: **not ready. R-041, R-043, R-046, and R-047 are decided; R-042, R-044,
R-045, and general evidence serialization remain open. ADR-0013 is withdrawn
and ADR-0014 is accepted.**

This document is a status pointer, not a decision record. Full decisions and
evidence belong in R-041 through R-047; delivery documents link them instead of
copying their values.

## Gate

| # | Condition | Status |
|---|---|---|
| 1 | Stage 5 maintained development advanced | **Satisfied 2026-08-19.** Deferred R-037 qualification remains S9.6 and is not a Stage 6 predecessor. |
| 2 | Corrected Stage 6 documents accepted by Product Owner | **Open.** Validation corrections are in progress. |
| 3 | R-003 remains authoritative | **Satisfied.** Reopened records now preserve its front-running, Recovery Authority, and measurement requirements. |
| 4 | Required S6.0 decisions accepted | **Open.** R-042, R-044, and R-045 remain unresolved; R-046, R-047, and ADR-0014 are accepted. |
| 5 | Package and command ownership is factual | **Partial.** Foundation packages are feasibility work, not Stage 6 evidence; no verifier package is registered. |
| 6 | Product Owner records coding start | **Open.** May occur only after every readiness item in A-D passes. |

## S6.0 research status

| ID | Boundary | Blocks | Status | Next evidence |
|---|---|---|---|---|
| R-041 | Canonical textual and wire name profile | S6.1 | **decided** | Maintained round-trip/rejection vectors after coding is authorized |
| R-042 | Claim ordering and authenticated inclusion | S6.5, S6.6 | **open** | Claim-ordering simulator and eight-scenario proof map |
| R-043 | Durable/restart-derived/cache-bounded state | S6.4 | **decided** | Naming-owned interface plus `internal/network/store` adapter conformance test |
| R-044 | Threshold Recovery Authority mechanism | S6.4, S6.6 | **open** | Source review, threshold recovery experiment, recovery ADR |
| R-045 | Anonymous Cost and admission | S6.5, S6.6 | **open** | Predeclared benchmark on R-023 host and weaker client |
| R-046 | Field-level role matrix | S6.2, S6.6 | **decided** | Maintained forbidden-field mutation tracer |
| R-047 | S6.2 Authority/Record authentication and query hiding | S6.2, S6.6 | **decided** | Maintained authenticated OHTTP role tracer |

## ADR status

- [ADR-0013](../adr/0013-stage-6-cryptographic-suite.md) is **withdrawn**. It
  authorizes no primitive, package, interface, or implementation.
- Accepted [ADR-0014](../adr/0014-private-naming-ohttp.md) is limited to S6.2
  Ed25519 authentication and OHTTP query hiding.
- S6.4 threshold recovery still requires R-044 findings and a separate ADR.

## Achievable next state

1. Implement the accepted R-046/R-047 S6.2 resolution tracer and adversarial
   role-view tests without selecting later control-operation proof mechanisms.
2. Research and measure R-044 and R-045 without implementing their production paths.
3. Run R-042's ordering experiment and freeze the eligible-set proof.
4. Freeze the Stage 6 manifest/evidence/verdict serialization and profile after
   those decisions exist.
5. Revalidate this brief, plan, checklist, and evidence contract; then the
   Product Owner may record coding start.

No implementation slice may convert an open item into an implicit default.
Existing `internal/naming` and `internal/namelease` work remains feasibility code
and cannot be cited as Stage 6 progress or evidence while the gate is closed.
