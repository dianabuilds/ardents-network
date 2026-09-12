# Endpoint and Service runtime

Status: **current maintained technical contract.** This document describes the
local Endpoint, generic Broker, Service publication, and Service Connection
Modules that exist in the repository. It does not select a supported desktop
profile, a qualified Application Isolation profile, a public Service protocol,
or a complete Route/Node qualification.

Although its directory is under `internal/application`, the Broker is
Network-owned because the maintained headless Endpoint uses it for local-grant
admission and session lifecycle. The sibling `interfacev1` directory has the
distinct `application-interface-v1` owner: it freezes the local protocol used
on both sides without owning either Network behavior or Browser presentation.

The selected closed successor's [workload](../product/protected-service-workload.md),
[confinement](application-confinement.md) and [protocol](protected-route-protocol.md)
own its future Endpoint composition under ADR-0081. The current runtime facts
below remain until explicit migration; generic callers do not acquire a
qualified-launch receipt through compatibility.

The successor text Publisher context independently owns its Introduction and
Responder prefixes and its Introduction registration. It consumes the existing accepted Instance binding to commit
an Instance-signed private Descriptor through the actual resolution duty. The
Instance owns a fresh volatile recipient and its monotonic revision floor;
registration loss erases the recipient, and context loss joins operations before
withdrawing Publication and Instance. A committed Publication retains cleanup
ownership even when cancellation prevents Lease handover. Failed withdrawal
retains its binding and original error until cleanup completes. This tested
composition consumes real recipient-confidential capsule delivery before opening
its separate Responder-domain forwarding prefix. Both Publisher prefixes obtain
genuine tokens through Source and share admission/cleanup code while retaining
separate selections and transports. Known Node/key/family overlaps across live
domains or subroles are excluded before selection and issuance; losing a member
cannot resample a retained set. A final handover rechecks the exact live
Responder owner. Worker loss preserves a surviving context's allocation, while
context loss joins both prefixes. Its refresh scheduler retains the old
published registration while the replacement Descriptor awaits acknowledgement;
the new registration cannot accept a capsule until that acknowledgement is
verified against its still-live context, Instance, channel and profile. Network
publication does not hold the shared Publisher mutex: a checked context
reservation retains exclusive Instance ownership against legacy publication
operations. Only the first successful switch bounds predecessor overlap to
60 seconds or its earlier signed expiry; exact retries retain that cutoff.
Scheduler timing is tested with accelerated events, not a wall-clock lifetime
qualification. These module paths do not establish complete command exposure
or installed-worker qualification.

The closed text-Service composition retains a bounded job-owned exchange through
JOIN and the authenticated Service transport's final cleanup. It prepares the
independent Rendezvous class-2 and Introduction class-1 stocks before concurrent
Source JOIN and capsule submission; each exact HELLO still requires its own
durable token transfer. Publisher JOIN consumes its independently accepted capsule
and current Responder prefix. The worker receives no raw JOIN stream: the existing
Service TLS and native Instance authentication precede Application I/O.

A clean JOIN peer CLOSE may precede consumption of the final authenticated
Service record. The client retains those bounded received bytes and their original
queue reservation until consumed, explicitly closed, cancelled, or expired under
the original data lifetime. Only a clean transport EOF permits that drain; later
protocol failures remain failures. Role TLS EOF alone is not successful outer
completion: joined retirement waits for the peer's successful outer terminal,
retains refusal/transport errors, and joins cleanup before releasing ownership.

## Ownership

The local runtime has separate Modules and Interfaces:

