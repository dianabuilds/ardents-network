# Principal Identity Protocol Contract

Status: frozen Principal-only v1 implementation contract.

This document materializes the protocol, transport, generation, and resource
catalogue decisions in the Principal Identity and Access product design.
Operator and Application identity services are enabled only on their respective
protected Unix listeners. Both product surfaces use Principal sessions on their
normal paths; Application content admission uses the durable owner-aware
Blob/content boundary.

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

The transport-independent owner is `internal/identity/access`. The Operator and
Application adapters remain separate. `Begin` accepts only a server-derived
`SourceKey` and complete
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

Application `BeginAuthentication` and `CompleteAuthentication` are exposed only
on the permission-protected Application Unix
listener. The public SDK requires a typed `SessionSigner`, a pinned Node
Principal, and that Unix socket. It validates every structured challenge field,
keeps the resulting session secret in process memory, coalesces concurrent
login attempts, retries a product unary call exactly once after
`Unauthenticated`, and never treats `PermissionDenied` as a login signal.
No protected-file Application signer is supplied; embedding
Applications retain key custody. Plaintext loopback/TCP never carries a
Principal session.

`ApplicationEnrollmentTicket` is a separately typed 32-byte one-use secret; it
is never parsed as a Principal, Credential, session, or grant. A protected
Operator call authorized by the exact
`identity.principal.enroll` action issues one ten-minute ticket bound to the
Node, the exact prospective Application Principal, a sorted subset of
`application.content.get/put`. Only a domain-separated SHA-256 ticket digest is
durable. The Application proves root possession through the Application
Begin/Complete enrollment purpose, then calls `EnrollApplication` without an
Authorization header on the protected Application Unix listener.

The canonical fresh-install `identity-access.db` schema is version 1.
`identity-application-enrollment-tickets-v1` stores the ticket digest and its
exact bindings. Ticket consumption, root-key enrollment, the canonical
authenticate-only Device Credential, and a finite Node-signed
Application-audience grant with principal-owned scope commit in one
transaction. The grant contains only the ticket's exact sorted actions. Corrupt
records, replay, cross-Node/interface/peer/principal presentation, expiry at the
exact boundary, and storage failure all fail closed. A transaction failure
leaves the ticket active and writes no partial enrollment or grant; the
ephemeral EnrollmentProof must be re-created after restart or a failed attempt.

The public SDK exposes a distinct typed `EnrollmentSigner` and an opaque,
redacted `ApplicationEnrollmentTicket`. It validates the pinned Node, exact
Application Audience, Unix profile, peer binding, purpose, timestamps,
Principal, Credential, and final Principal/Credential/Grant metadata before
returning; it clears all ticket, proof, signature, and Credential wire copies.
The Operator CLI writes a newly issued ticket only to a new protected file and
never prints it. There is no built-in Application root/file signer.

Application enrollment is available whenever the Application Interface itself
is enabled. Owner-aware content access is mandatory: knowledge of a CID is not
ownership proof or authorization to read. The Application session path is the
only normal Application credential branch.

Challenges are memory-only, exactly 120 seconds, atomic one-use, capped at
4,096 per Node and eight per normalized source, and rate-limited by the v1
10/minute token bucket with burst eight. Sessions are memory-only, contain no
authority, default to 15 minutes, and are capped at 16,384 globally and 16 per
`(source, Principal, Audience)`. A 32-byte secret is returned once; the service
stores only its process-keyed HMAC and non-secret session facts. Restart creates
new HMAC keys and invalidates sessions and one-use EnrollmentProof values.

The schema contains `identity-device-revocations-v1`, keyed by exact
`(Audience Node, Subject, DeviceID)`. Credential presentation alone performs no
durable write. Complete,
session presentation, and durable Device revocation share a device-lifecycle
linearization boundary, so a committed revocation cannot race a newly inserted
session. Revocation remains Node-local and blocks renewed Credentials for the
same DeviceID.

## Direct Admission

The same fresh-install schema contains separate versioned buckets for Principal
enrollment metadata, canonical Access Grants, the grant lookup index, and
permanent Access Grant revocations. Enrollment
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
decision. A valid one-hop Application Delegation changes only Effective after
the full delegatee, audience, action, scope, time, Credential, signature, and
revocation checks. Unknown actions/resources, corrupt records, and storage
failures fail closed.

