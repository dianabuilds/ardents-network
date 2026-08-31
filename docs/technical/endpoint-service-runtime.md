# Endpoint and Service runtime

Status: **current maintained technical contract.** This document describes the
local Endpoint, generic Broker, Service publication, and Service Connection
Modules that exist in the repository. It does not select a supported desktop
profile, a qualified Application Isolation profile, a public Service protocol,
or a complete Route/Node qualification.

## Ownership

The local runtime has separate Modules and Interfaces:

| Module | Interface responsibility | Implementation hidden from callers |
|---|---|---|
| internal/application/broker | Admit and consume one short-lived Local Grant capability for either connection or administration; revoke, drain, and close that finite session set; report generic/unqualified. | Capability generation, replay removal, expiry, commitments, session accounting, and grant invalidation. |
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
Broker capability. Only an authenticated ordered byte stream and bounded
terminal class cross the Interface. The `Publish` and `Withdraw`
Administration operations remain separately authorized; Publish dispatches the
Endpoint-owned `StartPublisher` transaction, not a raw Credential/signer
request. The Connection Interface cannot invoke either operation.

## Local admission

The Broker has one volatile generation. A Grant is bound to one opaque local
Principal and one of the closed surfaces connection or administration. Admit
creates a fresh one-use capability; Consume removes it before returning a
bounded receipt. Revoke, permitted finite drain, and close invalidate
unconsumed capabilities immediately. Work that already consumed a capability
is not claimed to be interrupted by Broker revocation.

The only current isolation observation is generic/unqualified. It means the
runtime deliberately makes no statement about sandboxing, hostile same-user
applications, process-tree confinement, supported host platforms, or
Application Location Privacy. A qualified platform Adapter requires separate
research and an ADR.

## Publication and connection lifecycle

    Administration Grant
      -> Endpoint.StartPublisher with an opened host Instance binding
      -> register the authenticated State-selected Introduction slot
      -> Publication.PublishAfterReadiness one higher Instance generation
      -> immutable public record + volatile signer
      -> Endpoint.Connect or Endpoint.Accept consumes a Connection Grant
      -> exact-Instance TLS challenge/proof + Service Connection v1
      -> zero or more replacement Attachments under immutable recovery facts
      -> one terminal outcome
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
maintained participant runtime still requires a Product Owner decision for its
exact State-derived Publisher attachment projection before it may construct
that binding and live profile.
AcquireAt yields an opaque Lease; the Lease can sign for its generation without
exposing the signer. Withdrawal, supersession, expiry, or close first prevent
new acquisition, then wait for bounded references before erasing private
material.

## Endpoint process contract

`internal/application/connection` owns the sole local Service-Link Connection
Interface: one private Unix attachment carries a bounded Service Link request,
opaque framed bytes, and one typed terminal Outcome. There is no result
sideband or Endpoint-owned local grammar. `internal/application/administration`
separately owns the closed Publish/Withdraw grammar. `RunParticipant` retains
the server transports and closes their exact socket paths after cancelling and
joining active clients; CLI and Browser adapters use the shared client.

Endpoint is a composition Module, not a second durable domain owner. It owns
no Namespace, Network State, Release, Update, Custody, or Route-selection
state. Route Attachments are already authenticated opaque carriers; Namespace
and State facts arrive only in the typed inputs required for Connection
binding.

For the bounded H4-3 Reference Site profile, an asynchronous User Reference
Session reports only `starting`, authenticated `ready`, `unavailable`, and
`stopped`. `ready` includes a fresh scoped loopback origin only after the exact
Target authenticated; unavailable/terminal events retain the bounded Endpoint
class and reason, never raw Route or peer diagnostics. The retained H4-3A
presentation is a closed static Reference Site. The active H4-3B tracer adds a
separate explicit alpha HTTP/1.1 bridge for one selected Service Connection:
it preserves ordinary request/response semantics and streaming, orders work on
that Connection, and has no Target, Route, content-profile, or browser-wide
proxy authority. For that presentation only, an authenticated remote
Application terminal closes the local bridge so a completed Publisher cannot
leave the visible name usable. The session owns no destination selection,
retry, generic proxy capability, or additional browser authority.

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

Endpoint contains no Browser, Firefox, proxy, presentation, or Browser Entry
state. `cmd/ardents-browser` and `internal/browseradapter` own the optional
Browser presentation and depend only on the local Connection Interface plus
Browser-owned Modules. Firefox-only source is retained as non-executable
compatibility evidence under `tests/compatibility/browser-endpoint-v4` in
accordance with [ADR-0061](../adr/0061-retain-firefox-entry-as-compatibility-evidence.md).

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
