---
id: R-080
title: What Stage 5 material remains after native Entry supersedes Bridge and WebTunnel?
status: accepted
owner: Product Owner and Codex
started: 2026-08-23
reviewed: 2026-08-23
---

# R-080 — Stage 5 historical provenance

## Decision this unlocks

Close the Stage-5 portion of DA-11 so M7 can remove the retired Bridge and
WebTunnel implementation without presenting its development harness as a
maintained Route or a current qualification obligation.

## Current contract

R-076/ADR-0024 select `ardents-interactive-route-v1` and retire H3
Bridge/WebTunnel bytes as C0. R-077 through R-079 supply the replacement Entry
contract. The source tree still has Stage-5 laboratory records and independent
verifier code, an active historical-reproduction profile, and the retired
`internal/bridge` and `internal/camouflage` packages. No accepted current
product claim, release gate, external observer, or peer compatibility promise
depends on executing the retired packages.

## Hypotheses

- **H1:** retain immutable Stage-5 records and their independent evidence
  readers as C4 provenance, while deleting the retired runtime packages and
  active live runners that require them.
- **H2:** retain Bridge/WebTunnel as a compatibility or reproduction runtime.
- **H0:** delete every Stage-5 record and verifier immediately.

## Evaluation criteria

The disposition must not restore H3 wire bytes, WebTunnel, a migration reader,
or a product Route dependency. Historical evidence must remain attributable to
its recorded source/profile rather than be misrepresented as v1 evidence. The
maintained module must build and test without retired runtime packages.

## Evidence plan

### Primary sources

- R-076, ADR-0024, R-077, R-078, R-079, and ADR-0027, inspected 2026-08-23.
- `docs/development/stage-8-compatibility-observer-inventory.md`,
  `tests/profiles/profiles.json`, and the package/import inventory, inspected
  2026-08-23.

### Experiment

After removal, `make quick-check` must pass and a repository import scan must
find no maintained Go caller of `internal/bridge` or `internal/camouflage`.

### Failure scenarios

- A historical artifact is accidentally described as a v1 qualification result.
- A live runner compiles against a removed Bridge/WebTunnel package.
- A retained evidence reader regains a product-runtime dependency.

## Findings

- **Inspection:** `internal/bridge` and `internal/camouflage` have no
  non-test maintained caller after the route command's entry-plan removal.
- **Inspection:** the only direct imports outside their own tests are
  Stage-5-tagged live runner tests; their Docker inputs build the same retired
  packages.
- **Inspection:** the compatibility inventory identifies laboratory formats as
  historical material, not a product compatibility surface.
- **Inference:** retaining retired execution paths would contradict the C0
  retirement selected by ADR-0024; retaining source-bound records and an
  independent verifier preserves provenance without making it runtime scope.

## Options

| Option | Disposition |
|---|---|
| C4 records/evidence readers; delete retired runtime and runners | Choose. Keeps auditable provenance without a maintained H3 path. |
| Keep Bridge/WebTunnel executable for reproduction | Reject. It contradicts ADR-0024 and has no named current observer. |
| Delete every historical record now | Defer. M14 audits the remaining laboratory record set as one retirement wave. |

## Recommendation

Choose H1 with high confidence. The strongest objection is that an old
campaign cannot be rerun from the current runtime tree; that is intentional:
its captured evidence is historical provenance, not evidence for the native
profile. A future claim needs fresh v1 Qualification evidence.

## Disposition

**Accepted 2026-08-23 under the Product Owner's standing Stage 8 delegation.**
DA-11 is closed for the M7 Stage-5 runtime subset. Remove
`internal/bridge`, `internal/camouflage`, the active live runners, and the
`blocked-entry-lab` evidence generator; keep the records and the independent
`blocked-entry-verify-lab` evidence reader until M14's whole-record audit. No
ADR is needed because this selects neither technology nor a new
protocol/format.