| Module | Interface responsibility | Implementation hidden from callers |
|---|---|---|
| internal/application/broker | Admit and consume one short-lived Local Grant capability for either connection or administration; revoke, drain, and close pending capabilities and active Connection leases; report generic/unqualified. | Capability generation, replay removal, expiry, commitments, admission-load accounting, and grant invalidation. |
| internal/application/interfacev1/connection | Carry one Target Link, one ordered byte stream with explicit directional input close, and exactly one bounded terminal outcome under `ardents-application-interface-v1`; retain the accepted AAI2 bytes and executable conformance vectors. | State, Entry, Target, Route, Credential, Custody, Service keys, retries, fallback, and Network diagnostics. |
| internal/application/interfacev1/administration | Carry one separately authorized `publish` or `withdraw` request and its closed success/unavailable result under the same interface version and vectors. | Connection bytes, publication inputs, Credential/key material, State, Route, Target, and Network diagnostics. |
| internal/endpoint | Compose one role-local participant and implement the shared Connection and Administration Interfaces. `RunParticipant` opens authenticated participant owners, delegates local transports to the Application Modules, and joins shutdown. | Broker consumption, authenticated State/Entry/Target projection, TLS carrier setup, publication acquisition, and Connection invocation. |
| internal/service/publication | Open, publish, acquire, unpublish, and close one exclusive Service Instance generation. | Crash-atomic public record/floor persistence, volatile Instance signer, live-reference accounting, drain, and private-material erasure. |
| internal/service/connection | Carry one logical authenticated Service Connection across fresh Route Attachments, preserve directional Application EOF through its existing authenticated Terminal record, and return one terminal outcome. | Exact Instance challenge/proof, continuity MAC, ordered data/acknowledgement offsets, replay handling, recovery deadline, and attachment cleanup. |

The caller-facing Endpoint seam is role-specific: a Publisher start request
cannot include Route, Credential, signer, or Application facts, and an outbound
connection cannot supply a Publisher binding. This keeps publication ownership, local admission, Route
attachment, and logical-stream recovery out of one mutable request bag.

The maintained Connection Interface adds one narrower consumer operation over
that composition. A headless caller supplies one explicit Target Link; Endpoint
retains the local Connection principal, authenticated State, Entry, Target
authentication, Route inputs, the one-use Transit Grant/key, and the Broker
admission input. After syntactic Target Link parsing and Network binding, Endpoint activates
and consumes the Connection capability before it reads current State, touches
Entry or private reachability, asks an issuer for a Transit Grant, opens Route,
or sends Introduction. Only an authenticated ordered byte stream and bounded
terminal class cross the Interface. The `Publish` and `Withdraw`
Administration operations remain separately authorized; Publish dispatches the
Endpoint-owned `StartPublisher` transaction, not a raw Credential/signer
request. The Connection Interface cannot invoke either operation.

For a User connection, Endpoint parses and binds the Target Link to its Network,
activates its local capability, and passes only the authenticated Target to the
opened `route.Route`. Route owns the volatile State/Entry/private-reachability/
Introduction sequence and returns only a verified Attachment plus immutable
Target/publication evidence. Endpoint supplies Route a narrow callback for its
durable membership Transit Grant journal; the callback cannot select a carrier
or peer. A Grant is terminalized immediately after receiving-Introduction TLS
admission, even when subsequent delivery or Service TLS fails. Fixed Grants
remain verified against current State inside Route. This is the boundary
selected by [ADR-0070](../adr/0070-own-volatile-user-route-orchestration.md).

## Local admission

The Broker has one volatile generation. A Grant is bound to one opaque local
Principal and one of the closed surfaces connection or administration. Admit
creates a fresh one-use capability. Administration consumes its capability
before work and receives only its bounded receipt. Connection activation also
consumes its capability, but returns an opaque active-session lease whose
cancelable context is the ancestor of all Network work for that operation.
The one-use capability expires after its finite admission window; successful
activation does not transfer that pending TTL into the active Connection.
The lease exposes neither the capability nor authority facts, counts against
the Connection Grant's finite budget of 64 sessions, and is released exactly
once after the terminal outcome. Administration has a separate finite budget
of six capabilities and cannot consume the Connection floor.

Exact revoke and Broker or Endpoint close immediately cancel matching active
Connection sessions as well as invalidating unconsumed capabilities. Drain
refuses new admission and is allowed only when that exact Grant carried
`PermitDrain` and the caller supplies a finite deadline. The first active-lease
drain deadline may only be shortened by later calls; it cannot be extended.
A missing or otherwise unprovable finite bound is denied or causes immediate
cancellation.

The only current isolation observation is generic/unqualified. It means the
runtime deliberately makes no statement about sandboxing, hostile same-user
applications, process-tree confinement, supported host platforms, or
Application-level Endpoint Location Privacy. A qualified platform Adapter
requires separate research and an ADR.

