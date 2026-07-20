# Data Availability And Replication Semantics

## 0. Status And Role

This document is the normative `v1` contract for distributed data availability,
replica placement, leases, repair, object composition, and chunking. It refines
`docs/data-substrate-requirements.md` and is owned by `Data Substrate`.

This contract describes target product behavior. Existing local retention and
encrypted fetch behavior remains valid, but metadata-only source announcements
and current `available-remote` state do not yet satisfy the replica commitments
defined here. STB-502 through STB-506 implement and prove this contract.

Waku is the canonical coordination and private-message carrier. Waku Store is
not a durable blob store and a Store result never counts as a replica.

## 1. Canonical Units And Relationships

### 1.1 Data Object

A Data Object is the logical owner-scoped record exposed to applications. It
contains application metadata and zero or more payload roots. A payload root is
a `manifest` reference. New `v1` writes MUST NOT attach raw blob references
directly to a Data Object.

An object without a payload root is metadata-only. Its metadata availability
does not imply payload availability.

### 1.2 Manifest

A Manifest is a deterministic, canonical structure with:

- owner and object identity binding;
- ordered references with byte ranges and logical roles;
- total plaintext length and media type;
- encryption/key identifier metadata, without key material;
- retention and replica intent identifier;
- canonical encoding version and content-derived identifier.

A leaf manifest contains ordered `blob` references. A root manifest may contain
ordered leaf `manifest` references. A manifest MUST contain only one reference
kind at a given level and MUST be acyclic.

Each leaf contains at most 512 blob references. A root contains at most 512 leaf
references. This bounds a two-level `v1` payload at approximately 16 GiB with
the canonical chunk size. Larger payloads fail explicitly as unsupported; the
implementation MUST NOT silently create an unbounded manifest graph.

### 1.3 Blob And Chunking

A Blob is the independently verifiable replication and transfer unit. Its ID
and CID are derived from the exact encrypted bytes retained and transferred.
Possession of the Blob does not grant plaintext access.

The canonical logical plaintext chunk size is 64 KiB:

- payloads of 64 KiB or less produce one Blob;
- larger payloads are split into consecutive 64 KiB chunks;
- only the final chunk may be shorter;
- each chunk is encrypted and content-addressed independently;
- byte order is defined only by the Manifest, never by source arrival order.

The 64 KiB choice keeps a ciphertext chunk plus protocol overhead inside the
128 KiB private-envelope bound. A privacy transport may further segment it into
fixed route cells; transport segmentation does not change Blob identity or the
Manifest.

## 2. Replica Intent

Replica Intent is versioned owner-authorized desired state attached to a
Manifest root. It applies transitively to every reachable Blob. It contains:

- desired committed-copy count;
- minimum usable-copy count;
- lease policy and renewal horizon;
- eligible-peer and failure-domain constraints;
- retention class: `temporary`, `durable`, or `local-only`;
- authorization/capability reference and policy version;
- creation, update, and optional expiry time.

The default durable objective is three committed copies on distinct node
identities, including a valid owner-local copy. Therefore normal placement for
an online owner seeks two remote committed replicas. The minimum usable count is
one. Policy may raise or lower the desired count but MUST NOT claim replicated
availability when the desired count is less than two.

Replica Intent is desired state. Only observed valid local payload and current
commit acknowledgements establish replica truth.

## 3. Eligible Peers And Placement

A peer is eligible for a reservation only when all are true:

- its identity and current network presence are authenticated and fresh;
- trust and capability evaluation authorizes retention and re-serving for the
  exact realm, object class, and encrypted content;
- policy permits the peer, retention class, lease duration, and byte count;
- advertised free capacity covers the reservation plus required safety margin;
- the peer supports the required protocol and encryption version;
- it is not the same node identity as an already selected placement;
- it is reachable through a currently usable canonical network route;
- it is not quarantined for corruption, repeated protocol failure, or abuse.

Placement MUST use distinct node identities. When trustworthy failure-domain
labels are available, it MUST spread replicas across failure domains before
placing a second replica in one domain. Unknown failure-domain information is
reported as `topology-unverified`; it is not invented from IP address alone.

