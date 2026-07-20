# STB-501 Evidence

Date: 2026-07-19
Status: completed

## Outcome

Distributed data availability now has one normative `v1` semantic contract in
`docs/data-availability-replication-semantics.md`. It separates local copy
retention state from aggregate object availability and separates a source
announcement from a lease-backed committed replica.

The contract defines the complete object composition and placement vocabulary:
Data Object, Manifest, Blob, Replica Intent, Reservation, Committed Replica,
Replica Lease, Source Observation, and Repair. These terms are also captured in
the root `CONTEXT.md` glossary without implementation details.

## Selected Semantics

- Default durable intent: three distinct committed copies including a valid
  owner-local copy, with one as the minimum usable count.
- Canonical chunk: 64 KiB plaintext, encrypted and content-addressed per Blob;
  bounded two-level manifests support approximately 16 GiB in `v1`.
- Reservation expiry: two minutes; default renewable Replica Lease: 24 hours.
- Signed replica-health freshness: 15 minutes, normally refreshed within five.
- Repair: bounded, persistent, idempotent, six attempts per candidate and at
  most 30 minutes after the final known lease expires.
- Aggregate states: `target-satisfied`, `best-effort`, `degraded`,
  `unavailable`, and `lost`.
- Local and replicated pin scopes are distinct; existing `PinBlob` is explicitly
  local and cannot be interpreted as a network durability promise.

## Acceptance Matrix

| Required case | Normative outcome |
| --- | --- |
| Owner offline | Current remote leases continue; delegated repair only when the signed intent permits it. |
| Peer loss | Freshness/lease truth stops counting the peer and starts replacement placement. |
| Corrupt copy | Copy is quarantined, excluded, and never selected as a repair source. |
| Quota refusal | Explicit rejection is retained and another eligible peer is selected. |
| Capability revocation | Peer stops counting and serving immediately; ciphertext handling follows policy. |
| Expiry | Copy stops counting at the lease boundary even if bytes await garbage collection. |
| Network partition | Local bytes survive; remote proof becomes stale; loss waits for lease expiry and bounded post-partition discovery. |

## Architecture And Security Truth

- Data Substrate owns availability, placement intent, retention, transfer, and
  repair truth; Policy, Discovery, and Network Foundation provide bounded inputs
  without taking ownership.
- Waku remains the canonical private coordination carrier and Waku Store is
  explicitly not counted as durable blob storage.
- Commit requires durable encrypted write, CID validation, and an authenticated
  acknowledgement. Announcements, reservations, and partial transfers do not
  count as replicas.
- The 64 KiB Blob chunk remains below the current 128 KiB private-envelope bound;
  future fixed route-cell segmentation does not change Blob identity.
- Diagnostics expose counts, lease/repair reasons, and topology verification
  without secrets, capability material, routes, selectors, or plaintext.

## Source-Of-Truth Updates

- `CONTEXT.md`
- `docs/data-availability-replication-semantics.md`
- `docs/data-substrate-requirements.md`
- `docs/domains/data-substrate.md`

The semantic document explicitly marks current metadata-only remote availability
as insufficient for replica commitment. STB-502 through STB-506 are the required
implementation and proof sequence; no existing code behavior is falsely
described as already satisfying the new contract.
