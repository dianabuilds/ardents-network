# STB-706 Release Regression Sweep

Date: 2026-07-20

## Release Scope

The sweep ties together the stabilization candidate and the STB-706 fixes for
transfer truth, configuration rollback, deployment cleanup, reachability-loop
CPU behavior, retained data atomicity, and workload-container cleanup.

## Cross-Cutting Checks

| Product truth | Result/evidence |
| --- | --- |
| startup, stop, restart | Phase 6 deployment/E2E evidence remains accepted; failed Compose and failed workload starts now clean exact resources |
| diagnostics/degraded truth | config rollback, workload observation, discovery privacy, and data privacy paths expose explicit domain reasons |
| local API truth | failed transfer/blob commits restore the last durable state; terminal transfer writes cannot return false success |
| real foundations | Waku remains the canonical network/messaging foundation; Docker remains the workload execution foundation |
| discovery/network truth | closed reachability observation becomes `unknown`; DNS failure withdraws obsolete discoveries and retries boundedly |
| service publication truth | hosted service publication remains gated by desired state plus real readiness/liveness observations |
| broken-path tests | persistence, duplicate command, closed channel, partial deployment, and Docker start failure all have concrete failure evidence |
| document alignment | persistent-state security and the stabilization review evidence describe the implemented commit points and residual risks |

The focused packages changed by the final findings passed in Linux containers:
`internal/data` in `1.315s` and `internal/workload/execution` in `0.112s`.
Production Go code-size checks passed. No test container remains running.

## Residual Risks

- The exact release commit still needs the STB-707 canonical acceptance matrix.
- Public release additionally requires the separate uninterrupted 48-hour
  qualification; that run is deliberately outside the stabilization/refactor
  goal and does not block beginning refactoring.
- The two dependency residuals remain bounded and registered in
  `docs/security-exceptions.md`; STB-707 must revalidate their classification.

## Decision

The candidate is `review-clean` and may advance to STB-707. It is not yet a
publicly shippable release until final acceptance and pre-release qualification
complete. No cross-cutting regression blocker remains in STB-706.