## Publication and connection lifecycle

    Administration Grant
      -> participant-owned Endpoint runtime with an opened host Instance binding
      -> register the authenticated State-selected Introduction slot
      -> Publication.PublishAfterReadiness one higher Instance generation
      -> immutable public record + volatile signer
      -> the participant-owned Connection boundary activates a session
      -> session authorization precedes State/Entry/issuer/Route work
      -> exact-Instance TLS challenge/proof + Service Connection v2
      -> zero or more replacement Attachments under immutable recovery facts
      -> one terminal outcome and exactly-once session release
      -> withdraw/supersede stops acquisitions, drains references, erases private material

The Service Connection record grammar retains ardents-interactive-route-v2,
including in the selected successor; the protected Route changes its context
and Attachment composition, not those record bytes. There is no H3 reader,
record-profile negotiation, direct fallback, Publication private key, or
Application IPC authorization. The parser bound of 16 KiB per Data record is
an allocation limit, not a product throughput promise.

The Connection owner implements the successor's coalesced initial
InstanceChallenge/Continuity request and InstanceProof/Continuity response.
NewAuthenticatedStream verifies both proofs against the independently supplied
publication identity and authenticated TLS Attachment before returning a stream.
The Publisher verifies initial Continuity before invoking its opaque Instance
signer. The private initial-state receipt binds the exact Attachment, has zero
Application offsets, and is consumed once by the stream lifecycle without a
second Continuity flight. Cancellation joins the owned transport close and
prevents later receipt consumption. NewStream retains the preceding sequential
composition.

The Endpoint text-Service binding retains the exact verified worker job and
independently checks the signed publication against its live State profile
and local authority bounds. Its Initiator creates a fresh Connection nonce and
salted commitment to the private local context; the local context and salt do
not leave Endpoint. Both roles derive the selected immutable logical context,
while each Attachment derives its separate exporter context from the capsule
digest and generation. TLS 1.3 permits only the selected X25519MLKEM768 and
X25519 groups. Publisher TLS and Instance proof use the currently acquired
opaque publication lease.

The initial text-Service stream invokes that real TLS/native Connection path
and retains its opaque job binding. The reader holds its worker operation
through authentication and document exchange. Publisher accepts only streams
belonging to its exact job and context before transferring bytes to its one
worker; a foreign or retired binding is closed and joined. Cancellation joins
accepted stream forwarding and native I/O before releasing the worker operation
and installed cleanup. A successful document remains conditional on current
context ownership after cleanup.

The installed worker/network profile exercises the initial-attachment composition
with the real installed launcher, Introduction/Route producers, Service authentication
and confined worker protocols for empty, 64-KiB and 4-MiB documents on both Carriers.
Its State, Authority/Instance provisioning and registration scheduling remain explicit
fixtures; this does not qualify the complete protected journey or hostile host.

