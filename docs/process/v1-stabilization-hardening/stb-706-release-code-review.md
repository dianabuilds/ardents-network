# STB-706 Release Code Review

Date: 2026-07-20

## Reviewed surface

The review covered the root `v1` implementation and its supported operational
surface:

- node startup, shutdown, live configuration, and diagnostics truth;
- Waku transport runtime, reachability, private messaging, and data transfer;
- workload observation and ingress paths;
- persistence transitions that affect operator-visible terminal state;
- `ardents.ps1`, `scripts/deploy`, Docker Compose definitions, and CI gates;
- handwritten production Go size in `internal`, `cmd`, and `boundary`.

The review compared behavior with the system, network, data, configuration,
deployment, engineering, and development contracts. Generated protobuf code and
the vendored Waku/libp2p forks were not treated as handwritten refactor targets.

## Closed blockers

### Data transfer terminal truth

Blob, manifest, and chunked fetch paths ignored persistence errors from transfer
progress and terminal transitions. A successful payload operation could return
success while operator-visible transfer truth remained pending. Commit
`c720505` centralizes lifecycle recording, preserves the primary failure when
terminal recording also fails, and propagates progress/completion failures.

Evidence:

- `go test ./internal/data/transfer`: passed in Linux container;
- `DAI-003`: passed over private Waku; report
  `tests/.artifacts/reports/stb-706-transfer-truth-dai003`.

### Configuration rollback truth

Live reload discarded every applier rollback error and still reported
`rolled_back`. Commit `f9c9fb3` adds the distinct `rollback_failed` outcome,
aggregates rollback errors, and degrades node diagnostics with
`config.reload.rollback_failed` when runtime owners may contain mixed effective
configuration.

Evidence: targeted `internal/runtime/config` and `internal/runtime/process`
Linux-container regression tests passed.

### Partial Docker deployment cleanup

An error during `./ardents.ps1 up` could leave partial containers behind,
especially before `cluster.json` existed. Commit `b561a06` makes startup clean
its exact Compose project containers/network while retaining persistent volumes
and preserving both startup and cleanup failures.

Evidence: an isolated deliberate failure using project
`ardents-stb706-failure` returned the provisioning error with zero remaining
containers. Its seven test-only retained volumes were then removed by exact
project label.

### Reachability event CPU spin

The network runtime reacquired the same closed reachability event channel on
every loop iteration. Unexpected subscription closure therefore caused an
unbounded CPU spin. Commit `a9d4fe8` detaches the closed subscription, withdraws
the public reachability claim as `unknown`, and continues bounded ticker-driven
reconciliation.

Evidence: the focused transport regression completed in `0.033s`; code-size
guard passed.

## Reviewed non-findings

- Workload observation errors returned through snapshot synchronization are not
  silent: the authority records `workload.observation.refresh_failed`, degrades
  workload/node health, and makes stale impact explicit before returning.
- Per-connection ingress proxy copy failures do not create false service
  readiness: hosted-service probes determine publication backing independently.
- Root production Go has no file or function above the hard size limits. The
  reviewed soft-limit growth in config reload was split by responsibility.

## Decision

No unresolved release-code-review blocker remains in the reviewed surface. This
decision advances STB-706 to the dedicated failure-oriented bug hunt; it is not
a release or final acceptance decision.
