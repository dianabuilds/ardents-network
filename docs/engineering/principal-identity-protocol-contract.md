# Principal Identity Protocol Contract

Status: frozen v1 implementation contract through PIA-011B.

This document materializes the protocol, transport, generation, and resource
catalogue decisions in the Principal Identity and Access product design. It
enables the identity service for each surface only on its own protected Unix
listener. Application enrollment is implemented behind a dormant gate;
product-call admission is staged in PIA-012, while production activation
remains blocked until PIA-014 supplies owner-aware Blob/content access.

## Sources And Generated Outputs

| Source | Generated output | Responsibility |
|---|---|---|
| `api/ardents/identity/v1/artifacts.proto` | `internal/identity/protocol` and `sdk/go/protocol/identityv1` | canonical signed artifact wire types and structured challenge fields; generated twice with explicit `M` mappings |
| `api/ardents/identity/v1/contract.go` | imported by server and SDK codecs | single immutable owner of v1 domains, prefixes, bounds, actions, and resource-kind contracts |
| `api/ardents/v1/identity.proto` | `internal/localapi/protocol` | Operator-only authentication and identity administration service |
| `api/ardents/application/v1/content.proto` | `sdk/go/protocol/applicationv1` | public Application content service |
| `api/ardents/application/v1/identity.proto` | `internal/applicationapi/protocol/applicationv1` and `sdk/go/protocol/applicationidentityv1` | Application-only authentication service; generated twice with explicit `M` mappings |

Domain models must not alias generated messages. Operator generated code must
not import `sdk/go`. Application identity is generated into separate server and
SDK Go packages from the same source and therefore retains one protobuf service
name and wire contract. The server copy maps structured artifacts to
`internal/identity/protocol`; the SDK copy maps them to
`sdk/go/protocol/identityv1`. This split is required because the two artifact
descriptors intentionally have the same protobuf file name and cannot be
registered in one Go binary. Application content remains the public generated
package used by the Node content adapter and SDK. Generated files are never
edited by hand. The canonical stale check is:

```text
powershell -File scripts/generate-api.ps1 -Check
```

The same gate regenerates and compares
`api/ardents/identity/v1/testdata/artifact-vectors.json`, including all six
artifact byte/ID/signature vectors and the signed timestamp/lifetime boundary
corpus. Server and SDK consume the file in separate test binaries so their
generated descriptors are never registered together.

The toolchain is pinned to protoc 33.4, protoc-gen-go 1.36.9, and
protoc-gen-connect-go 1.19.1. The generator contains the future artifact source
and both authentication sources explicitly; each becomes mandatory in the
leaf that introduces that source.

## Authentication Core

PIA-006B implements the transport-independent owner in
`internal/identity/access`; PIA-009A adds the Operator adapter and PIA-011A adds
the separate Application adapter. `Begin` accepts only a server-derived `SourceKey` and complete
`AuthenticationBinding`. It returns structured `ChallengeFields`; neither the
server nor SDK exposes an opaque signing payload. Server and SDK independently
marshal deterministic protobuf and prepend exactly one immutable domain:

```text
ardents:authentication-challenge:v1\x00
ardents:enrollment-challenge:v1\x00
```

The SDK exposes only purpose-specific signers. The authentication signer also
binds the supplied device key to its parsed Key Credential; the enrollment
signer binds the offline root key to `Challenge.Principal`.

PIA-011A exposes Application `BeginAuthentication` and
`CompleteAuthentication` only on the permission-protected Application Unix
listener. The public SDK requires a typed `SessionSigner`, a pinned Node
Principal, and that Unix socket. It validates every structured challenge field,
keeps the resulting session secret in process memory, coalesces concurrent
login attempts, retries a product unary call exactly once after
`Unauthenticated`, and never treats `PermissionDenied` as a login signal.
No protected-file Application signer is supplied by this leaf; embedding
Applications retain key custody. Plaintext loopback/TCP never carries a
Principal session.

