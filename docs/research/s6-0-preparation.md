# S6.0 preparation summary

Status: **complete. R-041 through R-047, R-055, and R-057 are decided; the
Product Owner accepted ADR-0014 and ADR-0017 through ADR-0020 and authorized
S6.3-S6.6 on 2026-08-20. Maintained implementation and mutation coverage are
complete; the bounded S6E1 command campaign and final disposition remain.**

This document is a status pointer, not a decision record. Full decisions and
evidence belong in R-041 through R-047; delivery documents link them instead of
copying their values.

## Gate

| # | Condition | Status |
|---|---|---|
| 1 | Stage 5 maintained development advanced | **Satisfied 2026-08-19.** Deferred R-037 qualification remains S9.6 and is not a Stage 6 predecessor. |
| 2 | Corrected Stage 6 documents accepted by Product Owner | **Satisfied 2026-08-20.** |
| 3 | R-003 remains authoritative | **Satisfied.** Reopened records now preserve its front-running, Recovery Authority, and measurement requirements. |
| 4 | Required S6.0 decisions accepted | **Satisfied 2026-08-20.** R-042, R-044, R-045, R-046, R-047, and R-055 are accepted. |
| 5 | Package and command ownership is factual | **Satisfied.** Maintained naming, evidence, verifier, and command boundaries are registered. |
| 6 | Product Owner records coding start | **Satisfied 2026-08-20.** S6.3-S6.6 were explicitly authorized. |

## S6.0 research status

| ID | Boundary | Blocks | Status | Next evidence |
|---|---|---|---|---|
| R-041 | Canonical textual and wire name profile | S6.1 | **decided** | Maintained round-trip/rejection vectors pass |
| R-042 | Claim ordering and authenticated inclusion | S6.5, S6.6 | **decided** | Complete input/rejection corpora and independent recomputation implemented |
| R-043 | Durable/restart-derived/cache-bounded state | S6.4 | **decided** | Naming-owned store and `internal/network/store` adapter conformance tests pass |
| R-044 | Threshold Recovery Authority mechanism | S6.4, S6.6 | **decided** | Standard-library Ed25519 threshold policy and mutation vectors implemented |
| R-045 | Anonymous Cost and admission | S6.5, S6.6 | **decided** | O1b profile measured and C7 pressure evidence implemented |
| R-046 | Field-level role matrix | S6.2, S6.6 | **decided** | Maintained forbidden-field mutation tracer implemented |
| R-047 | S6.2 Authority/Record authentication and query hiding | S6.2, S6.6 | **decided** | Maintained authenticated OHTTP role tracer implemented |
| R-055 | Canonical S6E1 evidence serialization | S6.6 | **decided** | Launcher/worker/verifier split and exhaustive mutation matrix implemented |
| R-057 | Current Namespace authentication | S6.3, S6.6 | **decided** | Threshold statement, compact proof, full corpus, and maximum-depth measurement implemented |

## ADR status

- [ADR-0013](../adr/0013-stage-6-cryptographic-suite.md) is **withdrawn**. It
  authorizes no primitive, package, interface, or implementation.
- Accepted [ADR-0014](../adr/0014-private-naming-ohttp.md) is limited to S6.2
  Ed25519 authentication and OHTTP query hiding.
- Accepted ADR-0017 through ADR-0020 own claim ordering, recovery signatures,
  Anonymous Cost, and current Namespace materialization respectively.

## Remaining completion step

Build the launcher and independently built verifier from the exact candidate,
run one bounded S6E1 A0-D6 campaign outside Git, retain its `pass` verdict, pass
the repository gates and review, then request the final Product Owner
disposition. This is development completion, not Qualification or a public
privacy claim.
