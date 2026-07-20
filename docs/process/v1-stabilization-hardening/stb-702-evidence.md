# STB-702 Evidence

Date: 2026-07-20  
Decision: accepted

## Capability

The Phase 7 QA campaign proves that targeted tests detect deliberate breakage
of eight critical invariants: private selector shape, route trust enforcement,
runtime-backed publication, workload policy admission, hosted-service
readiness, replica content identity, diagnostics redaction, and recovery
lifecycle truth.

The predeclared mutations, expected signals, exact rollback hashes, and result
ledger are retained in `stb-702-mutation-campaign.md`.

## Results

- Eight of eight mutants are killed by canonical targeted tests.
- MUT-01 initially survived because one fixture violated both selector length
  and alphabet rules. `TestCarrierValidationRejectsWrongLengthOpaqueSelector`
  now isolates the fixed-width invariant and kills the mutant.
- MUT-02 initially survived because route tests did not exercise an untrusted
  candidate with `DisableUntrustedRouteUse`. `TestCheckRouteUse` now contains
  that fail-closed assertion and kills the mutant.
- MUT-03 through MUT-08 were killed by existing tests with the predeclared
  product-level signals.
- Every temporary product mutation was reversed immediately. All eight target
  files matched their pre-experiment SHA-256 after rollback, and no mutated
  product diff remains.

## Docker Validation

All Go commands ran in `golang:1.26-bookworm` with CPU/memory bounds and both a
45-second Go timeout and a 90-second orchestration timeout.

The final restored-code detector run covered:

- `internal/network/privacy`
- `internal/policy/evaluation`
- `internal/publication`
- `internal/workload/execution`
- `internal/hosting/readiness`
- `internal/data/placement`
- `internal/diagnostics/event`
- `internal/node/recovery`

Result: eight package results passed, zero failures, 6.1 seconds wall time.

The code-size guard checked the two affected package trees with tests enabled.
The changed files had no soft or hard breach. It reported one unrelated,
pre-existing soft warning for `internal/network/privacy/envelope_test.go` at
345 LOC; this is below the 450 LOC hard limit and was not modified by STB-702.

## Acceptance

- Failure/degraded paths are the subject of every experiment.
- No runtime behavior, dependency, domain boundary, diagnostics shape, or Waku
  foundation behavior changed.
- No fake or deferred critical behavior was introduced.
- The two surviving mutants received canonical regression closure and were
  killed on repeat.
- No critical invariant mutation remains without follow-up closure.

STB-702 is accepted.