PIA-011B adds a separately named 32-byte `ApplicationEnrollmentTicket`; it is
never parsed as `application-token`, a Principal, a session, or a grant. A
protected Operator call authorized by the existing exact
`identity.principal.enroll` action issues one ten-minute ticket bound to the
Node, the exact prospective Application Principal, a sorted subset of
`application.content.get/put`, and the currently configured legacy Application
installation. Only a domain-separated SHA-256 ticket digest is
durable. The Application proves root possession through the Application
Begin/Complete enrollment purpose, then calls `EnrollApplication` without an
Authorization header on the protected Application Unix listener.

`identity-access.db` schema version 6 adds
`identity-application-enrollment-tickets-v1` and
`identity-application-legacy-installations-v1`. Ticket consumption, root-key
enrollment, the canonical authenticate-only Device Credential, a Node-signed
Application-audience grant with principal-owned scope, and the exact
legacy-installation disable tombstone commit in one transaction. The grant is
finite and contains only the ticket's exact sorted actions. Corrupt records,
replay, cross-Node/interface/peer/principal presentation, expiry at the exact
boundary, and storage failure all fail closed. A transaction failure leaves the
ticket active and legacy state unchanged; the ephemeral EnrollmentProof must be
re-created after restart or a failed attempt.

The public SDK exposes a distinct typed `EnrollmentSigner` and an opaque,
redacted `ApplicationEnrollmentTicket`. It validates the pinned Node, exact
Application Audience, Unix profile, peer binding, purpose, timestamps,
Principal, Credential, and final Principal/Credential/Grant metadata before
returning; it clears all ticket, proof, signature, and Credential wire copies.
The Operator CLI writes a newly issued ticket only to a new protected file and
never prints it. There is no built-in Application root/file signer.

The `application_interface.principal_enrollment_enabled` configuration field is
default-false and deliberately rejected until PIA-014 supplies durable,
owner-aware Principal Blob/content access. PIA-012 admission alone is not
sufficient: knowledge of a CID is not ownership proof or authorization to read.
This staged activation is required because a replacement enrollment atomically
disables the exact legacy token; enabling it earlier would lock the Application
out of content. The code and protocol are testable
with an injected gate, while production provisioning remains legacy-compatible.
PIA-017, not this leaf, owns the durable five-state surface retirement machine,
deadline, default change, and final credential removal.

Challenges are memory-only, exactly 120 seconds, atomic one-use, capped at
4,096 per Node and eight per normalized source, and rate-limited by the v1
10/minute token bucket with burst eight. Sessions are memory-only, contain no
authority, default to 15 minutes, and are capped at 16,384 globally and 16 per
`(source, Principal, Audience)`. A 32-byte secret is returned once; the service
stores only its process-keyed HMAC and non-secret session facts. Restart creates
new HMAC keys and invalidates sessions and one-use EnrollmentProof values.

`identity-access.db` schema version 2 adds
`identity-device-revocations-v1`, keyed by exact `(Audience Node, Subject,
DeviceID)`. Credential presentation alone performs no durable write. Complete,
session presentation, and durable Device revocation share a device-lifecycle
linearization boundary, so a committed revocation cannot race a newly inserted
session. Revocation remains Node-local and blocks renewed Credentials for the
same DeviceID.

## Direct Admission

PIA-007 upgrades `identity-access.db` to schema version 3 and adds separate
versioned buckets for Principal enrollment metadata, canonical Access Grants,
the grant lookup index, and permanent Access Grant revocations. Enrollment
records bind the exact `(Node, Principal)` key to the root public key and a
canonical whole-second enrollment time; loading re-derives the Principal.
Grant records contain the canonical signed artifact and issuer public key.
Every indexed lookup reloads and verifies the signature, self-certifying
issuer, artifact ID, complete Audience tuple, and index hash/key binding.

`Admit` implements only direct calls. It authenticates the memory-only session,
then reloads current durable Device and grant revocation state and requires one
matching grant for the exact Node, interface, protocol major, action, and
resource scope. Sessions contain no actions or permissions. The only v1 scopes
are `node`, `principal-owned`, and the complete exact
`(Node, Owner, ResourceKind, CanonicalID)` tuple. Success returns immutable
facts with `Actor == Effective`; product Policy remains a separate subsequent
decision. A Delegation presentation is rejected as unsupported until PIA-013.
Unknown actions/resources, corrupt records, and storage failures fail closed.

