# Private Target reachability

Status: **accepted private-reachability contract; the closed descriptor codec, Endpoint
composition, Gateway-local durable currentness state, and fixed-size OHTTP
Relay/Gateway exchange and ADR-0037's closed Entry-to-Initiator carrier exist;
both in-process and bounded local-process Target Link → lookup → Endpoint-owned
Service Connection tests pass. Live qualification remains pending.** This is the Target-keyed
companion to the Namespace-only private
resolution contract. It implements [ADR-0036](../adr/0036-target-private-reachability-v1.md)
and [ADR-0037](../adr/0037-private-reachability-entry-carrier.md).

The selected successor [protocol](protected-route-protocol.md#terminal-payloads-and-private-reachability)
replaces this transport and reachability payload under ADR-0081 while preserving
proof/currentness and conflict floors. The generation-2 facts below describe
the existing implementation, not the successor's exposed join fields.

## Private Descriptor recipient

The generation-3 codec binds the complete existing Publication to the current
State profile digest, a positive revision, one Introduction Node, random slot,
independent recipient HPKE public key and a whole-second validity interval of
at most 600 seconds within the Credential. Its Instance signature is the exact
transcript in the [protected protocol](protected-route-protocol.md#terminal-payloads-and-private-reachability).
The proof contains no data Rendezvous, reusable join or origin address.
`VerifyPrivate` requires the independently selected Target, Network and current
profile; the legacy `Verify` cannot accept this format without those checks.

The State-selected resolution Node receives publication and lookup through a
State-authorized Node Carrier and a fresh confidential terminal role channel.
It requires an actual class-1 admission before one operation, verifies the
Introduction assignment against current State, and invokes the real Store.
Every RESULT occupies 16,384 bytes. Descriptor commit does not prove that its
claimed slot is registered or that a Publisher is ready; registration,
publication acknowledgement and readiness must still be joined by the owning
Endpoint lifecycle.

The Endpoint Administration context owns a separate Introduction-domain Entry/
Interior prefix alongside its Source prefix. Its real issuer client supplies
class-2 forwarding tokens and the class-3 registration token; neither the worker
nor the Introduction Node selects these recipients. Registration owns a fresh
random slot, explicit revision and bounded expiry. WITHDRAW joins the terminal
result and channel cleanup; context loss cancels and joins registration, pending
withdrawal and both prefixes. A failed exact issuance batch remains retryable
without new blinding or allocation; a foreign pending batch cannot be replaced.
The same context now consumes the accepted non-exporting Instance binding and
real REGISTER receipt to commit the existing public Publication proof. Its
Instance root creates an independent volatile X25519 recipient only after the
Instance is durably consumed. Revisions strictly increase in that live Instance;
a restart cannot revive its signer or reset the floor into another live key.
The context signs the exact private Descriptor and sends it through its Source
prefix with real Control issuance and journal admission. The resolution Node
commits its actual Store before acknowledgement. An exact retry retains the
same signed bytes and key; explicit replacement uses a new registration,
recipient and higher revision within the same Publication generation.
Registration completion erases its recipient and joins that erasure on Close.
Context loss additionally withdraws its local Publication and Instance after
joining work. Cleanup ownership precedes a cancellable publication handover;
failed withdrawal retains the binding and original error and closes admission.
Another context cannot take over the selected Instance.
The context schedules refresh from the original registration creation time,
retains a fresh slot/key and strictly higher revision, and switches accepting
readiness only after the exact Descriptor receives a verified acknowledgement.
While publication waits on the network, the context reserves its Instance
without holding the Endpoint publication mutex. Legacy publication operations
cannot take that reservation. The prior published registration remains usable
until that switch, bounded by its original signed expiry. The registration retains the time of its first verified publication acknowledgement;
an exact retry cannot replace this local transition receipt. The switch shortens
its remaining acceptance to at most 60 seconds; an exact retry cannot extend
the cutoff. Unacknowledged replacements cannot accept capsules. Withdrawal and
context loss cancel and join the scheduler, registrations and recipient erasure.

These paths have real Node-network coverage on both Carriers, including old
capsule delivery while a replacement acknowledgement is delayed and refusal of
a new capsule until that acknowledgement. Accelerated scheduling does not prove
elapsed 300-second refresh or 60-second overlap trials. The complete installed
command/readiness transaction and system qualification remain required;
Descriptor acknowledgement alone is not Service Connection readiness.

The Endpoint text context can fetch a Descriptor through its retained admitted
source prefix. It derives the sole resolution recipient from live State, obtains
a real receiver-bound Control token through the issuer, records its attempt in
the context's journal and uses a fresh terminal TLS channel. The returned proof
is checked against the independently selected Target, Network and current
profile. Cancellation joins the operation before context shutdown completes.
Before returning a proof, the live context retains its publication generation,
publication digest, terminal Credential expiry, Descriptor revision and exact
Descriptor hash. It rejects lower generations/revisions and retains separate
publication and revision conflicts. A higher revision can repair only a revision
conflict; a new generation must not overlap any observed conflicting Credential.
The context stores at most 128 Target floors, without eviction or shared private
history. A full context refuses a new Target before issuance or admission;
existing Targets may still advance. Worker loss retains the floors; context
retirement clears them. Expiry never revives a predecessor. No Descriptor bytes
are cached for offline use, and these volatile context facts do not replace the
Store's durable floors or prove restart/migration qualification. The complete
command's protected read remains required; a lookup is not completed Connection
admission. The lookup-only network tests use explicit Publication and
Introduction-slot fixtures.
`PublishPrivate` and `LookupPrivate` retain the existing publication-generation,
no-overlap and same-generation Publication-conflict rules. Within one exact
Publication a higher revision replaces the current revision, including when its
expiry is shorter; a differing proof at the same revision persists as
unavailable. Only a strictly higher revision can repair that revision conflict.
For conflicting Publications at one generation, the Store retains the complete
signed Descriptor whose Credential has the latest observed `NotAfter`, together
with the terminal publication-conflict flag. Further valid conflicts can extend
this expiry floor even after lookup becomes unavailable; a shorter conflict
cannot reduce it. Reopening reconstructs the same signed floor, and a higher
generation must start at or after that expiry. This preserves the existing
record format; previously discarded observations cannot be reconstructed from
an older root and still require explicit adoption evidence.
Expired records retain their floors, never exposing a predecessor. The Store
retains at most 128 Targets, refuses additional Targets before writing, and
continues to permit updates of existing Targets without evicting their floors.

Private stored records use version 2 with separate publication/revision
conflict flags; existing v1/v2 Descriptor records retain stored version 1.
There is no implicit legacy/private format adoption. Before acknowledgement,
the Store syncs the record and containing directory. Initialization also
syncs directory links and creates the marker only after the records directory
is durable. A marked root with missing records, or an unmarked root with
retained records, refuses reopening. Any failed record persistence makes the
current owner unavailable until closed and reopened; it retains its exclusive
lease in the meantime. This does not claim that a storage device survives
failures beyond its filesystem's sync guarantees.

## Recipient-only capsule composition

The reader context now consumes its actual lookup and retained conflict floors
to prepare a capsule for one current qualified job. It selects the data-join
duty from authenticated State through its admitted Source prefix; neither the
Descriptor nor Application supplies a Rendezvous address. The selected Node,
key and known family must differ from the local adjacent peers, resolution,
Introduction and issuer duties. Join secret,
handshake context and delivery nonce are independently generated. The selected
HPKE suite seals the exact fixed plaintext, and its hash supplies the existing
Attachment exporter context.

Before any Publisher dial, the Administration context checks its channel-owned
registration, current private Descriptor and Publication, and opens the capsule
through the Instance's current non-exporting recipient. It independently checks
Target, publication, revision, profile, data-join duty, local job and Work Safety
bounds before constructing the same logical Service context. It rechecks caller,
job and the original capsule expiry after binding, before accepting the nonce.
Reader preparation inherits the job lifetime, interrupts its network flights on
retirement, and joins caller cancellation before handing over capsule bytes.
Opening attempts are limited to four in any second; accepted delivery nonces remain until the
original registration expiry plus 60 seconds, bounded to 2,640 entries per
context. Worker loss does not erase those entries; context retirement does.
The Source submits the capsule over a fresh class-1 Introduction Control channel.
Introduction generates a channel-local request nonce and sends the unchanged
sealed capsule through the Publisher's actual class-3 registration. That owner
bounds queued and active deliveries to 16, limits starts to four per second,
and reserves operation/RESULT/CLOSE plus withdrawal against its original 1 MiB.
Publisher acknowledges after independent capsule acceptance and joins the
terminal child CLOSE before handing over the binding. Both Endpoint exchanges
are job-scoped operations that context retirement joins; failed Source cleanup
is retained and disables new local admission.

Withdrawal disables the receiving slot and closes its completion signal; it
retains the inactive entry to prevent slot reuse. That entry includes the
registration request, rate and byte accounting, and the closed connection
object. Expired entries are reaped when a later registration is reserved, or
released when the receiver itself is discarded. Withdrawal alone is therefore
not a claim of immediate metadata or connection-object erasure. Receiver-state
observations must include these inactive entries as well as pending deliveries.
The fixed format, network submission/delivery and this composition are tested
with real issuer tokens, both Carriers, Instance HPKE and Service TLS. Data
JOIN and the complete installed command
remain required integration. Delivery acknowledgement is not a joined
Attachment or Service readiness.

## Generation-2 implementation
## Purpose and boundary

Given an exact, network-bound Target Link, the Endpoint obtains one
authenticated, current Service Publication and short-lived live introduction
facts. The result is input to the User Route composition; it is neither
a Service Connection nor evidence that the Publisher is online.

The protocol has three roles:

```text
Endpoint -- private lookup Entry --> Initiator -- OHTTP --> Destination Resolution Gateway
                                                            ^
Publisher -- authenticated descriptor publication ----------+
```

The Initiator can observe Endpoint adjacency but not the Target. The Gateway can
observe the Target but not the Endpoint origin. The maintained State assigns
the adjacent Initiator to `initiator` and the Gateway to the separate
`destination-resolution` duty domain. The Gateway identity and known family
are excluded from the later Service Connection's peers. This current C0
assignment is distinct from the public-product model that places Destination
Resolution within the non-adjacent Rendezvous Domain; it does not qualify that
public topology. ADR-0037's Initiator operation carries the opaque lookup, so
the Initiator is not a Rendezvous-domain HTTP Relay.

A lookup uses a separate Isolation Context/channel and a separate Initiator Entry acquisition from the connection
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
try the same authenticated Service Instance; Introduction slot replay controls
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
requires separate lookup and Service Connection attachment identifiers and
rejects the Gateway's identity or family when it overlaps any Service Connection
peer.

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
to the participant-owned Endpoint runtime, which composes the private
Introduction route from its accepted State and Entry facts.

## Required outcomes and qualification

The private-resolution result vocabulary must distinguish `resolved`,
`unavailable`, `stale`, `conflicting`, `invalid evidence`, `private resolution
unavailable`, and local policy/resource refusal without revealing route
topology to the Application. Only a verified `resolved` result may enter the
Endpoint-owned Connection composition; every other result creates no Route
Attachment, Application Connection, or listener.

Before private reachability supports an operational usability claim, a separate
Publisher, User, Gateway, Introduction, Initiator, Rendezvous, and Responder
process experiment must show: success; a changed Target; old still-time-valid
credential; same-generation publication conflict; expired/stale introduction
slot; substituted Entry invite; Gateway withholding; Publisher
withdrawal/offline; no direct Publisher request; and Connection/Route cleanup
on close. The two host Ubuntu run must retain its exact binary and State/profile
evidence.

The maintained implementation and test denominator cover in-process and
bounded local-process success paths, including the closed lookup carrier. The
retired stage-specific Reference C-2 topology is provenance at
[`fbb42034757513ac009114a00b933aefa76d8ddf`](https://github.com/dianabuilds/ardents-network/commit/fbb42034757513ac009114a00b933aefa76d8ddf),
not current qualification. The required failure matrix and two-host Ubuntu
qualification remain unfulfilled. C0 exposes this surface for audit without declaring it
operationally usable. Browser use requires a separate product decision and
Application isolation evidence; it is not a prerequisite for headless C0.

## Non-claims

This is not Namespace resolution, DNS, a public directory, an ordinary
descriptor server, a Publisher-origin hiding guarantee against colluding
Relay/Gateway, a protection against timing/volume correlation, or a guarantee
that the resolved Service stays available. It does not add browser isolation,
content safety, application authorization, replication, offline delivery, or
generic Internet proxying.
