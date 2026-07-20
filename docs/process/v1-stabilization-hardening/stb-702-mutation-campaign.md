# STB-702 Mutation Campaign

Status: predeclared; execution pending.

## Execution Contract

- Run only one mutation at a time against the committed `69bc86a` baseline.
- Execute the named targeted Go test in `golang:1.26-bookworm`, never on Windows.
- Bound each `go test` to 45 seconds and the container command to 90 seconds.
- Treat the mutant as killed only when the declared test fails for the expected
  product assertion, not for compilation, infrastructure, or timeout.
- Reverse the mutation immediately and verify the target file SHA-256 before the
  next experiment.
- A surviving mutant must receive a canonical regression test or a formal
  decision before STB-702 can complete.

## Predeclared Experiments

| ID | Critical invariant | Exact reversible mutation | Targeted detector and expected signal | Blast radius / rollback |
| --- | --- | --- | --- | --- |
| MUT-01 | Private carrier selectors must remain opaque and fixed-width. | In `internal/network/privacy/carrier_validation.go`, disable the `len(token) != tokenSize` rejection with `false &&`. | `TestCarrierValidationDetectsReadableSelectorMutation` must fail because the readable short selector is accepted. | Selector validation only; reverse the condition and verify file hash. |
| MUT-02 | Policy must deny untrusted routes when configured to do so. | In `internal/policy/evaluation/routes.go`, disable the `DisableUntrustedRouteUse && !candidate.Trusted` guard with `false &&`. | `TestCheckRouteUse` must fail on an untrusted-route assertion. If it remains green, add that missing canonical assertion and repeat. | Route policy evaluation only; reverse the condition and verify file hash. |
| MUT-03 | Publication requires both runtime readiness and exposure eligibility. | In `internal/publication/plan.go`, change the readiness guard from `!Ready || !ExposureEligible` to `!Ready && !ExposureEligible`. | `TestPublicationPlanSeparatesAllowedAndDeniedServices` must fail because `svc.not-exposure-eligible` is published. | Publication planning only; restore `||` and verify file hash. |
| MUT-04 | Workload policy references fail closed. | In `internal/workload/execution/docker_executor.go`, invert the admitted-set decision from `!allowed` to `allowed`. | `TestDockerExecutorTrustAndProvenanceAdmissionFailClosed` must fail because admitted and denied policy references are reversed. | Pure admission path; restore `!allowed` and verify file hash. |
| MUT-05 | Service readiness requires the configured consecutive-success threshold. | In `internal/hosting/readiness/controller.go`, change `successes >= SuccessThreshold` to `successes > SuccessThreshold`. | `TestControllerRejectsWrongGenerationAndRequiresConsecutiveRecovery` must fail because the second valid observation remains warming. | In-memory readiness state only; restore `>=` and verify file hash. |
| MUT-06 | Replica commits must bind ciphertext-derived identity to reservation and blob metadata. | In `internal/data/placement/commit.go`, return `nil` instead of the content-identity error inside the mismatch branch. | `TestReceiverRejectsWrongCIDPartialCommitExpiryAndReplay` must fail because the wrong-CID request no longer reports content-identity rejection. | Validation before durable storage; restore the error return and verify file hash. |
| MUT-07 | Diagnostics recursively redact sensitive keys. | In `internal/diagnostics/event/redaction.go`, disable the `isSensitiveKey` branch with `false &&`. | `TestMapRedactsSensitiveFields` must fail on the first unredacted secret/key/token assertion. | Map projection only; reverse the condition and verify file hash. |
| MUT-08 | Degraded subsystem health must produce a degraded boot lifecycle. | In `internal/node/recovery/boot_finalize.go`, map `HealthDegraded` to `ready` instead of `degraded`. | `TestCompleteBootUsesDegradedLifecycleWhenHealthIsDegraded` must fail on lifecycle state. | Boot finalization only; restore `degraded` and verify file hash. |

## Result Ledger

| ID | Baseline | Mutant signal | Result | Rollback hash | Follow-up |
| --- | --- | --- | --- | --- | --- |
| MUT-01 | pending | pending | pending | pending | pending |
| MUT-02 | pending | pending | pending | pending | pending |
| MUT-03 | pending | pending | pending | pending | pending |
| MUT-04 | pending | pending | pending | pending | pending |
| MUT-05 | pending | pending | pending | pending | pending |
| MUT-06 | pending | pending | pending | pending | pending |
| MUT-07 | pending | pending | pending | pending | pending |
| MUT-08 | pending | pending | pending | pending | pending |