The Endpoint's text Publisher network producer retains one qualified worker
across independent reads and owns Introduction receipt, JOIN and authenticated
Service-stream handover. It uses an unbuffered handover; cancellation joins the
producer, bridge and worker. A refused malformed, unauthenticated or rate-limited
capsule can leave the snapshot available only when the refusal acknowledgement
succeeds and no cancellation or cleanup failure accompanies it.
The Publisher startup owner qualifies the worker before opening its Source and
Introduction prefixes, creates the initial registration, and waits for verified
Descriptor publication before returning its Link. The returned owner retains
the network producer under the job lifetime, independently of the completed
startup request. Startup failure joins worker/context cleanup. Its Close is an
abort. Its separate withdrawal operation stops new Introduction acceptance before
network withdrawal and joins scheduled refresh before withdrawing the final
registration. Previously admitted reads retain their original lifetimes with an
additional five-second drain bound. Repeated withdrawal cannot extend that bound.
Producer drain preserves cancellation and cleanup failures, and context cleanup
waits for withdrawal ownership to finish. This composition still requires full
network lifecycle qualification and ordinary command adoption.
For a new authorized text context, the permission provisioning owner first uses
the installed launcher with an empty initialization and joins that preparation
worker before exporting any request. It opens no Service stream and publishes
no document. Cancellation or failed qualification cannot export a request;
a previously verified context still undergoes the current authority checks.
The text permission provisioning owner exports the exact public request and
reports its digest before waiting for an actual Custody response file. The
observer cannot grant authority. Waiting ends with caller/context cancellation
or the original request's hour boundary; malformed, partial or mismatched
responses fail without being retried into success. Only a verified import can
complete provisioning. The protected participant supplies trusted owner-only paths and consumes this owner before exposing its commands.
The text Administration owner retains that separately authorized context and
its original local Principal. Each snapshot publication and withdrawal consumes
a fresh Administration capability. Snapshot publication launches the installed
worker and returns success only after the real Descriptor acknowledgement;
bodyless publication cannot invent a document. Withdrawal can cancel and join
pending startup, while a committed publication delegates to the finite drain.
Shutdown aborts the context and joins startup and the retained run. The installed
network profile now exercises snapshot and withdrawal through the real local
Administration transport; that updated profile still requires execution on the
qualified host. Local refusal tests do not establish installed publication.
An explicit `ardents-text link` request consumes fresh Administration authority
and projects the committed publication's canonical Target Link through that
private local transport. Projection requires the current retained worker,
acknowledged live registration and current publication Credential with the same
Network and Target. Pending, withdrawn, expired or disconnected publications
return unavailable. The request cannot publish, retry or refresh, and the Link
is not a promise of future availability. The command joins cancellation of its
bounded output; ordinary runtime diagnostics do not contain the destination.
The text Connection owner consumes a fresh local Reader lease, launches the
installed worker, and completes Service authentication before returning its
AAI3 stream. It then validates the fixed local request, forwards the single
Service exchange through the confined worker, and joins worker retirement
before projecting its bounded RESULT through the local document grammar. This
projection creates no second remote request. Caller or original-context loss
interrupts both setup and local result I/O; owner shutdown joins the pending
read. One retained Reader context admits one read through result completion.
Local network tests exercise this result projection with explicit qualification
fixtures. The installed profile uses the actual AAI3 owner and launcher, but
that revised profile still requires execution on its qualified host.
The protected `RunTextParticipant` composition opens the accepted closed State,
Entry sets, token journal and existing Instance binding. It qualifies and
provisions both retained text contexts before opening the AAI3 Connection and
snapshot Administration transports, rechecks permission currentness before
exposure, and joins servers, contexts and persistent owners on shutdown.
`endpoint headless` selects this composition through an explicit v2 plan;
missing permissions or mixed legacy fields fail without selecting another
runtime. Persisted v1 plans retain `RunParticipant` compatibility. The protected
composition still requires installed command and full network lifecycle
qualification. No caller-supplied Target, permission file or local context
identifier may bypass these owners.
The coalesced authenticated stream requires its directional Terminal receipt
and peer confirmation even when only the initial Attachment is available.
Missing confirmation cannot yield a successful bounded outcome. This does not
create a replacement source: recovery and its retained post-close replay tail
remain separate required integration, not a readiness claim supplied by the
installed network profile. The preceding sequential headless stream retains
its existing orderly half-close behavior.

After a replacement Attachment commits, the Connection replays any accepted but
unacknowledged Data suffix without waiting for a further local Application read,
EOF, or Terminal. That replay remains ordered with later Application bytes and
is joined or interrupted by the Connection's existing terminal cleanup.

Publication persists public proof and its non-decreasing generation floor but
never persists a live Instance private key. The lower-level accepted Publisher
composition can receive one opened host Instance binding and use it as an
opaque Instance signer and fixed-purpose SealedIntroduction v1 recipient
without any Interface returning private bytes or an exportable HPKE key. The
maintained participant runtime opens that binding only after reconciling the
accepted public Credential with the durable publication floor. When its
optional host `service_instance_root` is configured, it consumes State's
indivisible Publisher attachment projection, obtains separate Introduction
and Responder credentials through the Endpoint-owned at-most-once journals,
and constructs the live profile without caller-supplied peers, roles, Grants,
keys, or Route facts. Missing, conflicting, or ambiguous State projection is
unavailable; without a Service Instance root the same process remains a
User-only participant.
AcquireAt yields an opaque Lease; the Lease can sign for its generation without
exposing the signer. Withdrawal, supersession, expiry, or close first prevent
new acquisition, then wait for bounded references before erasing private
material. If the caller cancels while that drain waits, the Publication retains
the withdrawn generation's cleanup ownership; a later publish, withdrawal, or
close joins the same drain before it can release the root lease or expose a
successor generation.

## Service credential and publication limit

