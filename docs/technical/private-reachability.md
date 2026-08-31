# Private Target reachability

Status: **accepted private-reachability contract; the closed descriptor codec, Endpoint
composition, Gateway-local durable currentness state, and fixed-size OHTTP
Relay/Gateway exchange and ADR-0037's closed Entry-to-Initiator carrier exist;
both in-process and seven-process Target Link → lookup → C-2 → Reference Site
tests pass. Live qualification remains pending.** This is the Target-keyed
companion to the Namespace-only private
resolution contract. It implements [ADR-0036](../adr/0036-target-private-reachability-v1.md)
and [ADR-0037](../adr/0037-private-reachability-entry-carrier.md).

## Purpose and boundary

Given an exact, network-bound Target Link, the Endpoint obtains one
authenticated, current Service Publication and short-lived live introduction
facts. The result is input to the existing User C-2 composition; it is neither
a Service Connection nor evidence that the Publisher is online.

The protocol has three roles:

```text
Endpoint -- private lookup Entry --> Initiator -- OHTTP --> Destination Resolution Gateway
                                                            ^
Publisher -- authenticated descriptor publication ----------+
```

The Initiator can observe Endpoint adjacency but not the Target. The Gateway can
observe the Target but not the Endpoint origin. Both are State-selected
Rendezvous-domain identities and their known families are excluded from the
later Service Connection's Rendezvous. A lookup uses a separate Isolation
Context/channel and a separate Initiator Entry acquisition from the connection
it will enable. No role receives a Publisher origin, Service private key,
complete Route, or authority to select a fallback.

The response must be a fixed-size OHTTP message using the authenticated common
Gateway configuration profile. For the interactive profile, State projects one
assigned Gateway identity/family together with that opaque signed
`GatewayProfile`; the Endpoint verifies its Reachability-owned signature
against the same State identity and has no configuration, ordering, URL, or
profile fallback. The Endpoint binds fresh nonce, Network ID, exact Target,
deadline, Gateway profile, and selected State generation/digest before
accepting a response. A Relay/Gateway failure is an explicit private
reachability failure; ordinary HTTP, DNS, Name resolution, local aliases,
catalogs, and Publisher origins are not fallbacks. The Endpoint never connects
directly to the Gateway or an ordinary HTTP Relay: it sends exactly one opaque
envelope through a fresh admitted Entry attachment, and the Initiator derives
the Gateway literal endpoint only from its authenticated State facts. The
Gateway HTTPS certificate is pinned to that State-selected Ed25519 Node key;
ambient roots and HTTP proxy configuration are not used.

## Descriptor authority and currentness

`Service Authority -> Credential -> current Instance -> Reachability Descriptor`

1. `Target = publication.Target(AuthorityPublic)`. The verifier derives the
   Target from the Authority public key contained in the candidate Publication;
   a supplied Target Link must be identical.
2. The Service Authority issues only non-overlapping Credential validity
   intervals for that Target. A later generation cannot begin before every
   predecessor's terminal `NotAfter`. A restart-safe Authority issuance ledger
   enforces this policy locally. Authority compromise remains an explicit
   limitation, not a resolver condition.
3. The Gateway durably stores per Target: highest accepted Credential
   generation, publication digest, terminal Credential expiry, and a
   `conflicting` terminal marker for two different valid records at the same
   generation. It never replaces that state from a lower generation. An equal
   generation is accepted only if its digest is identical; a different digest
   makes the Target unavailable until a valid later generation is accepted.
4. The current Instance signs a Reachability Descriptor bound to exactly one
   Publication digest. It may refresh a live Introduction slot while that
   publication remains current. Its expiry cannot exceed the Publication
   Credential expiry. The Gateway retains only the latest valid live descriptor
   for the same accepted publication and must discard it at expiry.
5. The Endpoint independently verifies all signatures, exact Target/Network,
   Authority derivation, Credential capabilities and time interval, publication
   digest, Instance descriptor signature, state binding, and every finite slot
   expiry. Any failed check is `invalid reachability evidence`, never a partial
   result.

Thus a Gateway can withhold results, return an expired descriptor, or deny
service, all of which become explicit unavailable outcomes. It cannot make a
different Target, forged publication, or older overlapping Credential produce
a Service Connection. A descriptor for an old but still-live slot may at most
try the same authenticated Service Instance; ordinary C-2 slot replay controls
then yield unavailable rather than a different destination.