PIA-008A adds schema version 4 with the single
`identity-bootstrap-tickets-v1` bucket. Bootstrap Tickets remain disabled by
default. When explicitly enabled, issuance returns one 32-byte secret once and
persists only a domain-separated SHA-256 digest, canonical issue/expiry seconds,
and an active/consumed state. There is at most one active ticket per unenrolled
Node; it expires after exactly ten minutes and may then be replaced. Consumption
is a single write transaction and persists a terminal tombstone, so replay and
concurrent consumption fail after one winner. The transaction helper is the
seam PIA-008B will extend with proof-bound enrollment and first-grant issuance;
PIA-008A does not expose a listener or change the legacy bootstrap default.

PIA-008B adds the owner operation for first Principal enrollment, still without
a listener or default-flow switch. The adapter supplies the current trusted
`AuthenticationBinding` separately; it must equal the proof Challenge binding
including Node, Operator interface, protocol, transport profile, and peer. The
operation verifies the root-bound Principal, canonical initial Credential, and
one-use typed EnrollmentProof before invoking a typed Node `AccessGrantIssuer`.
That interface can issue only an Access Grant payload and is not a generic byte
signer. The returned artifact is independently checked for exact payload
equality, canonical signature, issuer key, ID, and index binding.

The first grant is closed and deterministic: `node` scope, 30-day validity, and
only the six exact identity enrollment/device/grant administration actions. It
contains no wildcard. Ticket consumption, enrollment metadata, canonical grant,
and grant index commit in one `identity-access.db` write transaction; any later
write error rolls back all four. The standalone ticket-consume API remains
absent, so no caller can burn the recovery path without atomic enrollment.

PIA-008C upgrades `identity-access.db` to schema version 5. It adds canonical
enrolled-Credential records, keyed by the complete
`(Node, Principal, DeviceID, CredentialID)` tuple, and durable administration
idempotency records. Idempotency records are versioned and bind the stable
semantic request digest to a typed result prefix and checksum; changed payloads
conflict and any result corruption fails closed.

Every administration mutation repeats session, Device revocation, grant and
grant-revocation checks inside the same write transaction as its mutation and
recaptures the clock after acquiring that transaction. Grant proposal authority
uses lowercase unpadded base32 SHA-256 under
`ardents:grant-proposal:v1\x00` over the canonical length-prefixed subject,
Audience, sorted actions, scope and validity tuple. Device mutation authority
uses the same pattern under `ardents:device-resource:v1\x00` for the exact
Subject and DeviceID while ResourceRef supplies the exact Node.

Grant and Device revocation refuse to remove the final live recovery path. A
path requires both a current, unrevoked Operator grant containing all six exact
recovery actions and a current enrolled Credential whose DeviceID is not
revoked. Replacement grants/devices must already be durably visible in the same
snapshot. Lists require a mandatory Subject and return only verified non-secret
metadata. Mutation successes and all protected denials produce redacted audit
events; successful read-only lists do not produce mutation events.

## Services, Paths, And Limits

Both identity services use the method names `BeginAuthentication` and
`CompleteAuthentication`. Operator bootstrap uses the distinct public-bounded
`EnrollFirstPrincipal`; authenticated administration uses `EnrollPrincipal`,
`RevokeDevice`, `ListDeviceRevocations`, `IssueAccessGrant`,
`RevokeAccessGrant`, `ListAccessGrants`, and
`IssueApplicationEnrollmentTicket`; Application enrollment adds the distinct
`EnrollApplication` method. Their Connect paths are the normal
fully-qualified paths under `/ardents.v1.IdentityService/` and
`/ardents.application.v1.IdentityService/` respectively. Keeping a focused
ten-method Operator service and a three-method Application service stays below
the twelve-RPC budget.

PIA-009A registers the Operator service only in the permission-protected Unix
mux. Authentication and feature-gated first enrollment are explicit
`public_bounded` catalogue entries; every administration method has one exact
action and a server-derived canonical resource. Unknown procedures fail
closed. The adapter accepts exactly one unpadded base64url
`ArdentsOperatorSession` value decoding to 32 bytes. Bearer,
Application-session, malformed, multiple, padded, and oversized values fail
without fallback. Linux derives a per-connection binding from kernel
`SO_PEERCRED` normalized UID plus the configured socket path. PID and GID are
excluded so sessions and per-source limits remain stable across CLI processes
owned by the same OS user. Platforms without a
reviewed peer-credential API use the explicit shared-listener fallback: a
separately domain-separated binding derived from the socket path. The fallback
reduces per-process isolation but never removes
Node/interface/protocol/transport/peer checks.