Selection SHOULD prefer longer expected presence, lower current relay-retention
pressure, sufficient headroom, source diversity, and recent successful service.
These scores choose among eligible peers only; they cannot override admission.

For initial `v1` placement, the owner enumerates at most sixteen current node
records in deterministic identity order and requests a target-signed capacity
observation over `BLOB_REPLICA_CONTROL`. At most four queries run concurrently;
each round trip is bounded to three seconds. Peers already represented by a
commitment for the same Blob and intent are excluded before network I/O. A
transient missing response from an otherwise eligible peer receives one bounded
retry within the caller's placement/repair deadline; rejection and invalid
responses are never retried as availability.
The response binds the operation and target identity and reports free, reserved,
and committed bytes at the target's observed time. Selection requires headroom
of the greater of 64 KiB or five percent of the encrypted Blob size. Missing,
rejected, stale,
future-dated, untrusted, policy-denied, or unusable responses are ineligible;
unknown failure-domain information remains empty and topology-unverified.

Replica retention is always byte-bounded. A node with no explicit replica or
relay-retention byte limit uses a fail-safe 1 GiB replica quota; `0` does not
mean unlimited replica storage.

## 4. Reservation, Transfer, Commit, And Acknowledgement

### 4.1 Reservation

The placer sends an authenticated reservation offer containing the Replica
Intent version, Blob CID, encrypted byte count, requested lease duration,
protocol version, expiry, and an unpredictable operation nonce.

The implemented reservation protocol version is `1`. Version `1` permits one
canonical encrypted chunk inline: at most 64 KiB plaintext plus the 16-byte
AES-GCM authentication tag. Larger offers and unknown protocol/cipher versions
are rejected explicitly as `transfer_unsupported`; they are never allowed to
overflow the private-envelope limit or count as partial replicas. Large logical
payloads are represented by a bounded Manifest tree and placed as independently
verifiable chunk Blobs.

An acceptance is bound to the peer, Blob CID, intent version, reserved bytes,
and a single-use reservation token. It expires after two minutes. Acceptance
reserves capacity but does not count as a copy.

A peer may explicitly reject for quota, policy, unsupported version, revoked
capability, pressure, or duplicate commitment. Rejection is terminal for that
reservation and remains operator-visible; placement may select another peer.

### 4.2 Transfer And Validation

Transfer uses the private Data Substrate exchange over the canonical Waku
foundation. Every received Blob is authenticated, bounded, written as encrypted
bytes, and verified against the requested CID before commit. Partial data is
staged and MUST NOT be announced or served as available.

Chunk Manifests use the same `BLOB_FETCH_REQUEST` and `BLOB_FETCH_RESPONSE`
private message classes with a signed `resource_kind=manifest` discriminator.
The receiver verifies the deterministic Manifest CID before publishing it.
Manifest leaves are fetched before their referenced chunks, but become locally
published only after every referenced ciphertext CID is present and valid.
Interrupted attempts retain valid chunks for CID-based resume; unreferenced
`staging` chunks are removed during restart reconciliation.

### 4.3 Commit Acknowledgement

After durable write and CID verification, the retaining peer returns a signed
commit acknowledgement containing:

- peer and Blob identity;
- Replica Intent version;
- reservation nonce/token digest;
- committed encrypted size;
- lease start and expiry;
- protocol and storage-format versions.

Only a valid commit acknowledgement creates a Committed Replica. Duplicate
commit messages are idempotent. Acknowledgements for a different CID, peer,
intent version, expired reservation, or superseded operation are rejected.

A retaining peer may re-serve the exact encrypted Blob while it holds a current
local Committed Replica and peer serving remains enabled by policy. This narrow
replica obligation does not enable generic relay-cache re-serving: an expired,
revoked, corrupt, missing-payload, or non-committed relay entry remains
ineligible. The retaining peer receives no payload key through placement or
fetch and therefore serves ciphertext only.

## 5. Lease And Freshness Semantics