The `identity-bootstrap-tickets-v1` bucket stores first-enrollment ticket
records. Issuance returns one 32-byte secret once and persists only a
domain-separated SHA-256 digest, canonical issue/expiry seconds, and an
active/consumed state. There is at most one active ticket per unenrolled Node;
it expires after exactly ten minutes and may then be replaced. Consumption
commits with proof-bound enrollment and first-grant issuance in one transaction;
replay and concurrent consumption fail after one winner.

For first Principal enrollment, the adapter supplies the current trusted
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

The schema also contains canonical enrolled-Credential records, keyed by the complete
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

Both identity services use the method names `BeginAuthentication`,
`CompleteAuthentication`, and `EndSession`. Operator bootstrap uses the
distinct public-bounded
`EnrollFirstPrincipal`; authenticated administration uses `EnrollPrincipal`,
`RevokeDevice`, `ListDeviceRevocations`, `IssueAccessGrant`,
`RevokeAccessGrant`, `ListAccessGrants`,
`IssueApplicationEnrollmentTicket`, and `ImportDelegationRevocation`;
Application enrollment adds the distinct `EnrollApplication` method. Their
Connect paths are the normal
fully-qualified paths under `/ardents.v1.IdentityService/` and
`/ardents.application.v1.IdentityService/` respectively. The focused
twelve-method Operator service and four-method Application service remain
within the protocol budget.

The Operator service is registered only in the permission-protected Unix mux.
Authentication and first enrollment are explicit
`public_bounded` catalogue entries; every administration method has one exact
action and a server-derived canonical resource. Unknown procedures fail
closed. The adapter accepts exactly one unpadded base64url
`ArdentsOperatorSession` value decoding to 32 bytes. Any other scheme,
Application-session, malformed, multiple, padded, and oversized value fails
closed. Linux derives a per-connection binding from kernel
`SO_PEERCRED` normalized UID plus the configured socket path. PID and GID are
excluded so sessions and per-source limits remain stable across CLI processes
owned by the same OS user. Platforms without a
reviewed peer-credential API use the explicit shared-listener fallback: a
separately domain-separated binding derived from the socket path. The fallback
reduces per-process isolation but never removes
Node/interface/protocol/transport/peer checks.

All Operator service families on that Unix listener use the Principal
interceptor. It derives the action from the single
Operator procedure catalogue, derives an exact Node or Configuration resource,
passes the request-only `ResourceTarget` to `AdmitTarget` once, and places the
sealed `AuthorizedCall` in request context. `AdmitTarget` authenticates the
session and derives Actor/Effective before invoking the server-owned Finalize
function that creates the canonical `ResourceRef`; request fields cannot supply
Actor, Effective, Node, or Owner.
Unary and streaming handlers require that context and never parse Authorization
or repeat authorization. The streaming request is decoded and rejected for
unknown fields before admission; the resulting context is fixed at
establishment. There is one credential parser on this listener. A malformed or
failed Principal presentation is denied without invoking another authenticator.

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
Discovery uses only the canonical versioned kind-specific envelope containing
exactly one `NodeFacts` or `ServiceFacts` body. Flat or contradictory identity
fields fail closed. `DiscoveryRecord.source` is response metadata only. Import
ignores any supplied value and assigns the server-owned `imported` source after
signature and kind-specific validation.

The exact, unmodified WorkloadID or ServiceID is the canonical
resource ID. Both must be 1 through 512 bytes of printable non-space ASCII
(`0x21` through `0x7e`); no trimming, case folding, or path normalization is
performed. `RegisterWorkload` reads only `spec.id` during canonicalization.
`spec.owner` is never authoritative and cannot become the ResourceRef owner;
only the server finalizer may attach Effective after a successful Admit.

