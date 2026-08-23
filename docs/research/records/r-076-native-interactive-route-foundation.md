---
id: R-076
title: Which maintained Route and Carrier foundation closes DA-06 without retaining Horizon 3 wire bytes?
status: accepted
owner: Product Owner and Codex
started: 2026-08-23
reviewed: 2026-08-23
---

# R-076 — Native Interactive Route foundation

## Decision this unlocks

Close DA-06 for M7--M9: select the maintained Route topology, Carrier
representation, authority boundary, recovery owner, and transition treatment.
This is a foundation decision, not Route Qualification, a public-network
launch, or a claim of independently operated Nodes.

## Current contract

R-001 preserves the low-latency Interactive Route claim: a complete Route,
the exact Service Target, and plaintext never become available to one ordinary
role; unavailability never becomes a direct, shorter, cached, or weaker-profile
success. R-004 and ADR-0005 select the split C-5 data path and separate C-2
Introduction information-flow shape, not Tor or a public wire. ADR-0006 fixes
authenticated protocol transition and finite work safety. ADR-0009 selects
standard-library-first Go. R-075 rejects promoting either the Horizon 3 tracer
or another network's authority wholesale.

The current `route`, `routeplan`, `bridge`, and `camouflage` packages are H3
inputs. Their plan files, evidence unions, `ASIA`/`ASAT`/`ASCH`/`ASPR`/`ASCR`/
`ASPB`/`ASCF` frames, WebTunnel envelope, and unsigned lab framing have no
selected compatibility observer.

## Hypotheses

- **H1:** a native Go Route over authenticated TCP/TLS 1.3 can retain the
  selected split-leg information flow while keeping Ardents' State, Role Domain,
  Service publication, and recovery authority local.
- **H2:** a foreign overlay or endpoint camouflage implementation is required
  as the maintained Route foundation.
- **H0:** no option can name a compatible route, authority, transition, and
  Qualification boundary without weakening R-001/R-004.

## Evaluation criteria

The foundation must name an exact Route Profile, Carrier boundary, authority
and discovery inputs, role-local disclosure rule, version/downgrade treatment,
recovery owner, maintained implementation, and Qualification work. It must not
inherit H3 bytes merely because their controlled test passed, turn a Carrier
connection into a direct Service fallback, or adopt a foreign directory,
descriptor, peer-ID, or governance root.

## Evidence plan

### Primary sources

- R-001, R-004, R-013, R-015, R-023, R-075, ADR-0005, ADR-0006, and ADR-0009,
  inspected 2026-08-23.