PIA-009B1 migrates the Node and Configuration service families on that Unix
listener. Their Principal interceptor derives the action from the single
Operator procedure catalogue, derives an exact Node or Configuration resource,
passes the request-only `ResourceTarget` to `AdmitTarget` once, and places the
sealed `AuthorizedCall` in request context. `AdmitTarget` authenticates the
session and derives Actor/Effective before invoking the server-owned Finalize
function that creates the canonical `ResourceRef`; request fields cannot supply
Actor, Effective, Node, or Owner.
Unary and streaming handlers require that context and never parse Authorization
or repeat authorization. The streaming request is decoded and rejected for
unknown fields before admission; the resulting context is fixed at
establishment. During migration an explicit scheme router
keeps `Bearer` on the legacy interceptor and every value claiming
`ArdentsOperatorSession` on the Principal interceptor; malformed or failed
Principal presentation can never downgrade to bearer. Every accepted legacy
credential presentation emits a redacted migration audit record. PIA-009B2
extends the same path to Network and Diagnostics, PIA-009B3 extends it to
Workload, and PIA-009B4 completes the Operator handler migration with
Content/Transfer/Retention. The explicit legacy scheme remains a separate
migration path until PIA-017; a Principal failure never selects it.

The two tuple-shaped B2 exact resources use target-module-owned canonical IDs;
access control never joins fields with a delimiter. A discovery-record lookup
ID is `drr1_ || base64url_no_pad(SHA-256("ardents:discovery-record-resource:v1\x00" ||
u32be(len(kind)) || kind || u32be(len(subject)) || subject))`. A diagnostic
subject ID is `dsr1_ || base64url_no_pad(SHA-256("ardents:diagnostic-subject-resource:v1\x00" ||
u32be(len(scope)) || scope || u32be(len(resource_id)) || resource_id))`.
Discovery tuple parts are non-empty printable ASCII and at most 512 bytes each.
Diagnostic resource IDs may be empty for subsystem-wide explanations, otherwise
use the same printable-ASCII/512-byte rule. Diagnostic scope is one of `boot`,
`configuration`, `data`, `diagnostics`, `discovery`, `identity_access`,
`network`, `node`, `operator_access`, `policy`, `service`, `transport`, or
`workload`. Recent-event limits are `0..1000`; a non-empty cursor is the unique
positive canonical decimal representation of an `int64` sequence.
`DiscoveryRecord.source` is response metadata only. Import ignores any supplied
value and assigns the server-owned `imported` source after signature validation.

PIA-009B3 uses the exact, unmodified WorkloadID or ServiceID as the canonical
resource ID. Both must be 1 through 512 bytes of printable non-space ASCII
(`0x21` through `0x7e`); no trimming, case folding, or path normalization is
performed. `RegisterWorkload` reads only `spec.id` during canonicalization.
`spec.owner` is never authoritative and cannot become the ResourceRef owner;
only the server finalizer may attach Effective after a successful Admit.

PIA-009B4 applies the same 1-through-512 printable non-space ASCII rule to
ObjectID, ContentReference, ManifestID, and TransferID. Owner fields in object
or manifest requests are not access authority; owner-required Operator
ResourceRefs use `Owner=Effective` only in the server finalizer. Principal
`PublishObject` and `PublishManifest` require a non-empty canonical declared ID.
Principal `PublishBlob` requires a non-empty payload no larger than 1 MiB. Its
exact resource ID is the payload-derived CID; any declared ID, CID, or hash must
match the derived identity. Payload hashing is deferred inside the single Admit
until session and DeviceID revocation checks succeed. The explicit legacy path
retains its prior metadata-only/server-generated-ID compatibility during the
migration window.
The Operator catalogue has no `principal-owned` action rows: such grant
proposals are rejected, and any previously persisted signed Operator grant with
that scope is ignored during every match. Content ownership scopes remain an
Application-only feature and require the server-owned binding introduced by
PIA-014; PIA-012 admission and stamping a request ID with Effective are not
ownership proof.

