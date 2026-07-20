# STB-505 Evidence

Date: 2026-07-20
Status: completed

## Outcome

Ardents now proves encrypted multi-node availability over the canonical private
Waku foundation. A four-node E2E topology reaches the declared replica target,
survives loss of a committed storage peer, repairs onto a spare peer, and lets
the owner recover and decrypt content after deleting its local payload. Foreign
storage peers retain and serve ciphertext only.

The complementary failure matrix proves that quota refusal, capability
revocation, corrupt retained bytes, and exhausted post-lease repair capacity do
not produce false availability. The public availability surface ends in
terminal `lost` only after no validated copy or current lease remains and the
bounded repair policy is exhausted.

## Implemented Contract

- The canonical Data API exposes replica intent mutation, reconciliation,
  availability snapshots, and persistent repair records through Authority and
  NodeRuntime; tests no longer depend on a private implementation facade.
- Availability cannot report `lost` while any validated copy remains. A partial
  target is `degraded`, including after local or remote copy loss.
- Repair work for the same `(Manifest, intent version)` is serialized by missing
  replica ordinal so concurrent repairs cannot select the same peer from stale
  pre-commit state. Independent blobs remain parallel.
- A node with a current committed replica lease may re-serve that encrypted
  payload through a narrow policy path. Generic relay-temporary payload serving
  remains denied.
- Placement failures expose bounded aggregate reason counts without peer IDs,
  routes, selectors, capability material, or payload data.
- Replica intent, availability, repairs, terminal fate, and diagnostic events
  remain persistent and sorted through the public read surface.

## Multi-Node Recovery Proof

`DAE-002` starts one owner and three independent storage-capable peers with
distinct signing grants and replay ledgers. It proves:

- encrypted placement reaches desired replication `3` with minimum `2`;
- the removed peer is a committed non-bootstrap storage target, so the failure
  is storage loss rather than accidental topology partition;
- repair selects the spare peer and restores the target;
- retained foreign bytes do not contain plaintext and the Blob API does not
  expose protected payload bytes;
- after the owner deletes its local payload, it fetches from a committed remote
  replica, decrypts with the owner-held key, and returns to target-satisfied
  availability;
- bounded `availability_observed` and `replica_repaired` diagnostics are emitted.

The final canonical E2E run completed this scenario in 24.452 seconds.

## Failure-Matrix Proof

`DAE-003` uses the same real four-node topology and proves:

- a quota-limited peer returns `quota_refused` while another eligible peer can
  still commit;
- corrupt committed bytes stop counting as a valid copy;
- revoking the owner's sender capability at the spare peer prevents replacement
  placement with an aggregate `capability_denied` explanation;
- removing the local payload leaves zero valid copies without claiming success;
- six bounded post-lease failures exhaust repair and produce terminal `lost`
  with zero current leases and zero pending repairs.

The final canonical E2E run completed this scenario in 17.408 seconds.

## Acceptance Checks

- Canonical Linux-container fast suite passed on the final code.
- Full integration passed 128/128 scenarios with zero failures in 347.718
  seconds. Reports are under
  `tests/.artifacts/reports/stb-505-integration-final/`.
- Full E2E passed 16/16 scenarios with zero failures in 125.913 seconds. Reports
  are under `tests/.artifacts/reports/stb-505-e2e-final/`.
- Linux race validation passed for `./internal/data/...`,
  `./internal/policy/...`, `./internal/runtime/authority`, and
  `./internal/runtime/process`.
- `go vet ./...` passed and `go mod verify` reported `all modules verified`.
- Test-catalog validation reported 144 tests, 144 formal bindings, 43
  scenarios, zero scenarios without tests, and zero issues.
- Production code-size validation passed for Data API, replication, transfer,
  policy, Authority, and NodeRuntime process packages.
- Import-boundary validation and focused unit suites passed. A final scan found
  no deferred `TODO`, `FIXME`, `HACK`, or production `panic` in the changed
  domains.

## Security And Architecture Review

- Waku remains the canonical carrier; no parallel transport, fake storage
  substrate, or new domain facade was introduced.
- Foreign nodes receive protected ciphertext only. Decryption keys remain with
  the owner and are never sent in repair, health, placement, or fetch messages.
- The committed-replica serving exception requires a local active commitment
  for the exact Blob and does not weaken generic relay retention policy.
- Quota, capability, corruption, and insufficient-capacity failures remain
  explainable without revealing peer identity, routes, selectors, or secrets.
- Integration-only terminal-budget acceleration is isolated in the existing
  process test-hook boundary; production repair state and public reconciliation
  paths remain authoritative.

## Resource And Orchestration Truth

All tests ran inside Linux Docker containers. Substantive commands used detached
named Compose containers and Linux `timeout`; progress was observed with short
state/log polls rather than foreground waits.

The first combined integration/E2E command reached its explicit five-minute
timeout while integration tests were still making progress. It was split into
independent suites with twelve-minute bounds. Integration then completed in
approximately 8.5 minutes and E2E in approximately 2.9 minutes. This was an
orchestration-budget correction, not a product failure.

Observed `vmmemWSL` was approximately 2.8-3.8 GiB during the final runs. The
final snapshot is `tests/.artifacts/resources/stb-505-final.json`; it records
approximately 3.9 GiB for WSL and 215.23 GiB free on drive C. No CPU, memory, or
disk exhaustion occurred.

## Evidence Surface

- `docs/data-availability-replication-semantics.md`
- `docs/qa/e2e/data-availability-peer-loss-recovery.md`
- `docs/qa/e2e/data-availability-terminal-failure.md`
- `docs/qa/integration/data-replica-reservation-placement.md`
- `internal/data/api/availability.go`
- `internal/data/availability.go`
- `internal/data/replica_placement.go`
- `internal/data/replication/repair_batch.go`
- `internal/data/replication/selection.go`
- `internal/data/transfer/fetch_request.go`
- `internal/policy/service.go`
- `internal/runtime/authority/controller_data_availability.go`
- `internal/runtime/process/api_authority_availability.go`
- `tests/e2e/data-substrate/availability_test.go`
- `tests/e2e/data-substrate/availability_failure_test.go`
- `tests/testkit/privacy.go`
- `tests/.artifacts/reports/stb-505-integration-final/summary.json`
- `tests/.artifacts/reports/stb-505-integration-final/junit.xml`
- `tests/.artifacts/reports/stb-505-e2e-final/summary.json`
- `tests/.artifacts/reports/stb-505-e2e-final/junit.xml`
- `tests/.artifacts/resources/stb-505-final.json`

## Phase 5 Transition Gate

Passed. Availability semantics are implemented beyond Waku Store; placement,
reservation, transfer, replication, repair, and terminal loss are bounded and
observable; real multi-node E2E proves encrypted recovery after peer loss; and
quota plus failure paths never claim unavailable data as available.