- [RFC 8446: TLS 1.3](https://www.rfc-editor.org/rfc/rfc8446.html), accessed
  2026-08-23. TLS protects a reliable ordered byte stream and provides
  authenticated confidentiality and integrity after its handshake.
- [RFC 9180: HPKE](https://www.rfc-editor.org/rfc/rfc9180.html), accessed
  2026-08-23. It supplies the existing reviewed sealed Introduction primitive;
  this record selects no cryptographic primitive implemented by Ardents.
- [Go `crypto/tls` package documentation](https://pkg.go.dev/crypto/tls),
  accessed 2026-08-23.

### Experiment

M7/M8 must introduce target-owned behavior and process tests before deleting a
source package. A later R-023 Qualification run must use this exact Profile,
including role-view, active substitution/replay, recovery, capacity, and
independent-operator evidence. H3 results remain C4 provenance only; they are
not measurements of this foundation.

### Failure scenarios

- a Node accepts a profile or generation not authenticated by current State;
- a route/open/recovery path reaches the Service directly or chooses a lower
  profile after an error;
- a Bridge/Entry sees or selects the complete Route;
- a replayed Introduction or Rendezvous attachment joins a new connection;
- a future peer uses an unauthenticated version signal to downgrade a Route;
- a Node's previous Role Domain overlaps a new duty; and
- a co-resident test topology is presented as independent operation.

## Findings

- **Inspection:** R-013's native C-5/C2 shape already demonstrates controlled
  role-local forwarding, a separately sealed Introduction, endpoint TLS, and
  explicit failure, but explicitly marks its codec and plan files
  non-production.
- **Sourced fact:** TLS 1.3 is designed to provide an authenticated protected
  channel over a reliable ordered transport; its handshake detects parameter
  tampering. HPKE is a standard sealed-message construction rather than a
  reason to create a first-party cryptographic primitive.
- **Inspection:** the chosen H3 WebTunnel component is an endpoint-adjacent
  camouflage Adapter. It owns neither Route authority nor retry/continuity
  semantics, so it cannot be the Route foundation.
- **Inference:** native TCP plus mutually authenticated TLS 1.3 is the only
  evaluated carrier that preserves the selected role shape without importing a
  foreign authority system. It has an honest limitation: it is not a transport
  camouflage or broad-observer-resistance promise.

## Selected Profile

Choose **`ardents-interactive-route-v1`** as the sole maintained H3 Route
Profile foundation.

1. **Topology.** A connection has a User-selected C-5 data path
   `User -> Initiator -> Rendezvous -> Responder -> Service` and a separately
   selected C-2 Introduction path. The selected Rendezvous never receives or
   forwards the Introduction. The position count is an implementation of the
   ADR-0005 knowledge boundary, never Application or Service API data.
2. **Carrier.** Each adjacent Node leg is TCP carrying mutually authenticated
   TLS 1.3. The Go standard library owns TLS; no Tor, Arti, libp2p, QUIC,
   WebTunnel process, first-party crypto, cgo, or `unsafe` enters the Profile.
   TLS resumption and 0-RTT are disabled. A direct TCP connection is permitted
   only from an endpoint to its selected adjacent Entry/Node; there is never a
   direct endpoint-to-Service success path.
3. **Authority and discovery.** Current authenticated Network State supplies
   the Profile generation, Node credential, address, Role Domain, duty, expiry,
   and capability facts. Endpoint-local Route selection consumes opaque State
   View/Duty/Resource/Entry ports. Private resolution/publication supplies the
   exact Service-side Introduction material; it supplies neither a foreign
   directory nor a global peer identity. An Entry Invite authorizes only its
   bounded adjacent candidate and replay/replacement history. It cannot choose
   a Route.
4. **Protected setup.** The sealed one-use Introduction uses the already
   reviewed RFC 9180 suite. Its authenticated associated data bind the Profile
   identifier and generation, Network identity, Epoch digest, selected
   Rendezvous identity/reachability, expiry, one-use join token, and endpoint
   handshake context. The Service rejects every mismatched, expired, malformed,
   replayed, or substituted value. Rendezvous atomically consumes its random
   attempt/join handle before joining opaque streams.
5. **Endpoint connection.** Only after the two opaque legs join do User and
   Service run a fresh TLS 1.3 handshake. The User authenticates the exact
   resolved Target/Instance. Service Name, Target, and target-derived SNI/ALPN
   never appear in an adjacent carrier handshake. Endpoint TLS binds the same
   Profile/generation, Network identity, Target/Instance generation, Isolation
   Context, and Route attempt; it does not authenticate a complete Route to a
   Node.
6. **Recovery.** `service/connection` owns replay, ordered bytes, deadlines,
   and terminal outcome; `route` owns only a fresh bounded Attachment. Recovery
   may use another authenticated candidate satisfying the exact profile and
   exclusion rules. It must not reuse stale authority, relax a profile, reveal a
   Target, or fall back to direct networking.
7. **Transition and bytes.** H3 W02/W04 bytes are C0 retired, not a supported
   peer version. `v1` begins with no legacy reader. State and publication bind
   exact supported Profile generations; an endpoint selects the highest
   mutually supported *qualified* generation before Route construction, never
   by a Node-supplied downgrade. Future announced, overlap-supported,
   preferred, required, and retired generations follow ADR-0006/R-015,
   including the ordinary 90-day overlap and finite no-new-work/terminal
   deadlines. The v1 on-wire canonical encoding, test vectors, and conformance
   corpus are a required M8 design artifact under R-015; no current H3 codec
   may become its implicit predecessor.

## Options

| Option | Product/security fit | Disposition |
|---|---|---|
| Native `ardents-interactive-route-v1` | Preserves the selected split-leg information flow, keeps authority in Ardents State/publication, and is implementable in maintained Go. It has no camouflage or broad-observer claim. | choose |
| Promote native H3 C-5/C2 bytes | Reuses a controlled tracer but lacks selected authority, protocol transition, and Qualification evidence. | reject |
| Tor/Arti | Brings foreign descriptor/directory/identity semantics; Arti is not the selected maintained Go route implementation. | reject |
| libp2p Circuit Relay | Provides peer-addressed reachability rather than the selected endpoint-hidden split Route. | reject |
| WebTunnel | Remains an H3 endpoint camouflage Adapter, not a Route authority or recovery owner. | reject |

## Recommendation

Choose H1 with medium confidence and create ADR-0024. It unblocks source
ownership work while keeping public promotion behind R-023 Qualification and
independent evidence. The strongest argument against H1 is maintenance cost of
a native protocol; the alternative candidates either carry more incompatible
authority than implementation leverage or do not implement the required Route.

## Disposition

**Accepted 2026-08-23 under the Product Owner's standing Stage 8 delegation.**
ADR-0024 records the consequential transport/protocol foundation. DA-06 is
closed for M7--M9. `camouflage` is not retained; M7 deletes it rather than
creating `route/webtunnel`. R-013's Carrier Lab and R-036's WebTunnel record
remain historical evidence only. M8 must create the v1 canonical codec,
vectors, and downgrade/mixed-generation tests before any peer-facing profile
is announced; until then a local target implementation has no compatibility
promise.