The default Replica Lease is 24 hours. It is renewable while authorization,
policy, capacity, payload integrity, and Replica Intent remain valid.

- renewal begins no later than eight hours before expiry;
- a signed replica-health observation is source-fresh for 15 minutes and is
  normally refreshed at least every five minutes;
- freshness is additionally bounded by network-presence expiry, capability
  expiry/revocation, and the Replica Lease expiry;
- an expired or revoked lease stops counting immediately;
- clock skew beyond the network protocol bound is rejected rather than extending
  a lease;
- a short network partition does not delete local encrypted bytes, but stale
  copies stop counting until authenticated renewal or revalidation succeeds.

A Source Observation is merely a fresh fetch candidate. It never becomes a
Committed Replica without a matching lease and commit acknowledgement.

The `v1` renewal path reuses the private `BLOB_REPLICA_CONTROL` message class;
it does not introduce a public topic or parallel transport. A signed
`health_query` binds the current commitment and requested expiry, and the signed
`health_result` returns `healthy`, `corrupt`, or `revoked` with the resulting
commitment state. A timeout or malformed response marks the source-side
observation stale without revoking the still-current lease or declaring loss.

## 6. Pin Semantics

`local` pin prevents normal local garbage collection while policy and local
capacity permit it. It creates no remote availability claim.

`replicated` pin creates a durable Replica Intent with no owner-selected end
time. It is still realized through finite renewable Replica Leases. It does not
override quota, capability revocation, corruption handling, or operator policy.

The existing `PinBlob` operation has `local` scope. A replicated pin requires an
explicit scope and intent in the extended API; absence of scope MUST NOT be
interpreted as a network durability promise.

Unpinning stops future renewal. Existing remote copies remain valid only through
their current lease or a replacement temporary-retention policy; unpin does not
silently command immediate destructive deletion.

## 7. Availability States And Guarantees

Copy retention state (`available-local`, `retained-temporary`, `pinned`,
`expired`, `deleted`) and aggregate availability are independent axes.

The aggregate state for a Manifest is derived from every reachable Blob:

- `target-satisfied`: every Blob has at least the desired number of distinct,
  valid, fresh Committed Replicas;
- `best-effort`: every Blob has at least one currently usable validated copy,
  but no multi-copy Replica Intent is active or fully committed;
- `degraded`: every Blob has at least the minimum usable count, but one or more
  is below desired count, stale, topology-unverified under a required constraint,
  or undergoing repair;
- `unavailable`: at least one Blob has no currently usable validated source,
  but an unexpired lease, partition, or incomplete bounded repair leaves loss
  unproven;
- `lost`: at least one Blob has no validated copy after all known leases are
  expired/revoked, all known sources are rejected/missing/corrupt, and the
  bounded repair/source-discovery cycle terminates without a candidate.

An Object is no more available than its least-available payload Manifest.
Metadata-only Objects report metadata availability separately.

The word `guaranteed` is allowed only as `lease-backed target satisfied`: at the
observation time the configured number of independent peers have authenticated
commitments. It is not a promise of future reachability under arbitrary
simultaneous failure or partition. Waku delivery, an announcement, a reservation
acceptance, or an in-progress transfer alone provides no availability guarantee.

`lost` is terminal for the current repair operation and requires explicit
operator-visible evidence. If a previously unknown valid copy is later
authenticated, the state may recover to `degraded` and a new repair cycle starts;
the prior loss event remains in diagnostics.

## 8. Repair Triggers And Reconciliation

Repair starts when any Blob has fewer valid committed copies than desired due to:

- lease expiry or failed renewal;
- peer loss or presence becoming stale;
- failed integrity verification or a corruption report;
- capability or trust revocation;
- policy change or placement-constraint drift;
- explicit peer deletion/eviction acknowledgement;
- quota refusal during initial placement;
- a network partition ending and revealing divergent replica truth.

Corrupt or identity-mismatched copies are quarantined immediately and never used
as repair sources. Revoked peers stop counting immediately. Quota refusal never
counts as a replica and selects another eligible peer.

