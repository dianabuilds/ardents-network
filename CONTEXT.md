# Ardents Network

Ardents is a managed peer-to-peer node whose product domains own observable
runtime truth. This glossary names the concepts shared across those domains.

## Data Availability Language

**Data Object**:
The logical, owner-scoped record that gives application meaning to retained data.
_Avoid_: File, payload record

**Blob**:
A content-addressed unit of encrypted bytes that can be stored, transferred, and verified independently.
_Avoid_: File, attachment, chunk record

**Manifest**:
A canonical description that orders blobs, or child manifests, into the payload of a Data Object.
_Avoid_: Blob list, file metadata

**Replica Intent**:
The owner-authorized availability objective for a manifest and all blobs reachable from it.
_Avoid_: Replication promise, desired copies

**Reservation**:
A short-lived, peer-bound acceptance of capacity that authorizes one prospective replica transfer.
_Avoid_: Replica, allocation

**Committed Replica**:
A validated encrypted copy covered by a current, authenticated Replica Lease.
_Avoid_: Announced source, cached copy

**Replica Lease**:
A renewable, time-bounded commitment by an eligible peer to retain and serve a Committed Replica.
_Avoid_: TTL, pin

**Source Observation**:
Fresh evidence that a peer claims and is authorized to serve a Blob; it is a fetch candidate, not proof of a Committed Replica.
_Avoid_: Replica acknowledgement, availability guarantee

**Repair**:
The bounded reconciliation process that replaces missing, stale, corrupt, expired, or ineligible Committed Replicas.
_Avoid_: Retry, re-fetch