The Instance key is generated by the Service host and bound into its public
Service Credential. The inner Service TLS challenge/proof authenticates that
exact Instance; it is not a general mandatory-mTLS product layer. In
particular, an X.509 certificate elsewhere in the deployment has no authority
to create, replace, or renew a Service Credential.

The currently supported Custody issuance is interactive and produces one
public signed response for an independently approved public request. A
Credential lasts at most 24 hours, may end no later than 48 hours from
issuance, and cannot overlap a predecessor for the same Target. There is no
automatic Credential renewal route. The C0 successor rule is an implementation
limit, not a permanent product requirement.

After `CommitPublished`, the Instance root deliberately no longer retains the
private material needed to reopen that published generation. A real
participant restart therefore fails at `Credential()` before it could call
`OpenBinding()`; a successor cannot begin before the predecessor's terminal
`NotAfter`. Deleting a root or publication floor, or changing Target, is not a
recovery procedure for that Service. A later, separately selected design may
change this limitation, but the maintained runtime has no such recovery path.

## Endpoint process contract

`internal/application/interfacev1/connection` owns the sole local Target-Link
Connection Interface: one private Unix attachment carries a non-empty Target
Link of at most 512 bytes, opaque frames of at most 16 KiB, and one UTF-8 typed
terminal outcome with a 128-byte class and 512-byte diagnostic reason. EOF
without that outcome is not success. Setup does not retry or select an
alternate link. `internal/application/interfacev1/administration` separately
owns only `publish` and `withdraw`; it cannot carry Connection data or silently
turn a failure into another success state. Both packages declare
`ardents-application-interface-v1` and execute checked vectors under
`testdata/conformance-v1.json`. There is no result sideband or Endpoint-owned
local grammar. `RunParticipant` retains the Network server implementation and
closes its exact socket paths after cancelling and joining active clients;
external Applications use only the versioned client. No Browser client is
selected in the maintained product.

The Administration client owns its Unix socket from successful dial through the
closed `publish` or `withdraw` response. Caller cancellation immediately
interrupts that owned request I/O and returns the caller's cancellation or
deadline error; a completed response wins only when its cancellation callback
has already been stopped. This aborts local waiting, not a server operation
already accepted by the peer: the client never invents an outcome, retry, or
rollback for Publish or Withdraw.

The `ardents-application-interface-v1` frame identity and its opaque link bytes
remain accepted persisted-interface obligations. A runtime plan carrying the
complete historical Alpha corpus triple is therefore a narrow migration
adapter: it recognizes only an exact `ardents-alpha://` Service Link, resolves
it through that plan's already accepted local floor, and then supplies the
bound Target to the same Endpoint/Route path. Fresh C0 plans omit that triple
and accept only Target Links. A malformed Target Link never falls back to a
Service Link, and the adapter ends only after an explicit versioned
plan/interface migration.

Endpoint is a composition Module, not a second durable domain owner. It owns
no Namespace, Network State, Release, Update, Custody, or Route-selection
state. Route Attachments are already authenticated opaque carriers; Namespace
and State facts arrive only in the typed inputs required for Connection
binding.

`Stream.CloseInput` is an orderly directional operation, distinct from
`Stream.Close`. A local Application sends the accepted zero-length AAI2 input
frame to state that no more request bytes will arrive; the local transport
preserves it through Endpoint and native Service Connection as the existing
authenticated Terminal record. Conversely, only a verified matching remote
Terminal gives the local Application reader EOF. Either transition leaves the
opposite direction available for a response, is safe to repeat, and rejects
later writes in its closed input direction. Cancellation, malformed local
input, carrier loss, and full close remain abort paths, and neither local nor
native EOF is semantic success without the one typed terminal outcome.

A locally written Terminal is a directional completion obligation, not proof
of peer receipt. Under the closed Service Connection v2 grammar selected by
[ADR-0075](../adr/0075-service-connection-v2-terminal-receipt.md), marker `1`
is a Terminal receipt: it names the same generation and offset and is sent only
after the peer verifies that Terminal. Marker `2` confirms the peer observed
that receipt; the receiving endpoint retains recovery ownership until it has
that matching confirmation. If a replacement Attachment completes Continuity
before either control record, Service Connection owns replay: it first replays
any unacknowledged Data at the carried offsets, then emits the same Terminal on
the new generation. The peer still presents only its first verified Terminal as
Application EOF. This preserves one logical half-close without reissuing an
Application operation or inventing EOF on a timer. A headless path that did not
supply an Attachment opener retains its existing orderly half-close and does
not enable recovery.