`internal/service/reachability.Store` implements the Gateway-local part of
this invariant with an exclusive durable root: it persists one accepted exact
descriptor plus a conflict bit per Target, reconstructs the signed fact on
restart, refuses a lower generation, requires non-overlap for a higher
Credential, accepts a slot refresh only when its expiry increases, and records
two differing Publications at one generation as persistent `conflicting`.
The Gateway's `Publish` boundary requires a current authenticated State-role
authorization callback before it gives a descriptor to that Store; the Store
itself remains the lower-level durable currentness owner. Endpoint composition
requires separate lookup and C-2 attachment identifiers and rejects the
Gateway's identity or family when it overlaps any C-2 peer.

## Bounded records

`internal/service/reachability` implements a closed versioned Descriptor
record: v1 contains exactly one bounded fixed Introduction Transit Grant; v2
declares membership-level dynamic Introduction submission and contains no
publisher-specific authorization. Both contain Network ID, Target, Authority
public key, Publication digest, State digest/epoch,
Introduction/Rendezvous identities, opaque reachability/join values,
whole-second expiry, complete Publication bytes, and a current-Instance
Ed25519 signature. Its `Verify` operation rejects altered, trailing,
unsupported-version, wrong-target, wrong-network, expired, and mismatched
Publication evidence before an Endpoint can open Entry. No field is implicit
or caller-assembled from an untrusted configuration.

### Descriptor publication

The Publisher supplies:

- exact Network ID and Target;
- one complete immutable Publication record;
- its digest and Credential generation;
- a current-Instance signature over a versioned descriptor transcript;
- one State-bound Introduction profile: State epoch/digest, Introduction Node
  identity, Rendezvous Node identity, opaque reachability and join-handle,
  either a fixed submission authorization (v1) or an explicit
  membership-level declaration (v2), and whole-second expiry. A decodable
  Transit Grant carries its own exact attachment and local TLS-key binding;
  an Endpoint must not invent or substitute either under that Grant. Under v2
  it instead obtains one exact target-free Grant through the separate
  State-selected Credential Relay, and must not reinterpret v1 as that
  permission. The separate purpose-scoped signer, durable budget, fixed
  encrypted outcomes, and Endpoint reconciliation lifecycle are owned by
  [Transit Grant acquisition](transit-grant-acquisition.md);
- no Node endpoint literal, User identity, Entry Invite, Route, Application
  bytes, Service Authority private material, or Publisher ordinary origin.

The Gateway checks identity/role eligibility against its current State view and
stores the descriptor only after all bounded validity checks pass. It returns
an opaque classified acknowledgement to the Publisher; `accepted` is not a
current Service Connection claim.

### Private lookup response

The Gateway returns either a fixed class `resolved`, `unavailable`, or
`invalid`, always bound to the request nonce/deadline/Target/Network, plus only
for `resolved`:

- immutable Publication bytes;
- the current Instance-signed live Introduction profile; and
- the Gateway role identity/profile evidence already selected from State.

The Endpoint obtains its Initiator, connection Rendezvous, and Entry acquisition
from its own current State/Entry owners. It must check that all role identities
and known families satisfy the declared exclusions before passing typed values
to `OpenUserIntroductionRoute`.

## Required outcomes and qualification

The public Endpoint result vocabulary must distinguish `resolved`,
`unavailable`, `stale`, `conflicting`, `invalid evidence`, `private resolution
unavailable`, and local policy/resource refusal without revealing route
topology to the browser. Only a verified `resolved` result may create the
scoped loopback Reference Site; every other result leaves no listener.

Before private reachability supports an operational usability claim, a separate Publisher, User, Gateway,
Introduction, Initiator, Rendezvous, and Responder process experiment must
show: success; a changed Target; old still-time-valid credential; same-
generation publication conflict; expired/stale introduction slot; substituted
Entry invite; Gateway withholding; Publisher withdrawal/offline; no direct
Publisher request; and listener removal on close. The two host Ubuntu run must
retain its exact binary and State/profile evidence.

The maintained implementation and test denominator cover in-process and
bounded local-process success paths, including the closed lookup carrier. The
retired stage-specific Reference C-2 topology is provenance at
[`fbb42034757513ac009114a00b933aefa76d8ddf`](https://github.com/dianabuilds/ardents-network/commit/fbb42034757513ac009114a00b933aefa76d8ddf),
not current qualification. A selected real browser, the required failure
matrix, and a two-host Ubuntu result remain unqualified, so C0 exposes this
surface for audit without declaring it operationally usable.

## Non-claims

This is not Namespace resolution, DNS, a public directory, an ordinary
descriptor server, a Publisher-origin hiding guarantee against colluding
Relay/Gateway, a protection against timing/volume correlation, or a guarantee
that the resolved Service stays available. It does not add browser isolation,
content safety, application authorization, replication, offline delivery, or
generic Internet proxying.
