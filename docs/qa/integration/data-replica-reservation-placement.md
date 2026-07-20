# Data Replica Reservation And Placement

## Scenario ID

`DAI-002`

## Layer

`integration`

## Domain

`Data Substrate`

## Category

`replica reservation / private placement / commit acknowledgement`

## Goal

Prove that an encrypted Blob is counted as a remote committed replica only after
an authenticated capability-scoped reservation, durable ciphertext retention,
content-identity validation, and commit acknowledgement over the real private
Waku data-exchange path.

## Preconditions

- two real Waku nodes share an authorized Data Substrate capability channel;
- both signed node records are available to trust evaluation;
- the source owns one locally available encrypted Blob;
- target policy permits relay retention and has sufficient quota.

## Steps

1. Start the target node and then bootstrap the source node from it.
2. Exchange signed node records and wait for usable network participation.
3. Store encrypted bytes on the source and request one placement on the target.
4. Observe reserve acceptance and commit acknowledgement through the private
   replica-control message class.
5. Read source and target placement ledgers and target Blob truth.

## Expected Result

- the target retains exactly the requested ciphertext under its canonical CID;
- the target Blob is encrypted and `relay-temporary`;
- both ledgers contain the same active, unexpired commitment for the target;
- the target may re-serve that ciphertext only while the local commitment is
  current and peer serving remains policy-enabled;
- reservation token, capability material, selectors, and ciphertext do not
  appear in diagnostics.

## Failure/Degraded Variants

- untrusted or revoked peers receive an explicit rejection and no reservation;
- insufficient quota returns `quota_refused` and stores no Blob;
- wrong CID, partial/empty ciphertext, expired reservation, excessive lease,
  conflicting duplicate, or replay cannot create a commitment;
- a reservation acceptance without durable commit never counts as a replica.

## Related Tests

- `tests/integration/data-substrate/replica_placement_test.go::TestDataReplicaPlacementCommitsEncryptedCopyOverPrivateWaku`
- `internal/data/placement/receiver_test.go`

## False Positive Risk

- checking only the response can miss absent target bytes or source ledger truth;
- using an in-memory carrier can hide Waku capability, encryption, and replay
  failures;
- a metadata-only `available-remote` record is not a committed replica.

## False Negative Risk

- both capability authorities must authorize the opposite sender for the
  request/acknowledgement flow;
- signed discovery records must be imported before trust evaluation;
- waits must be bounded and must not accept context timeout as policy denial.
