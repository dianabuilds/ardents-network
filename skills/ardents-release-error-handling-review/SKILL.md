---
name: ardents-release-error-handling-review
description: Release-time error handling review workflow for Ardents. Use when preparing a release or reviewing risky changes and you need to verify that errors are not ignored, failure paths are explicit, degraded states are explainable, error values preserve meaning, and runtime behaviour does not silently continue after broken operations.
---

# Ardents Release Error Handling Review

Use this skill when correctness depends on failure-path discipline.

The goal is not just "no ignored errors". The goal is that every important failure becomes an explicit, explainable runtime outcome.

## Read First

- `docs/system-concept.md`
- `docs/development-contract.md`
- `docs/engineering-constraints.md`
- the relevant domain document

## Workflow

1. State the release surface and the operations that can fail.
2. Review filesystem, network, persistence, serialization, startup, shutdown, and reconciliation paths for ignored or collapsed errors.
3. Check whether errors are converted into the right product outcome: fail, degrade, retry, reject, or report.
4. Check whether the local API and diagnostics preserve enough meaning for operators.
5. Check whether tests cover at least one representative failure path per risky area.
6. Reject the release if important errors are dropped, blurred, or converted into false success.

## Mandatory Checks

- no ignored `error` return in critical paths
- no broad "best effort" behaviour hiding authoritative failure
- no conversion of hard failure into success without explicit degraded reporting
- no generic wrapping that erases domain, operation, and recovery meaning
- no retry loop without bounded outcome and observable failure
- no startup/shutdown/save/load path that can fail silently
- no local API success response after internal failure

## Release Hotspots

- `Load` / `Save` / restore paths
- startup and shutdown
- network publish / subscribe / fetch paths
- diagnostics ledger writes
- service publication and unpublication
- workload reconcile and restart flows
- request decoding / encoding / API mapping

## Reject If

- a critical failure can be ignored
- a user-visible success can hide an internal failure
- degraded behaviour is possible but unexplained
- the code drops the original cause needed for diagnosis
- tests prove only the success path

## Output

When using this skill, produce:

- reviewed failure surfaces
- exact dropped, blurred, or mishandled error paths
- release impact of each issue
- required remediation before shipping
