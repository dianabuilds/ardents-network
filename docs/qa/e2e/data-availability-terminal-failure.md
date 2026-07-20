# Availability Failure Matrix And Terminal Loss

## Scenario ID

`DAE-003`

## Layer

`e2e`

## Domain

`Data Substrate`

## Category

`quota / capability revocation / corruption / terminal insufficient capacity`

## Goal

Prove that refusal and failure paths never become false availability and that
zero validated copies reach explicit `lost` only after the bounded repair
budget is exhausted.

## Preconditions

- an owner and three storage-capable Waku peers share a private Data capability
  group;
- one peer has a replica quota below the required encrypted size plus headroom;
- two peers have sufficient capacity;
- desired copies are two and minimum copies are one.

## Steps

1. Place one remote copy and verify the constrained peer is denied with
   `quota_refused` while another peer commits successfully.
2. Corrupt the committed peer's protected payload and verify signed health
   observation marks the commitment corrupt.
3. Revoke the owner sender capability at the remaining healthy peer.
4. Delete the owner's local payload and reconcile availability.
5. Verify zero valid copies produce `unavailable` while repair remains pending.
6. Advance the real persistent repair records through six deterministic
   post-lease failures and reconcile again.
7. Read terminal repair records, aggregate availability, and diagnostics through
   the canonical local surface.

## Expected Result

- quota refusal never creates a commitment or copy count;
- revoked capability produces `capability_denied` and stores no replacement;
- the corrupt copy does not count and remains visible as corrupt;
- pending repair reports `unavailable`, not premature `lost`;
- after six terminal attempts with no current lease or validated copy, repair
  records are `failed` and aggregate availability is `lost`;
- placement errors expose aggregate reason counts without peer identities.

## Failure/Degraded Variant

- any reservation acceptance, corrupt bytes, or revoked peer counted as valid
  is a release blocker;
- terminal loss while a current lease remains is a release blocker;
- a remaining validated owner copy must keep state `degraded`, never `lost`.

## Related Tests

- `tests/e2e/data-substrate/availability_failure_test.go::TestAvailabilityFailureMatrixEndsInHonestTerminalLoss`
- `internal/data/availability_test.go`

## False Positive Risk

- checking only returned errors can miss false copy counts;
- skipping the intermediate `unavailable` state can hide premature terminal
  loss;
- a generic timeout does not prove explicit capability refusal.

## False Negative Risk

- revocation uses the fixture's fixed authority clock;
- repair backoff is advanced through the integration-only deterministic state
  hook while terminal state is asserted only through the public surface.

## Notes

The test hook does not invent a separate repair implementation; it advances the
same persisted production repair state machine to avoid a 30-minute wall-clock
test.