The same 1-through-512 printable non-space ASCII rule applies to
ObjectID, ContentReference, ManifestID, and TransferID. Owner fields in object
or manifest requests are not access authority; owner-required Operator
ResourceRefs use `Owner=Effective` only in the server finalizer. Principal
`PublishObject` and `PublishManifest` require a non-empty canonical declared ID.
Principal `PublishBlob` requires a non-empty payload no larger than 1 MiB. Its
exact resource ID is the payload-derived CID; any declared reference or hash
must match the derived identity. Payload hashing is deferred inside the single
Admit until session and DeviceID revocation checks succeed.
The Operator catalogue has no `principal-owned` action rows: such grant
proposals and signed Operator grants are rejected, and that scope is never
matched on the Operator surface. Content ownership scopes remain an
Application-only feature and require the server-owned durable binding; admission
or stamping a request ID with Effective is not ownership proof.

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
Unix sockets. Plaintext loopback HTTP exposes neither authentication nor
protected product calls. Operator remote access
uses OpenSSH stream-local forwarding to the protected Operator socket; `ssh -W`
to loopback HTTP is forbidden for Principal sessions. Non-loopback plaintext
TCP is forbidden. A future TCP path requires a separately approved mTLS
transport profile.

## CLI Signer Custody And Command Names

The local, offline `ardentsctl` identity hierarchy is:

```text
identity principal create [--signer-file PATH]
identity principal import --from-file PATH [--signer-file PATH]
identity principal show [--signer-file PATH]
identity device create [--root-signer-file PATH] [--signer-file PATH] [--valid-for DURATION]
identity device show [--signer-file PATH]
```

These commands never resolve a Node context, connect to a listener, authenticate
a call, or perform enrollment. `--output json` is deterministic and
noninteractive. The default root and device paths are respectively
`<os.UserConfigDir>/ardents/identity/principal-root-v1.json` and
`<os.UserConfigDir>/ardents/identity/device-v1.json`. `--valid-for` defaults to
90 days and may not exceed the v1 Key Credential hard lifetime. Import accepts
only another valid protected `principal-root` bundle; raw seed text and opaque
wallet formats are not implicit import formats.

`identity login|status|logout` manages the process-local session cache. `login`
proves connectivity
and authentication for the current process; `status` and `logout` inspect or
clear only that process-local cache. No session helper daemon or persistent
session file is implied. The frozen `identity enroll`,
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

Every protected CLI call requires a canonical pinned target Node Principal, the
protected device signer, and either `unix:///absolute/operator.sock` or
`--ssh-operator-socket /absolute/remote/operator.sock`. Remote Principal mode
starts one managed OpenSSH `-N -T` tunnel with
`ExitOnForwardFailure=yes` and a private local stream socket; it never uses
`ssh -W`, a remote shell helper, or loopback TCP. Effective HTTP targets reject
before Begin/Complete, even for loopback. The canonical context fields are
`signer_file` and `ssh_operator_socket`; unknown fields and alternate credential
inputs fail closed.

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

## Credential Acceptance Matrix

| Presented scheme | Operator Unix | Operator loopback HTTP | Application Unix | Application loopback HTTP |
|---|---:|---:|---:|---:|
| `ArdentsOperatorSession` | accept | reject | reject | reject |
| `ArdentsApplicationSession` | reject | reject | accept | reject |
| any other or unknown scheme | reject | reject | reject | reject |

Multiple schemes, a malformed selected scheme, or a failed Principal session
always reject without fallback. Bootstrap and Application Enrollment Tickets
are accepted only by their exact public-bounded enrollment methods and are not
normal call credentials.

## Resource Catalogue

Every row is protected unless marked `public_bounded`. `C` is the request-only
canonicalizer and `F` is finalization after Admit determines Actor/Effective.
Malformed values deny before mutation; every row requires a sibling-action
denial test. Accepted scopes are `node`, `principal-owned`, and/or `exact` as
listed; unknown scope/resource/action values deny.

| Procedures | Action(s) | Class | ResourceKind and C/F contract | Scopes |
|---|---|---|---|---|
| Identity Begin/Complete on each surface, Application EnrollApplication, and Operator ImportDelegationRevocation | public_bounded | read/write | no grant resource; listener derives binding; enrollment additionally consumes the separately typed ticket and proof; revocation import verifies one canonical owner-signed artifact | none |
| Identity EndSession on each surface | session_lifecycle | write | no grant resource; requires and invalidates the exact surface- and transport-bound session | none |
| StartNode, StopNode, GetNodeStatus, GetNodeFeatures, GetNodeRuntime, StreamNodeEvents | `node.start`, `node.stop`, `node.status`, `node.features`, `node.runtime`, `node.events` | per existing catalogue | `node`; C empty, F audience Node | node |
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
