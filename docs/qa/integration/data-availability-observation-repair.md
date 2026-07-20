# Data Availability Observation And Repair

## Scenario ID

`DAI-004`

## Layer

`integration`

## Domain

`Data Substrate`

## Category

`lease health / availability reconciliation / bounded replica repair`

## Goal

Prove that confirmed encrypted copies are derived from current signed replica
commitments, corrupt or unreachable copies stop counting, and repair selects a
different eligible peer over the real private Waku path while preserving truth
and intent across owner restart.

## Preconditions

- three real Waku storage-capable nodes share independent grants and replay
  ledgers for one authorized Data Substrate capability channel;
- the source has direct bootstrap connectivity to both targets;
- signed node records and trust anchors authorize source/target interaction;
- the source owns one encrypted Blob, its Manifest, and a versioned two-copy
  Replica Intent.

## Steps

1. Commit the encrypted Blob from the source to target one.
2. Corrupt target one's protected payload and issue a signed health probe.
3. Reconcile availability and verify repair commits to target two.
4. Advance the Replica Intent version, restore target one, then stop its Waku
   transport and issue a bounded health probe.
5. Reconcile again and verify the stale copy is replaced through the independent
   target-two route.
6. Stop the owner, read retained encrypted metadata on target two, restart the
   owner, and read the persisted availability snapshot.

## Expected Result

- a health response binds the prior commitment and records corruption without
  counting the damaged copy;
- a health timeout records a stale commitment while its lease remains current;
- each repair excludes the failed peer and commits to a different eligible peer;
- final truth is `target-satisfied` with two valid copies and bounded diagnostic
  events for repair and availability observation;
- owner shutdown does not remove remote ciphertext, and intent/availability
  truth survives owner restart.

## Failure/Degraded Variant

- source announcements without a commitment never count as copies;
- expired leases stop counting at their boundary;
- a partition with a current lease remains `unavailable`, not `lost`;
- terminal loss requires six post-lease failures or the persisted 30-minute
  post-lease deadline;
- malformed health responses, revoked authorization, corruption, and timeout
  cannot renew a commitment.

## Related Tests

- `tests/integration/data-substrate/availability_repair_test.go::TestDataAvailabilityRepairsCorruptReplicaToDifferentWakuPeer`
- `internal/data/availability_test.go`
- `internal/data/replica_placement_test.go`
- `internal/data/transfer/private_exchange_test.go`

## False Positive Risk

- shared replay state between targets could make a two-party channel appear to
  support three nodes; the group fixture uses a distinct ledger per node;
- a single bootstrap hub would turn peer loss into a whole-network partition;
  the source therefore has independent routes to both targets;
- checking only Blob metadata would miss absent payload, stale commitment, or
  incorrect aggregate availability.

## False Negative Risk

- real Waku formation and health timeouts require bounded startup windows;
- test contexts must remain longer than the intentional three-to-four-second
  peer-loss probe but shorter than the container timeout.

## Notes

STB-504 establishes reconciliation and diagnostics truth. Broader quota,
revocation, insufficient-capacity, fetch-after-loss, and ciphertext-only E2E
coverage remains the explicit scope of STB-505.
