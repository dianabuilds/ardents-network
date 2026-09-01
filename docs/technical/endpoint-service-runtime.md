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

## Ownership

The local runtime has separate Modules and Interfaces:

| Module | Interface responsibility | Implementation hidden from callers |
|---|---|---|
| internal/application/broker | Admit and consume one short-lived Local Grant capability for either connection or administration; revoke, drain, and close pending capabilities and active Connection leases; report generic/unqualified. | Capability generation, replay removal, expiry, commitments, admission-load accounting, and grant invalidation. |
| internal/application/interfacev1/connection | Carry one Service Link, one ordered byte stream, and exactly one bounded terminal outcome under `ardents-application-interface-v1`; retain the accepted AAI2 bytes and executable conformance vectors. | State, Entry, Target, Route, Credential, Custody, Service keys, retries, fallback, and Network diagnostics. |
| internal/application/interfacev1/administration | Carry one separately authorized `publish` or `withdraw` request and its closed success/unavailable result under the same interface version and vectors. | Connection bytes, publication inputs, Credential/key material, State, Route, Target, and Network diagnostics. |
| internal/endpoint | Compose one role-local participant and implement the shared Connection and Administration Interfaces. `RunParticipant` opens authenticated participant owners, delegates local transports to the Application Modules, and joins shutdown. | Broker consumption, authenticated State/Entry/Target projection, TLS carrier setup, publication acquisition, and Connection invocation. |
| internal/service/publication | Open, publish, acquire, unpublish, and close one exclusive Service Instance generation. | Crash-atomic public record/floor persistence, volatile Instance signer, live-reference accounting, drain, and private-material erasure. |
| internal/service/connection | Carry one logical authenticated Service Connection across fresh Route Attachments and return one terminal outcome. | Exact Instance challenge/proof, continuity MAC, ordered data/acknowledgement offsets, replay handling, recovery deadline, and attachment cleanup. |

The caller-facing Endpoint seam is role-specific: a Publisher start request
cannot include Route, Credential, signer, or Application facts, and an outbound
connection cannot supply a Publisher binding. This keeps publication ownership, local admission, Route
attachment, and logical-stream recovery out of one mutable request bag.

The maintained Connection Interface adds one narrower consumer operation over
that composition. A headless caller supplies an explicit Service Link and its
local Connection principal; Endpoint retains accepted naming state, Entry,
Target authentication, Route inputs, the one-use Transit Grant/key, and the
Broker admission input. After syntactic Service Link parsing, Endpoint activates
and consumes the Connection capability before it reads current State, touches
Entry or private reachability, asks an issuer for a Transit Grant, opens Route,
or sends Introduction. Only an authenticated ordered byte stream and bounded
terminal class cross the Interface. The `Publish` and `Withdraw`
Administration operations remain separately authorized; Publish dispatches the
Endpoint-owned `StartPublisher` transaction, not a raw Credential/signer
request. The Connection Interface cannot invoke either operation.

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
Application Location Privacy. A qualified platform Adapter requires separate
research and an ADR.

## Publication and connection lifecycle

    Administration Grant
      -> participant-owned Endpoint runtime with an opened host Instance binding
      -> register the authenticated State-selected Introduction slot
      -> Publication.PublishAfterReadiness one higher Instance generation
      -> immutable public record + volatile signer
      -> the participant-owned Connection boundary activates a session
      -> session authorization precedes State/Entry/issuer/Route work
      -> exact-Instance TLS challenge/proof + Service Connection v1
      -> zero or more replacement Attachments under immutable recovery facts
      -> one terminal outcome and exactly-once session release
      -> withdraw/supersede stops acquisitions, drains references, erases private material

Service Connection accepts only the closed ardents-interactive-route-v1
profile. It has no H3 reader, profile negotiation, direct fallback,
peer-selected profile, Publication private key, or Application IPC
authorization. Its parser bound of 16 KiB per Data record is an allocation
limit, not a product throughput promise.

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
material.

## Endpoint process contract

`internal/application/interfacev1/connection` owns the sole local Service-Link
Connection Interface: one private Unix attachment carries a non-empty Service
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

Endpoint is a composition Module, not a second durable domain owner. It owns
no Namespace, Network State, Release, Update, Custody, or Route-selection
state. Route Attachments are already authenticated opaque carriers; Namespace
and State facts arrive only in the typed inputs required for Connection
binding.

Explicit publication withdrawal uses a fresh Service Administration capability
and returns `unpublished` only for the exact Target/generation after retained
connections drain; an established connection may therefore finish as `clean
service connection close` while no later publication acquisition is possible.
A fresh repeated withdrawal must return `service unavailable`; the publication
owner rejects acquisition as soon as unpublish begins, before retained leases
finish draining.
A non-EOF Publisher Application socket failure and an abrupt Publisher Endpoint
loss are `abrupt connection loss`, never `service unavailable` or clean close.
If a Publisher fails after HTTP response headers are committed, the local HTTP
server may expose only the already received body prefix because it cannot emit
a second status. `ReferenceConnection.Done` remains the authoritative bounded
terminal result, and the scoped proxy is withdrawn without a same-name,
other-Target, or Internet fallback. A distinct registered Target is addressed
only by an explicit request for its own authenticated name.

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
  native Route foundation; [ADR-0028](../adr/0028-native-service-connection-v1.md)
  selects the closed Service Connection grammar.
- The Broker is limited to its explicit generic/unqualified contract; it makes
  no platform-isolation or Application Location Privacy claim.