The endpoint that sends marker `2` cannot learn from a finite record whether
the peer received that final confirmation. After a successful `RunBounded`
outcome, a recovery-enabled Service Connection therefore retains a bounded
in-memory post-close tail until the operation Context, authenticated Work
Safety deadline, or no-new-recovery boundary ends it. That tail owns only the
already-settled Terminal and receipt-control replay; it accepts no Application
Data and never presents another Application EOF. Thus the caller's completed
Application operation is released while a returned peer can still recover the
missing final control proof. The tail adds neither durable recovery state nor a
headless recovery path.

The v1 Application Client serializes `Write` with `CloseInput`, so an accepted
write's complete frames precede the zero-length input-close frame; if the
directional close wins, the later write is rejected. Full `Close` and lifetime
context cancellation are aborts and do not wait for that serialization lock:
they close the owned Unix transport to interrupt any blocked read or write,
reject later operations, and join Client-owned work before publishing `local
cancellation`. A verified remote terminal outcome completed by the joined
receiver remains authoritative; transport errors induced by the abort cannot
replace it or become a second outcome. An interrupted `Write` may report only
its completed payload prefix plus an error and can never become clean success.

`Dial` owns the Unix transport and its cancellation from the instant a socket is
opened through request write, status, and any refusal read. A setup cancellation
guard closes that transport immediately and returns the originating cancellation
or deadline error; no later setup deadline or silent peer can replace it. On an
accepted status, one locked handoff changes that same guard from setup transport
cleanup to `Client.Close` before `Dial` returns. If cancellation won first,
setup joins its cleanup and returns no Client; if the successful handoff won,
the returned Client owns the remaining context watch, transport, I/O joins, and
terminal outcome. Thus neither an error branch nor the success/cancellation race
leaves an unowned socket or a guard that can close a successfully transferred
Client without a cancellation.

Repeated and concurrent `Close` calls join the same cleanup and return its
result. The headless `open`
caller joins its response copier on input failure or cancellation and removes
the partial output before returning; it neither receives nor closes the socket
directly.

Explicit publication withdrawal uses a fresh Service Administration capability
and returns `unpublished` only for the exact Target/generation after retained
connections drain; an established connection may therefore finish as `clean
service connection close` while no later publication acquisition is possible.
A fresh repeated withdrawal must return `service unavailable`; the publication
owner rejects acquisition as soon as unpublish begins, before retained leases
finish draining.
A non-EOF Publisher Application socket failure and an abrupt Publisher Endpoint
loss are `abrupt connection loss`, never `service unavailable` or clean close.
An Application may receive a byte prefix before an abrupt failure. The typed
terminal outcome remains authoritative for the Connection lifecycle; received
bytes do not prove that an Application operation completed. Interpretation of
HTTP status, response completeness, or semantic retry belongs to the external
Application. Endpoint never substitutes another Target or an Internet path.

The Endpoint contains no Browser presentation or Browser Entry state. The
former Browser implementation and qualification lanes are retired; Firefox
source remains only as non-executable compatibility evidence under
`tests/compatibility/browser-endpoint-v4` in accordance with [ADR-0061](../adr/0061-retain-firefox-entry-as-compatibility-evidence.md)
and [ADR-0069](../adr/0069-retire-active-browser-implementation.md).

## Verification and related decisions

- Go tests for Broker, Endpoint, Publication, and Service Connection exercise
  the Module Interfaces and failure paths.
- Application Connection and Administration behavior tests exercise framing,
  typed refusal/outcome, cancellation, join, and exact socket cleanup through
  their public Interfaces. Architecture tests forbid a second Endpoint-local
  transport owner and enforce the command dependency graphs.
- [ADR-0024](../adr/0024-native-interactive-route-foundation.md) selects the
  native Route foundation; [ADR-0075](../adr/0075-service-connection-v2-terminal-receipt.md)
  selects the closed Service Connection grammar.
- The Broker is limited to its explicit generic/unqualified contract; it makes
  no platform-isolation or Application-level Endpoint Location Privacy claim.
