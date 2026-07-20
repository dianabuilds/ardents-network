# STB-702 Mutation Campaign

Status: completed on 2026-07-20.

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
| MUT-01 | Private carrier selectors must remain opaque and fixed-width. | In `internal/network/privacy/carrier_validation.go`, disable the `len(token) != tokenSize` rejection with `false &&`. | `TestCarrierValidationRejectsWrongLengthOpaqueSelector` must fail because a 31-character selector using only the valid alphabet is accepted. | Selector validation only; reverse the condition and verify file hash. |
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
| MUT-01 | green | Initial detector stayed green; the focused regression failed with accepted selector/code `""`. | killed after follow-up | `8985E52135F8684DABC1B42CB1B4028695FC5C3ED804D56FD79C079C1EF9367F` | Added `TestCarrierValidationRejectsWrongLengthOpaqueSelector`. |
| MUT-02 | green | Initial detector stayed green; the strengthened detector failed on `expected untrusted route denial`. | killed after follow-up | `54A341E7E6F2B45C2D0EDFA173521FDEB8B437FB8E66D11FD38D664B89A123E7` | Added untrusted-route assertions to `TestCheckRouteUse`. |
| MUT-03 | green | Detector exposed `svc.not-exposure-eligible` in the allowed set. | killed | `4A8E4B01AF5753FD67D3EA2CF9941C5C701202D79E478D5833F66F1E259B9E98` | Existing detector sufficient. |
| MUT-04 | green | Detector reported an unexpected denial for the admitted `trusted` policy reference. | killed | `F7CC4FCC096FBA0795C16E281CAC3C39117198574D8843243D46660EC3584860` | Existing detector sufficient. |
| MUT-05 | green | Detector observed `warming` instead of `ready` at the configured threshold. | killed | `A68E045A73AECACD9972F72057B00C0E36B0A9B6C59B93E9B286C50EBB12912B` | Existing detector sufficient. |
| MUT-06 | green | Detector received `durable replica store is unavailable` instead of the required content-identity rejection. | killed | `291752474FBE65572CF7048982E4FBC615EF513C64A305944D7EADA862C274BB` | Existing detector sufficient. |
| MUT-07 | green | Detector exposed `secret field = "top-secret"`. | killed | `79F4A3AF2B166BE728C9DFCED8247881950FA8DA16AC108EAC94D89F966EFB0C` | Existing detector sufficient. |
| MUT-08 | green | Detector observed lifecycle `ready` instead of `degraded`. | killed | `2E5BA4E27D49ABF84746603B63C6A9AA42249EDEF4D881F73EA4F75D5EFA0E06` | Existing detector sufficient. |