Headers are exactly `Authorization: ArdentsOperatorSession <base64url-secret>`,
`Authorization: ArdentsApplicationSession <base64url-secret>`, and optional
`Ardents-Delegation: <base64url-canonical-delegation>`. Authorization has
exactly one value and at most 128 encoded bytes. Delegation has zero or one
value, at most 22 KiB encoded and 16 KiB decoded. Public errors map to
Unauthenticated, PermissionDenied, InvalidArgument, AlreadyExists/Conflict,
ResourceExhausted, Unavailable, and Internal without secret details.

All v1 count, time, size, rate, timestamp, and skew limits are the normative
constants in product design section 4.7; adapters may only configure lower
limits. Their single protocol owner is `api/ardents/identity/v1/contract.go`;
server `identity/access`, the public SDK domain codec, and adapters must not
define parallel defaults or catalogues.

Principal authentication endpoints are exposed only on permission-protected
Unix sockets. Plaintext loopback HTTP is legacy-only. Operator remote access
uses OpenSSH stream-local forwarding to the protected Operator socket; `ssh -W`
to loopback HTTP is forbidden for Principal sessions. Non-loopback plaintext
TCP is forbidden. A future TCP path requires a separately approved mTLS
transport profile.

## CLI Signer Custody And Command Names

PIA-010A freezes the local, offline `ardentsctl` identity hierarchy as:

```text
identity principal create [--signer-file PATH]
identity principal import --from-file PATH [--signer-file PATH]
identity principal show [--signer-file PATH]
identity device create [--root-signer-file PATH] [--signer-file PATH] [--valid-for DURATION]
identity device show [--signer-file PATH]
```

These commands never resolve a Node context, require a bearer, connect to a
listener, or perform enrollment. `--output json` is deterministic and
noninteractive. The default root and device paths are respectively
`<os.UserConfigDir>/ardents/identity/principal-root-v1.json` and
`<os.UserConfigDir>/ardents/identity/device-v1.json`. `--valid-for` defaults to
90 days and may not exceed the v1 Key Credential hard lifetime. Import accepts
only another valid protected `principal-root` bundle; raw seed text and opaque
wallet formats are not implicit import formats.

PIA-010B implements `identity login|status|logout`. `login` proves connectivity
and authentication for the current process; `status` and `logout` inspect or
clear only that process-local cache. No session helper daemon or persistent
session file is implied. PIA-010C implements the frozen `identity enroll`,
`identity grant list|issue|revoke`, and `identity device revoke` commands over
the protected Operator transport. Bootstrap Ticket input is a protected file,
never argv; subsequent enrollment and all grant/revocation RPCs use a current
Principal session.

The CLI session cache key is the typed tuple `(Node Principal,
Interface_OPERATOR, protocol major 1, signer Principal)`. A configured client
owns one effective protected transport, so a peer-bound session cannot move to
another local socket or SSH tunnel. Concurrent misses and refreshes singleflight.
At `now >= ExpiresAt` an entry is unusable. `Unauthenticated` evicts only the
failed entry generation, reauthenticates once, and replays once; a second
`Unauthenticated` and every `PermissionDenied`, transport, cancellation, or
unknown failure return without another retry. Server streams may replay their
single request only when authentication fails before the first event.

Principal mode requires a canonical pinned target Node Principal, the protected
device signer, and either `unix:///absolute/operator.sock` or
`--ssh-operator-socket /absolute/remote/operator.sock`. Remote Principal mode
starts one managed OpenSSH `-N -T` tunnel with
`ExitOnForwardFailure=yes` and a private local stream socket; it never uses
`ssh -W`, a remote shell helper, or loopback TCP. Effective HTTP targets reject
before Begin/Complete, even for loopback. Legacy bearer selection is explicit
through `--legacy-token`, `--legacy-token-file`, the corresponding
`ARDENTS_LEGACY_*` environment variables, or `legacy_token_*` context fields;
mixing schemes is invalid and every source emits a warning without the
credential value. Old ambient CLI token names are ignored rather than allowed
to downgrade the default Principal mode. Principal context fields are
`signer_file` and `ssh_operator_socket`.

