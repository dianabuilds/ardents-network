# Encrypted Availability After Peer Loss

## Scenario ID

`DAE-002`

## Layer

`e2e`

## Domain

`Data Substrate`

## Category

`multi-node encrypted replication / peer loss / repair / fetch recovery`

## Goal

Prove that an owner and three storage-capable Waku peers reach three confirmed
encrypted copies, replace one lost remote copy on a distinct peer, and recover
the owner's deleted local payload from a remaining committed replica.

## Preconditions

- four real runtime nodes use independent state directories and one authorized
  private Data capability group;
- the owner holds the only plaintext key and starts with one encrypted Blob;
- three remote peers trust the owner and are independently discoverable;
- the owner sets desired copies to three and minimum copies to two.

## Steps

1. Reconcile the Replica Intent through the canonical local data surface.
2. Confirm one valid owner-local copy and two current remote commitments.
3. Stop a committed non-bootstrap storage peer and fail its bounded health
   probe.
4. Reconcile and confirm repair commits to the previously unused storage peer.
5. Inspect remote files and API snapshots for ciphertext-only retention.
6. Delete the owner's local payload, fetch it through the canonical data
   surface, decrypt with the owner-held key, and reconcile again.
7. Read availability, repair records, and bounded diagnostics.

## Expected Result

- declared availability equals three validated distinct copies before and
  after repair;
- the lost peer is absent from current active commitments;
- remote payloads contain no tested plaintext and expose no payload bytes in
  Blob snapshots;
- the owner recovers the exact plaintext from a remaining committed replica;
- `availability_observed` and `replica_repaired` are operator-visible.

## Failure/Degraded Variant

- concurrent repair ordinals must not select the same target;
- a generic relay-retention policy denial must not prevent a current committed
  replica from serving its exact ciphertext;
- loss of a bootstrap hub is a partition scenario and is intentionally outside
  this single-storage-peer failure budget.

## Related Tests

- `tests/e2e/data-substrate/availability_test.go::TestEncryptedAvailabilitySurvivesPeerLossRepairAndOwnerPayloadLoss`

## False Positive Risk

- counting discovery announcements instead of commitments can report copies
  that do not exist;
- retaining ciphertext without proving fetch recovery can hide an unusable
  replica;
- sharing the plaintext key with storage peers would invalidate the privacy
  proof.

## False Negative Risk

- the stopped peer must not be the fixture's only bootstrap hub;
- all network waits and health probes are bounded by explicit contexts.

## Notes

Waku remains the real coordination and private-message carrier. No in-memory or
alternate transport substitutes the tested path.
