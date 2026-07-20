# STB-504 Evidence

Date: 2026-07-19
Status: completed

## Outcome

Ardents now derives Manifest availability from persistent local payload truth and
fresh signed replica commitments, renews replica leases through the existing
private Waku control channel, and runs bounded repair to different eligible
peers. Source announcements alone never count as copies. Intent, availability,
repair backlog, health state, post-lease budget, and terminal fate survive daemon
restart.

## Implemented Contract

- Versioned Replica Intents apply transitively to every Blob reachable from a
  Manifest root and persist with availability snapshots and repair records.
- Aggregate truth exposes desired/minimum/valid/current-lease/stale/expired/
  corrupt/pending counts, reason, observation time, and next lease expiry.
- Signed `health_query` / `health_result` messages reuse
  `BLOB_REPLICA_CONTROL`; healthy responses renew a 24-hour lease, corrupt and
  revoked responses stop counting, and timeout/malformed responses mark stale.
- Health observations are fresh for 15 minutes and normally refreshed every
  five minutes; renewal starts within the eight-hour horizon.
- Repair identity is deterministic per `(intent version, Blob CID, missing
  ordinal)`, persists across restart, uses default concurrency two (hard cap
  four), bounded per-attempt contexts, deterministic exponential jitter from
  five seconds to five minutes, and at most six post-lease attempts.
- `LossEligibleAt` follows the latest known lease boundary and `DeadlineAt` is
  thirty minutes later. Pre-expiry attempts can repair early but cannot consume
  terminal loss budget. A current lease or observed partition stays
  `unavailable`; `lost` requires the post-lease budget/deadline to finish.
- Placement excludes every peer already observed for the same Blob and intent,
  including stale/corrupt peers, so repair selects a different eligible node.
- Renewal validates local protected payload before mutating commitment state.
- Bounded diagnostics emit `availability_observed`, `replica_health_stale`, and
  `replica_repaired` without payloads, keys, capability material, selectors, or
  raw routes.

## Failure And Recovery Coverage

- fresh or stale source announcements without commitments do not count;
- expired commitments stop counting at the lease boundary;
- corrupt protected payload is quarantined by signed health response;
- peer transport loss becomes stale after bounded explicit health timeout;
- independent Waku routes allow repair to a different storage peer;
- partition attempts do not cause premature terminal loss;
- partition/rejoin restores a fresh commitment and clears obsolete repair;
- six post-lease failures and the 30-minute post-lease deadline produce terminal
  loss only after all current leases cease to protect uncertainty;
- owner shutdown preserves remote encrypted retention, and owner restart restores
  intent and availability truth;
- missing local payload cannot partially renew an in-memory commitment.

## Acceptance Checks

- Canonical Linux-container fast suite passed on final code; `internal/data`
  completed in 2.404 seconds and `internal/runtime/process` in 14.432 seconds.
- Full Data Substrate integration passed 13/13 scenarios with zero failures in
  66.182 seconds. The retained canonical reports are under
  `tests/.artifacts/reports/stb-504-data-integration-full/`.
- Final focused DAI-004 passed over real three-node private Waku in 19.563
  seconds after the size-driven refactor.
- Linux race validation passed for `./internal/data/...`; final package times
  include 2.525 seconds for `internal/data` and 1.986 seconds for transfer.
- `go vet ./...` passed; `go mod verify` reported `all modules verified`.
- Test-catalog validation reported 142 tests, 142 formal bindings, 41 scenarios,
  and zero missing bindings, missing documents, orphan scenarios, or issues.
- Production code-size validation passed for `internal/data`,
  `internal/runtime/authority`, and `internal/runtime/process`. Two initial soft
  breaches were split into cohesive local/remote truth and manifest traversal
  helpers; the final checker reported `code size check passed`.

## Security And Architecture Review

- Data Substrate remains the owner of intent, commitments, availability, repair,
  and terminal fate. Replication owns Waku health/placement orchestration;
  Authority only exposes the existing runtime seam. No new domain facade or
  transport was introduced.
- Waku remains the canonical carrier. The three-node test fixture uses one
  authorized group capability channel with distinct signing grants and durable
  replay ledgers per node; it does not share two-party replay state.
- Health signatures bind action, operation, source, target, public key, prior
  commitment, requested expiry, resulting commitment, status, and reason.
- Only protected ciphertext is retained by foreign nodes. Health, availability,
  repair state, and diagnostics contain no plaintext or key material.
- The source has independent routes to both storage peers in DAI-004, so stopping
  one peer tests replica loss rather than accidentally partitioning the entire
  topology.

## Resource And Orchestration Truth

All tests ran inside Linux Docker containers. The final host snapshot is
`tests/.artifacts/resources/stb-504-final.json`: `vmmemWSL` used approximately
4.36 GiB, drive C retained 216.31 GiB free, and the highest sampled host process
used less than one percent CPU. Peak observed `vmmemWSL` during the final fast
suite was approximately 6.49 GiB; no CPU, memory, or disk exhaustion occurred.

Every substantive command used a detached named Compose container, a Linux
`timeout`, redirected logs, and separate sub-second `ps`/`top` polls. A stale UI
command card was explicitly distinguished from a completed container, and no
foreground Docker wait was used for acceptance.

## Evidence Surface

- `docs/data-availability-replication-semantics.md`
- `docs/qa/integration/data-availability-observation-repair.md`
- `internal/data/availability/`
- `internal/data/availability.go`
- `internal/data/replica_intent.go`
- `internal/data/repair_state.go`
- `internal/data/replica_placement.go`
- `internal/data/placement/health.go`
- `internal/data/replication/health.go`
- `internal/data/replication/reconcile.go`
- `internal/data/replication/selection.go`
- `internal/data/transfer/private_exchange.go`
- `tests/integration/data-substrate/availability_repair_test.go`
- `tests/testkit/privacy.go`
- `tests/.artifacts/reports/stb-504-data-integration-full/summary.json`
- `tests/.artifacts/reports/stb-504-data-integration-full/junit.xml`
- `tests/.artifacts/resources/stb-504-final.json`

## Deferred By Explicit Phase Boundary

STB-505 remains responsible for the broader E2E matrix: fetch continuity after
peer removal, quota refusal, capability revocation, terminal insufficient
capacity, ciphertext-only intermediary proof, canonical control-surface
exposure, and retained availability/repair diagnostics across that matrix.