Signer files are strict JSON objects no larger than 32 KiB. Version is exactly
`1`, algorithm is exactly `ed25519`, binary fields use canonical unpadded
base64url, and duplicate, unknown, trailing, missing, oversized, or mismatched
state is rejected. The root bundle kind is `principal-root` with exactly
`version`, `kind`, `algorithm`, `principal`, `root_public_key`, and
`root_private_seed`. The device bundle kind is `device` with exactly `version`,
`kind`, `algorithm`, `principal`, `root_public_key`, `device_id`,
`device_public_key`, `device_private_seed`, and canonical `credential` bytes.
The device Credential is finite, authenticate-only, root-signed, and must bind
every repeated Principal/root/device field exactly.

Root and device custody remain separate: routine session signing opens only the
device bundle, while device Credential issuance and enrollment proof are the
explicit root-key operations. The CLI exposes only the typed `SessionSigner`
and `EnrollmentSigner` methods from product design; no public generic
`Sign([]byte)` exists. Creation/import is atomic no-replace. On Unix directories
are `0700`, files are `0600`, and permissive existing key files are refused. On
Windows the existing protected-DACL contract grants only the current user and
SYSTEM instead of pretending Unix mode bits apply. A missing immediate signer
directory is created private; an existing directory or signer file is validated
without permission/ACL mutation and refused if unsafe. Temporary files receive
their final private protection before the first secret byte is written.
Human/JSON output contains
only public IDs, public keys, Credential ID, and validity; private seeds,
canonical Credential bytes, signatures, paths, and file contents are never
rendered.

## Credential Compatibility Matrix

| Presented scheme | Operator Unix | Operator loopback HTTP | Application Unix | Application loopback HTTP |
|---|---:|---:|---:|---:|
| legacy `Bearer` | explicit migration state only | explicit migration state only | reject | reject |
| legacy `ArdentsApplication` | reject | reject | explicit migration state only | explicit migration state only |
| `ArdentsOperatorSession` | accept when Principal path enabled | reject | reject | reject |
| `ArdentsApplicationSession` | reject | reject | accept when Principal path enabled | reject |

Multiple schemes, a malformed selected scheme, or a failed Principal session
always reject without fallback. Bootstrap and break-glass credentials are
separate schemes and are not represented by this normal-credential matrix.

## Resource Catalogue

Every row is protected unless marked `public_bounded`. `C` is the request-only
canonicalizer and `F` is finalization after Admit determines Actor/Effective.
Malformed values deny before mutation; every row requires a sibling-action
denial test. Accepted scopes are `node`, `principal-owned`, and/or `exact` as
listed; unknown scope/resource/action values deny.

