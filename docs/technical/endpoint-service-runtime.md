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
| internal/endpoint | Compose one role-local process and expose typed publication, withdrawal, inbound-connection, and outbound-connection operations. Run owns bounded plan loading, readiness, local listeners, signal cancellation, joining, and residue cleanup. | Local socket choreography, result-channel negotiation, Broker consumption, TLS carrier setup, publication acquisition, and Connection invocation. |
| internal/service/publication | Open, publish, acquire, unpublish, and close one exclusive Service Instance generation. | Crash-atomic public record/floor persistence, volatile Instance signer, live-reference accounting, drain, and private-material erasure. |
| internal/service/connection | Carry one logical authenticated Service Connection across fresh Route Attachments and return one terminal outcome. | Exact Instance challenge/proof, continuity MAC, ordered data/acknowledgement offsets, replay handling, recovery deadline, and attachment cleanup. |

The caller-facing Endpoint seam is role-specific: a publication request cannot
include Route or Application facts, and an outbound connection cannot supply a
publisher signer. This keeps publication ownership, local admission, Route
attachment, and logical-stream recovery out of one mutable request bag.

The maintained Connection Interface adds one narrower consumer operation over
that composition. A headless caller supplies an explicit Service Link and its
local Connection principal; Endpoint retains accepted naming state, Entry,
Target authentication, Route inputs, the one-use Transit Grant/key, and the
Broker capability. Only an authenticated ordered byte stream and bounded
terminal class cross the Interface. The existing `Publish` and `Withdraw`
operations remain the separately authorized Service Administration surface;
the Connection Interface cannot invoke them.

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
      -> Endpoint.Publish
      -> Publication.Publish one higher Instance generation
      -> immutable public record + volatile signer
      -> Endpoint.Connect or Endpoint.Accept consumes a Connection Grant
      -> exact-Instance TLS challenge/proof + Service Connection v1
      -> zero or more replacement Attachments under immutable recovery facts
      -> one terminal outcome
      -> unpublish/supersede stops acquisitions, drains references, erases private material

Service Connection accepts only the closed ardents-interactive-route-v1
profile. It has no H3 reader, profile negotiation, direct fallback,
peer-selected profile, Publication private key, or Application IPC
authorization. Its parser bound of 16 KiB per Data record is an allocation
limit, not a product throughput promise.

Publication persists public proof and its non-decreasing generation floor but
never persists a live Instance private key. The maintained Publisher receives
one opened host Instance binding: Endpoint can use it as an opaque Instance
signer and as the fixed-purpose SealedIntroduction v1 recipient without any
Interface returning private bytes or an exportable HPKE key. The old direct
HPKE-private input remains only in lower-level compatibility evidence.
AcquireAt yields an opaque Lease; the Lease can sign for its generation without
exposing the signer. Withdrawal, supersession, expiry, or close first prevent
new acquisition, then wait for bounded references before erasing private
material.

## Endpoint process contract

The Endpoint v1 Application contract chooses a separate terminal-result channel
before opaque Application bytes flow. Raw-tail delivery and timing-selected
terminal results are retired. Endpoint cancellation closes only its owned
Application, result, and Route listeners, joins blocked accepts, and removes
those socket paths before it returns.

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

The retained Firefox compatibility plan may select
`BrowserEntryProfile: "firefox-alpha"`. In that regression path the Endpoint
owns the alpha proxy port, a fresh local liveness capability, and a separate
one-process proxy credential at the native host's fixed per-user state path
while an authenticated alpha route is live; it removes them before route
withdrawal or shutdown. An absolute `BrowserEntryStatePath` remains a local
test override and cannot be combined with the profile. The native host reproves
the proxy before it returns either the port or the credential for a matching
loopback Basic-auth challenge, and the proxy strips that authentication header
before forwarding to the selected Publisher presentation. This retained path
does not install an add-on, start a browser, or change DNS, proxy, VPN, or trust
settings. Its historical release-bound manifest lifecycle is defined by the
superseded [ADR-0045](../adr/0045-firefox-first-unlisted-browser-entry-delivery.md);
[ADR-0061](../adr/0061-retain-firefox-entry-as-compatibility-evidence.md)
keeps it outside the headless product and candidate qualification.

The generic `endpoint.Run` plan still accepts this profile only so exact
compatibility evidence remains reproducible. `AlphaBrowserResolution` is not a
selected participant Browser Entry and must not be promoted to a normal
Endpoint command from R-106 inputs alone. Promotion requires a new decision for
browser/system resolution and HTTP/HTTPS trust, followed by its own affected
qualification.

## Verification and related decisions

- Go tests for Broker, Endpoint, Publication, and Service Connection exercise
  the Module Interfaces and failure paths.
- The Endpoint recovery process test exercises readiness, cancellation, join,
  socket cleanup, publication, and opaque Application stream boundaries.
- [ADR-0024](../adr/0024-native-interactive-route-foundation.md) selects the
  native Route foundation; [ADR-0028](../adr/0028-native-service-connection-v1.md)
  selects the closed Service Connection grammar.
- The Broker is limited to its explicit generic/unqualified contract; it makes
  no platform-isolation or Application Location Privacy claim.