Repair is idempotent per `(intent version, Blob CID, missing ordinal)`, uses
bounded concurrency and exponential backoff with jitter, and persists its state
across daemon restart. It MUST NOT create more simultaneous reservations than
the desired count plus one replacement candidate per missing Blob.

One persisted repair attempt may perform one additional placement cycle when
the first cycle contains only transient missing-response or generic control
exchange failures. The retry remains inside the same attempt deadline and does
not apply to quota, policy, capability, trust, integrity, lease, or explicit
unsupported denials. A failed bounded cycle is recorded once before normal
persisted backoff.

Missing ordinals for the same Blob and intent are scheduled sequentially so a
successful commitment is visible before the next ordinal selects a target.
Repairs for different Blobs remain eligible for bounded parallel execution.
This prevents concurrent ordinals from choosing the same peer against a stale
placement snapshot.

The default repair cycle permits six attempts per candidate, backs off from five
seconds to at most five minutes, and runs for at most 30 minutes after the last
known lease expires. A still-active lease or an observed network partition keeps
the state `unavailable`; `lost` requires this post-lease cycle to finish with no
validated candidate. Policy may extend these bounds but may not shorten them
silently or declare loss while a current commitment exists.

The persisted repair record distinguishes all proactive attempts from
post-lease attempts. Attempts made while a lease is current may find a
replacement early, but do not consume the terminal loss budget. `LossEligibleAt`
tracks the latest known lease boundary and `DeadlineAt` is thirty minutes later;
both extend monotonically when a later valid lease is observed.

## 9. Required Failure Semantics

- **Owner offline:** remote current commitments continue serving; repair may be
  coordinated by an authorized replica only when the intent delegates that
  right. Otherwise state can remain target-satisfied until a lease needs owner
  authorization, then degrades explicitly.
- **Peer loss:** the lease stops counting when presence/freshness bounds expire;
  repair selects a different eligible peer.
- **Corrupt copy:** quarantine, explicit integrity failure, decrement committed
  count, repair only from another validated source.
- **Quota refusal:** record the reason and continue bounded placement; never
  convert acceptance or partial transfer into availability.
- **Capability revocation:** stop counting and serving immediately; retain or
  erase ciphertext according to revocation/retention policy, with diagnostics.
- **Expiry:** stop counting at the exact lease boundary; local bytes may await
  GC but cannot be advertised as committed.
- **Network partition:** preserve local bytes and commitments, mark remote proof
  stale as bounds expire, avoid declaring loss until lease expiry and bounded
  post-partition discovery complete, then reconcile by intent version and CID.

## 10. Operator And API Truth

For each Object/Manifest, the canonical surface must expose:

- intent version, desired and minimum counts;
- valid, stale, pending, rejected, corrupt, and expired replica counts;
- aggregate availability state and reason;
- last successful commit/renewal and next lease expiry;
- placement constraint and topology-verification outcome;
- active repair operation, attempts, next retry, and terminal failure;
- source observations separately from committed replicas;
- local pin scope and replicated pin intent.

Secrets, capability material, private selectors, raw routes, and plaintext are
never included in these diagnostics.

STB-504 emits bounded `availability_observed`, `replica_health_stale`, and
`replica_repaired` diagnostics with state, reason, copy counts, intent version,
and repair ordinal. Payload bytes, content keys, capability material, private
selectors, and raw routes are excluded.

The canonical local data surface exposes `SetReplicaIntent`,
`ReconcileDataAvailability`, `GetAvailability`, and `ListReplicaRepairs`.
Unsatisfied placement reports aggregate denial reason counts without peer
identities, selectors, or routes.

## 11. Implementation Gates For Subsequent Tasks

STB-502 implements authenticated reservation/commit state and eligible peer
selection. STB-503 implements canonical chunk/manifests and resumable
integrity-checked transfer. STB-504 implements persistent lease renewal, repair
reconciliation, and bounded diagnostics. STB-505 completes canonical control
exposure and proves multi-node peer loss, corruption, refusal, revocation,
expiry, restart, partition, fetch recovery, and ciphertext-only intermediaries.