| Procedures | Action(s) | Class | ResourceKind and C/F contract | Scopes |
|---|---|---|---|---|
| Identity Begin/Complete on each surface and Application EnrollApplication | public_bounded | read/write | no grant resource; listener derives binding; EnrollApplication additionally consumes the separately typed ticket and proof | none |
| StartNode, StopNode, GetNodeStatus, GetNodeCapabilities, GetNodeRuntime, StreamNodeEvents | `node.start`, `node.stop`, `node.status`, `node.capabilities`, `node.runtime`, `node.events` | per existing catalogue | `node`; C empty, F audience Node | node |
| GetEffectiveConfiguration, ReloadConfiguration | `config.effective`, `config.reload` | read/write | `configuration`; C empty, F audience Node singleton | node, exact |
| GetNetworkStatus, ListRouteCandidates | `transport.network_status`, `transport.route_candidates` | read | `network`; C empty, F audience Node | node, exact |
| GetDiscoveryStatus, GetLocalPresence, ListPeers, ListRecords | `discovery.status`, `discovery.local_presence`, `discovery.peers`, `discovery.list_records` | read | corresponding Node collection; C validates paging only, F audience Node | node, exact |
| ResolveRecord | `discovery.resolve_record` | read | `discovery-record`; C strict kind+subject, F audience Node | node, exact |
| ResolveService | `discovery.resolve_service` | read | `service`; C strict ServiceID, F audience Node | node, exact |
| ImportRecord | `discovery.import` | write | kind-specific record; C validates signed record and derives canonical ID before import, F audience Node | node, exact |
| Register/Start/Stop/Restart/GetWorkloadStatus | `workload.register`, `workload.start`, `workload.stop`, `workload.restart`, `workload.status` | per existing catalogue | `workload`; C strict spec ID/request ID, F audience Node | node, exact |
| ListWorkloads | `workload.list` | read | `workload-collection`; C paging only, F audience Node | node, exact |
| GetHostedService, GetServicePublicationStatus | `workload.hosted_service`, `workload.service_publication` | read | `service`; C strict ServiceID, F audience Node | node, exact |
| ListHostedServices | `workload.hosted_services` | read | `service-collection`; C paging only, F audience Node | node, exact |
| PublishObject/GetObject | `data.publish_object`, `data.get_object` | write/read | `content-object`; C strict non-empty canonical object ID, F audience Node | node, exact |
| ListObjects | `data.list_objects` | read | `content-object-collection`; C paging only, F audience Node | node, exact |
| PublishBlob | `data.publish_blob` | write | `content-blob`; C computes bounded payload reference and validates declared ID/CID equality, F audience Node | node, exact |
| GetBlob/FetchBlob/RetainBlob/PinBlob/DropBlob/ListBlobSources | `data.get_blob`, `data.fetch_blob`, `data.retain_blob`, `data.pin_blob`, `data.drop_blob`, `data.blob_sources` | per existing catalogue | `content-blob`; C strict ContentReference, F audience Node | node, exact |
| ListBlobs | `data.list_blobs` | read | `content-blob-collection`; C paging only, F audience Node | node, exact |
| GetTransfer | `data.get_transfer` | read | `transfer`; C strict TransferID, F audience Node | node, exact |
| ListTransfers | `data.list_transfers` | read | `transfer-collection`; C paging only, F audience Node | node, exact |
| PublishManifest/GetManifest | `data.publish_manifest`, `data.get_manifest` | write/read | `content-manifest`; C strict non-empty canonical manifest ID, F audience Node | node, exact |
| ListManifests/GetDataInventory | `data.list_manifests`, `data.inventory` | read | corresponding content collection; C paging only, F audience Node | node, exact |
| GetDiagnostics/GetHealthSummary | `diagnostics.snapshot`, `diagnostics.health_summary` | read | `diagnostics`; C empty, F audience Node | node, exact |
| GetPendingOperations | `diagnostics.pending_operations` | read | `operation-collection`; C paging only, F audience Node | node, exact |
| ExplainFailure | `diagnostics.explain_failure` | read | `diagnostic-subject`; C closed scope + strict resource ID, F audience Node | node, exact |
| ListRecentEvents | `diagnostics.recent_events` | read | `event-collection`; C validates cursor/limit, F audience Node | node, exact |
| Application Put | `application.content.put` | write | `content-owner`; C validates bounded payload, F owner=Effective and audience Node | principal-owned |
| Application Get | `application.content.get` | read | `owned-content`; C strict ContentReference, F owner=Effective and audience Node | principal-owned, exact |
| EnrollPrincipal | `identity.principal.enroll` | write | `principal`; C strict PrincipalID plus proof, F audience Node | exact |
| IssueApplicationEnrollmentTicket | `identity.principal.enroll` | write | `principal`; C strict prospective Application Principal and closed Application action subset, F audience Node | exact |
| RevokeDevice | `identity.device.revoke` | write | `device`; C strict PrincipalID+DeviceID and binding proof, F audience Node | exact |
| ListDeviceRevocations | `identity.device-revocations.list` | read | `device-revocation-collection`; C mandatory PrincipalID filter, F audience Node | exact |
| IssueAccessGrant | `identity.grant.issue` | write | `grant-proposal`; C canonical proposal hash, F audience Node | exact |
| RevokeAccessGrant | `identity.grant.revoke` | write | `access-grant`; C strict AccessGrantID, F audience Node | exact |
| ListAccessGrants | `identity.grant.list` | read | `grant-collection`; C mandatory subject PrincipalID, F audience Node | exact |

No canonicalizer trims, case-folds, path-cleans, queries product state, trusts a
request owner, or falls back to the Node resource. Adding any procedure requires
one catalogue row, one extractor, malformed-input coverage, and sibling denial
in the same change.
