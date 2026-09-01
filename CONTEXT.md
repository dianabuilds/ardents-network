# Ardents Network Context

Ardents is a public, independently operated network for location-private
application services. This glossary defines the network product boundary and
contains no protocol, library, or implementation-language choices.
It describes eventual product language, not current implementation status.
[Product scope](docs/product/scope.md) names the maintained C0 audit boundary;
that candidate is not yet a public or independently operated network.

## Actors and software

**User**:
A human who reaches a Service through an Application connected to Ardents. The
network does not create a public identity for a User.
_Avoid_: Account, Principal, network identity

**Developer**:
A person or team that connects an Application or Service to Ardents.
_Avoid_: Hosting customer, Node operator

**Endpoint Owner**:
The person or system administrator controlling one local Ardents endpoint and
its Application access. This role has no network-wide authority and is not
required by any other endpoint.
_Avoid_: Network administrator, Ardents operator, central approver

**Endpoint**:
One deployed Ardents runtime and protected local-state boundary controlled by an
Endpoint Owner. It scopes Local Grants, Entry Sets, Capability Readiness,
resource accounting, and lifecycle state even when several helper processes are
used. It is not a Person, Device identity, User identity, Service, or Node.
_Avoid_: User account, machine identity, infrastructure peer

**Distribution Profile**:
The supported delivery shape of one Endpoint release: Installed wraps the
platform executable in host package lifecycle, while Portable is that executable
plus only unavoidable authenticated non-secret static configuration templates/
resources, copied and run from an
Owner-chosen path without implicit system integration. When the exact
authenticated executable is run, both expose the same Endpoint capabilities and
claims; pre-execution custody and packaging conveniences differ, and neither
shape makes protected Endpoint state portable.
_Avoid_: Edition, feature tier, portable identity, weaker client

**Client**:
An activated Endpoint capability that initiates Service Connections for local
Applications. It creates no public User identity and is not a separate
installation or trust root.
_Avoid_: User identity, account, initiator Node

**Publisher**:
An activated Endpoint capability that publishes and accepts connections for one
or more explicitly authorized Services. It is not itself a Service identity or
public Contributor Node.
_Avoid_: Service Authority, hosting provider, Responder Node

**Network Contributor**:
A person or organization that supplies a bounded network role without owning
Users, Services, or Application Data.
_Avoid_: Trusted operator, central provider

**Application**:
Software that uses Ardents to reach or provide a Service and owns the meaning,
authorization, persistence, and user experience of its data.
_Avoid_: Ardents plugin, built-in app, Node

**Application Interface**:
The local boundary through which an external Application publishes or opens
Service Connections without embedding Ardents networking logic. It contains
separately authorized Connection and Service Administration Interfaces;
optional SDKs may wrap it but do not define it. Authority Custody is a stronger
separate local boundary and is never implied by this interface.
_Avoid_: Mandatory SDK, application runtime, network wire protocol

**Application Principal**:
An operating-system-enforced or launcher-brokered identity for one local
Application/helper process tree and session to which Local Grants are bound. A
desktop user account, PID, loopback port, or copyable file/token alone is not a
distinct principal. Applications that the platform cannot distinguish are one
local trust domain and receive no malicious-sibling isolation claim.
_Avoid_: User account, process ID, reusable API key, network identity

**Network-Isolated Application Boundary**:
A qualified local attachment or execution boundary in which the complete
Application/helper process tree can communicate only through scoped local IPC
or loopback with its granted Ardents Application Interface. Ordinary network
ingress/listeners and DNS, HTTP, WebSocket, WebRTC, QUIC, or arbitrary socket
egress are denied by default; origin, cache, and storage state do not cross
Isolation Contexts; and an external or secondary request either names an exact
Ardents destination or fails. A generic adapter outside this boundary remains
usable but receives no Application-level Endpoint Location Privacy claim.
_Avoid_: Generic browser proxy, content sanitizer, proof that an Application is anonymous

**Connection Interface**:
The least-privileged part of the Application Interface for opening or accepting
Service Connections and exchanging bytes. It cannot expose Service Authority or
Service administration.
_Avoid_: Message API, Service Administration Interface

**Service Administration Interface**:
The separately authorized part of the Application Interface for managing
publication and Service configuration with an already authorized public
Credential and matching non-exportable private Instance Key. It cannot create,
import, export, or rotate Service Authority, issue a Credential, or export the
Instance Key. Access to the Connection Interface does not grant access to it.
_Avoid_: Authority Custody, Control Plane, Connection Interface

**Authority Custody Boundary**:
The strongest separately granted local operation surface for creating,
importing, exporting, reconciling, or signing with Service and Name Authorities;
issuing bounded public Credentials to host-generated Instance public keys;
rotating/transferring Name Authority; and initiating Service Target replacement.
The first public product has no in-place Service Authority rotation: root replacement creates a new
Authority and Target, while local vault-wrapping keys may rotate independently.
This boundary is outside the ordinary Application Interface and is never implied
by Connection or Service Administration access.
_Avoid_: Service Administration Interface, shared admin token, network administrator

**Authority Vault**:
Protected local or offline storage for Service and Name root material together
with their authority-owned monotonic commitments and signing watermarks. It
contains no Local Grants, Service Instance Keys, Route state, or Application
Data and is accessed only through Authority Custody.
_Avoid_: Endpoint configuration, runtime key store, Recovery Bundle

**Local Grant**:
An endpoint-local permission scoped to an Application, optional Service, and
allowed operations through one Application Principal. Revocation immediately denies new operations and invalidates
all descendant session capabilities. Authority-custody and administration
sessions close immediately; data connections close immediately unless the Owner
first selected a finite drain-then-revoke action. A persistent local policy may
survive restart, but every process/session must be rebound to the authorized
operating-system-local principal and receive fresh ephemeral capability state.
It is not a network credential or approval from Ardents.
_Avoid_: Global admin token, network account, Service Authority

**Service**:
An application-defined function reachable inside Ardents through a Service
Target and, optionally, a Service Name.
_Avoid_: Node, clearnet host, hosted process

**Service Instance**:
One replaceable running instance that accepts live connections for a Service.
_Avoid_: Service identity, permanent origin

**Node**:
Infrastructure that performs one or more compatible network roles such as
entry, relay, discovery, rendezvous, or bridge. A Node identity is not an
application address, and direct-origin source duties cannot be combined with
Route or Destination Resolution eligibility by one identity or known family.
_Avoid_: User, Service, peer contact

**Reference Application**:
A small application used to prove the network contract without becoming a
mandatory part of that contract.
_Avoid_: Network core, universal client runtime

**Product Core**:
The smallest durable Ardents information-flow and responsibility contract,
independent of one implementation or delivery milestone.
_Avoid_: Current backlog, complete public network, V1

**Delivery Horizon**:
An explicit promotion boundary controlling when an accepted product requirement
may enter implementation scope. Decision maturity does not imply horizon entry.
_Avoid_: Priority label, release version, fixed requirement

**Carrier Lab**:
The disposable Ubuntu-only controlled experiment that falsifies the current
Interactive Route candidate before naming or public-network systems are built.
_Avoid_: Reference Application, test network, Ardents release

**Closed Test Network**:
A persistent multi-host but project-controlled Ardents environment used to test
later vertical slices without claiming independent or public operation.
_Avoid_: Public Beta, decentralized network, Carrier Lab

**Public Beta**:
The first externally usable Ardents network allowed to make only the claims that
have passed its public operator, control, platform, and qualification gates.
_Avoid_: Carrier Lab, project-key test network, V1

**Stable Network**:
A Public Beta successor that also passes the stronger diversity, operational,
update, recovery, and external-review gates declared for stable use.
_Avoid_: Prototype, documentation-complete network

**Overlay Service**:
Optional reusable functionality built over Ardents, such as retained delivery,
content replication, group coordination, or application identity.
_Avoid_: Implied network primitive, mandatory core

## Addressing and naming

**Service Target**:
The machine-verifiable, location-independent identity to which an Application
connects. It survives routine migration under the same Service Authority but is
replaced after that authority is lost or compromised.
_Avoid_: Node ID, User ID, IP address

**Service Authority**:
The durable root authority whose holder controls one Service Target and may
authorize bounded Service Instance Credentials. Its loss or compromise requires
replacing that target rather than claiming safe revocation.
_Avoid_: User identity, Node key, Service Instance

**Service Instance Key**:
A private key generated on the active Service host for one bounded Instance
generation. Its public key is authorized by a Service Instance Credential, and
its possession proves that credential in publication and endpoint handshakes.
Routine migration generates a new key and never exports this secret.
_Avoid_: Service Authority, public credential, permanent server key

**Service Instance Credential**:
A public, bounded, monotonic Service-Authority signature binding one Service
Target, Ed25519 Instance public key, separate X25519 Introduction recipient
public key, exclusive generation, validity bounds, network, and allowed
capabilities. The matching Service Instance Key permits publication and
target-authenticated handshakes but not export or replacement of Service
Authority. The Introduction recipient key is not derived from the Instance Key.
_Avoid_: Private key, Service Authority, hosting account

**Authority Recovery Bundle**:
A versioned encrypted backup of Service or Name root material together with its
network/root identity, last authenticated authority-owned generation or revision
commitments, and non-decreasing signing watermarks. It excludes Local Grants and
runtime Instance Keys. Restored authority remains export-only until it reconciles
current authenticated state and can advance monotonically.
_Avoid_: Private-key file, endpoint backup, help-desk recovery token

**Service Name**:
A lowercase ASCII, dot-hierarchical human-readable name in the canonical
Namespace, controlled by a Name Authority, that resolves to a Service Target and
may remain stable while a Service migrates or replaces a compromised target.
_Avoid_: Onion address, IP address, search keyword

**Service Link**:
The explicitly Ardents-scoped shareable form of a Service Name, such as
`ardents://blog.alice`. It is not an ordinary web or DNS address.
_Avoid_: DNS URL, public domain, implicit hostname

**Alpha Service Link**:
An explicitly bounded, non-Namespace shareable alpha reference, such as
`ardents-alpha://blog.alice`. It can select only a pre-provisioned Alpha Name
Corpus and never asserts that its Service Name is current in the canonical
Namespace or publicly registrable.
_Avoid_: Service Link, public name claim, DNS URL

**Alpha Name Corpus**:
One finite, signed, expiring, and explicitly withdrawable set of Alpha Service
Link-to-Service-Target bindings for a declared alpha cohort. It is a disclosed
test input, not a registrar, a partial Namespace, or proof that a name remains
valid after its signed interval.
_Avoid_: Namespace, public directory, hidden registry

**Target Link**:
The explicit shareable Ardents form of a machine-verifiable Service Target. It
bypasses naming but never target authentication, routing, or Application
authorization. Its v1 form is `ardents-target:v1:<base64url>` over exactly a
fixed Target algorithm identifier, 32-byte Ardents network identifier, and
32-byte opaque Target; it is unambiguously distinct from a Service Link and
contains no origin or mutable reachability.
_Avoid_: Service Name, origin address, naming fallback

**Destination Binding**:
The immutable connection provenance of the exact destination form supplied by
the Application. A Name-bound connection records the authenticated Name
generation/revision and Target; a Target-bound connection records only the exact
Target. A changed Name binding can terminate but never silently retarget a live
connection. A Target-bound connection never inherits Name recovery.
_Avoid_: Redirect, fallback destination, mutable connection target

**Name Authority**:
The authority controlling one Service Name's authenticated binding independently
of Service Authority. It is not needed to publish, resolve, or connect to the
Service during ordinary operation.
_Avoid_: Service Authority, User identity, registrar administrator

**Unlisted Service**:
A Service that anyone knowing its exact Service Name may attempt to open while
Ardents provides no index, search, or recommendation mechanism for finding it.
Knowing the name is discovery, not authorization or secrecy.
_Avoid_: Public listing, secret name, capability-gated Service

**Namespace**:
The one network-wide naming boundary in which a complete canonical Service Name
has the same meaning for every honest compatible client and may delegate bounded
subordinate names. It is not a service directory, local alias scope, or mandate
for one administrator.
_Avoid_: Resolver provider, search directory, local alias scope

**Name Lease**:
The time-bounded canonical Namespace state assigning control of one Service Name
to a Name Authority through Active and Grace states until it is Released. It is
not permanent property, human identity, or publication permission.
_Avoid_: Name ownership, User account, registrar approval

**Name Generation**:
The distinct lifetime of a Service Name created by one accepted claim and shared
by its Lease and Name Records. Reclaiming a Released name starts a new generation.
_Avoid_: Record version, permanent name identity, revived lease

**Recovery Policy**:
Optional precommitted, generation-bound rules defining which scoped Recovery
Authorities may replace a Name Authority, with what threshold and visible delay.
_Avoid_: Help desk, network administrator, account recovery

**Recovery Authority**:
Authority scoped to participation in one Recovery Policy. It is not a User
identity, Name Authority, registrar, or network-wide administrator.
_Avoid_: Guardian account, operator privilege, global recovery key

**Recovery Pending**:
The visible bounded state after valid recovery initiation and before its outcome,
during which the Service Name fails closed rather than trusting a possibly
compromised binding.
_Avoid_: Background recovery, silent transfer, resolver warning

**Name Record**:
Name-Authority-authenticated data that binds a Service Name to a current Service
Target without publishing an ordinary network location.
_Avoid_: DNS A record, service listing, origin address

**Resolver**:
The product function that verifies a Service Name and returns its current
Service Target or an explicit resolution failure. Successful resolution is not
itself a successful Service Connection.
_Avoid_: Search engine, trusted DNS server

**Private Resolution**:
Resolution that prevents any one ordinary Node from learning both the querying
endpoint's ordinary location and exact Service Name. It does not make a
predictable name secret or unguessable.
_Avoid_: Secret name, anonymous DNS, unobservable lookup

**Private Reachability Resolution**:
Retrieval of current Service Descriptor/reachability for an exact Service Target
without giving any one ordinary Node both the querying endpoint's origin and the
Target or its publicly testable lookup value. It is required for Target Links as
well as name-derived Targets and has no direct public fallback.
_Avoid_: Direct descriptor server, target secrecy, origin address lookup

**Anonymous Cost**:
A bounded per-operation resource burden that raises abuse cost without relying
on identity, account, payment, IP reputation, or stable cross-context state. It
is not proof of one person or fair access.
_Avoid_: Identity check, registration fee, token stake

**Protocol-reserved Name**:
One of a finite transparent set of names or labels unavailable solely for
versioned protocol safety. It is not a discretionary brand, content, or legal
reservation.
_Avoid_: Trademark hold, administrator block, takedown list

**Service Descriptor**:
Authenticated, time-bounded network metadata used to contact a Service Target
without revealing an ordinary origin address.
_Avoid_: Name Record, IP endpoint, application profile

**Destination Resolution Role**:
A destination-aware infrastructure role that serves authenticated Name/Target
and Service Descriptor lookup or publication without learning endpoint origin.
Public Beta permits it only to Rendezvous-domain identities, never endpoint-adjacent
domains; an endpoint excludes every resolution identity and known family used for
an exact destination/context from that connection's Rendezvous selection. The
same identity or known family cannot also serve a direct-origin source role.
_Avoid_: Direct resolver, fifth endpoint Entry, DNS fallback

**Direct-Origin Source**:
A bootstrap, Candidate Materialization, authenticated-time, release, or other
public-artifact source contacted outside an established Ardents Route. It may
see requester origin, requested artifact, timing, and probable Ardents use, so
a globally advertised source identity and known family serving this duty are
ineligible for every Route position and Destination Resolution during the
overlapping assignment. An ordinary carrier peer that serves bytes is instead
quarantined locally by each Endpoint that contacts it.
_Avoid_: Trusted bootstrap, private resolver, anonymous download

**Direct Source Exposure Set**:
The bounded Endpoint-local set of authenticated Node identities and known
families actually contacted as Direct-Origin Sources. It is never uploaded or
reset by creating an Application or Isolation Context. Members are excluded
from Route and Destination Resolution selection until their finite exposure
lease and all dependent work terminate. Before contact they must not appear in
retained endpoint-adjacent/prepared-role state or live work; source order,
retries, set growth, and candidate skipping are finite and endpoint-precommitted,
with explicit unavailability on exhaustion. Post-readiness destination-sensitive
fetches use private paths. An unauthenticated external source remains an honest
origin-exposure and hidden-control limitation rather than presumed independent.
_Avoid_: Route history upload, unbounded source sampling, privacy proof for first contact

## Connections and routing

**Service Connection**:
A live protected, reliable, ordered, bidirectional byte stream bound to an
authenticated Service Target whose lifetime can span bounded replacement of
underlying Carrier Channels. It carries opaque Application Data without message
boundaries and implies no offline delivery, semantic completion, or automatic
Application-operation replay. Its Work Safety Lease and terminal `not-after`
prevent an old live stream from preserving expired trust indefinitely.
_Avoid_: Node-to-node message, conversation, Mailbox

**Forward Secrecy**:
The requirement that later compromise of Service Authority, Service Instance
Key, Node long-term keys, or recorded ciphertext does not decrypt a completed
Service Connection when both endpoints were honest during it and erased its
ephemeral session/leg keys. It does not protect a live compromised endpoint,
promise post-compromise healing inside an existing connection, or guarantee
physical erasure from memory dumps, swap, hibernation, or snapshots.
_Avoid_: Encryption at rest, endpoint-compromise protection, future impersonation

**Carrier Channel**:
A replaceable transport-specific channel carrying part of a Service Connection
over its current Route. Its lifetime and identity do not define the lifetime,
identity, or Application semantics of the Service Connection.
_Avoid_: Service Connection, Application stream, Route Profile

**Connection Result**:
The authoritative outcome of attempting or ending a Service Connection: success
bound to an authenticated target, clean close, or a bounded failure class;
detected authenticity or integrity violations are never success. It is not a
delivery receipt, an Application result, or a route trace.
_Avoid_: Delivery status, application response, route diagnostics

**Application Data**:
Opaque bytes exchanged for an Application; only that Application defines
whether they represent HTTP, chat, files, commands, or another protocol.
_Avoid_: Built-in message, network command, Node identity

**Endpoint Location Privacy**:
A Route Profile property that prevents the opposite endpoint from learning an
endpoint's ordinary network location and limits how Nodes can link that location
to a Service. It does not hide identity disclosed by Application Data or behavior.
_Avoid_: Total anonymity, content anonymity, endpoint compromise

**Isolation Context**:
A local, network-invisible boundary assigned by an endpoint to an Application
and optionally subdivided by it. Connections in different contexts must not
share linkable routing or session state.
_Avoid_: User account, global Persona, cosmetic privacy setting

**Local Traffic Observer**:
A network-only observer adjacent to one endpoint that can see that endpoint's
ordinary location and external connection metadata, but is not assumed to
observe both ends.
_Avoid_: Endpoint administrator, compromised Device, Broad Traffic Observer

**Broad Traffic Observer**:
A passive adversary able to observe traffic timing and volume near both
endpoints, or across enough network locations to correlate one low-latency
Service Connection.
_Avoid_: Omniscient attacker, malicious Node

**Route Profile**:
A versioned, exact, testable product contract for latency, throughput, resource
cost, path behavior, observer resistance, and honest limitations of a class of
Service Connections. It may select a different Route Implementation without
changing Application Interface or Service Connection semantics; the selected
profile is authenticated and cannot be silently weakened.
_Avoid_: Anonymous mode, user-tunable routing knobs, routing algorithm, silent fallback

**Route Module**:
The logical deep Module below the Service Connection boundary. Its stable
Interface carries connection-scoped protected frames under an exact Route
Profile and Isolation Context while hiding the routing family, hop shape,
introduction, rendezvous, multipath, mixing, padding, and Carrier Channel
Adapters used by its Implementation.
_Avoid_: Application Interface, universal routing language, fixed topology

**Route Qualification**:
The evidence state of a specific implementation candidate after it passes every
required observer, Node-role, endpoint, and active-attack falsification test for
a Route Profile. It does not extend to excluded adversaries, untested builds, or
later changes.
_Avoid_: Security proof, anonymous by design, documentation-only claim

**Qualification Evidence Bundle**:
The immutable, content-addressed evidence record bound to one exact candidate
and its qualification conditions. It contains the precommitted inputs, complete
raw observations, invalidations, and deterministic verdict outputs needed to
recompute its Route Qualification.
_Avoid_: Test report, selected results, log archive

**Interactive Route**:
The low-latency Route Profile intended for live Applications. It does not
promise resistance to timing-and-volume correlation by a Broad Traffic
Observer. Its baseline data path has five logical carrier positions: User
Entry, User Interior, Rendezvous, Service Interior, and Service Entry. This is
an information-flow shape, not a selected routing protocol or proof that the
operators behind different Nodes are independent.
_Avoid_: Fully anonymous route, clearnet connection, three-position route

**Route Knowledge Separation**:
The Interactive Route property in which several separately operated Node roles
each receive only adjacent and role-specific information, so one ordinary Node
acting only from its role-local view is not directly given an endpoint-origin-to-
Name/Target/opposite-endpoint binding. The adversary controls or observes no
endpoint, second position, or active probe source. Node-plus-endpoint timing/
volume confirmation is an explicit non-claim. A public Target need not remain
unknown to every operator, and different Node IDs do not prove separate control.
_Avoid_: Direct P2P path, single trusted proxy, traffic-confirmation resistance,
Target secrecy, fixed Tor circuit

**Introduction Role**:
A route-control role that holds an expiring service-specific opaque slot and
forwards a sealed connection invitation without receiving either endpoint's
ordinary location. Because an Unlisted Service is public to anyone who knows or
guesses its exact name, the operator may independently know its Service Name or
Service Target; the role must not link that knowledge to either endpoint origin.
_Avoid_: Service Entry, access-control service, secret Target guarantee

**Introduction Path**:
A temporary route-control path, separate from the Rendezvous data path, through
which a User delivers one sealed, expiring, single-use connection invitation to
an Introduction Role and the Service. It carries no Application Data and
provides no offline storage or delivery.
_Avoid_: Rendezvous forwarding, message mailbox, data path

**Correlated Control**:
Effective control or coordinated observation of multiple Node roles by one
adversary, regardless of distinct Node IDs or advertised operators. It breaks
the single-Node anonymity claim when the combined views span a Route Knowledge
Separation boundary.
_Avoid_: Node count, advertised independence, distinct Node IDs

**Rendezvous**:
A temporary network role, selected by the User for one new Service Connection,
through which two endpoint-selected legs join without revealing either ordinary
network location to the other endpoint. Its state may survive bounded
Introduction retry or leg replacement only while that attempt or connection is
still alive; it is never pooled across completed connections.
_Avoid_: Origin server, reverse proxy, reusable connection pool

**Entry Set**:
A small, long-lived, endpoint-selected set of endpoint-adjacent Nodes or Bridges.
Public Beta has ordinary and Bridge regimes and permits at most one set for each activated
adjacent Role Domain and regime in an installation. A client uses the Initiator
domain; publication uses separate Responder and Introduction domains, including
when client and Publisher are co-resident. Applications, Services, Targets,
Isolation Contexts, and Bridge Invites do not create additional sets. Channels
and higher Route state remain separated. A single failure does not rotate a set
to an untried Node.
_Avoid_: Fresh Entry per connection or context, network-global Entry pool,
permanent Node

**Interior Set**:
A small, medium-lived, endpoint-selected rolling set between an Entry Set and
connection-specific roles. It is Isolation-Context- or Service-role-scoped,
rotates more often than Entry, and avoids a fresh unbounded sample on every
Route.
_Avoid_: Permanent middle, fresh random middle per retry, user-tunable hop

**Role Domain**:
A stable, publicly verifiable Node-eligibility class separating Initiator,
Rendezvous, Responder, and Introduction exposure. Destination Resolution is a
role restricted to the non-adjacent Rendezvous Domain and excluded from acting as
the same connection's Rendezvous. One Node Identity and one honestly declared
operator family occupy only one domain during its assignment lifetime; this
prevents same-identity cross-leg or Entry-plus-lookup overlap but does not prove
independent control.
_Avoid_: Trust tier, manual allowlist, Node role chosen per connection

**Role Domain Assignment**:
An authenticated finite assignment of one Node Identity and honestly declared
operator family to one Role Domain. New duty is allowed only when its maximum
terminal lifetime fits before assignment `not-after`. Reassignment first stops
new work and drains/quarantines the identity and known family until every old-
domain duty expires; neither is eligible in a new domain during overlap.
_Avoid_: Instant domain flip, per-connection domain, overlapping eligibility

**Bridge**:
A non-public or replaceable entry path used when ordinary ways of joining
Ardents are blocked or fingerprinted.
_Avoid_: Trusted gateway, permanent bootstrap Node

**Transport Camouflage**:
A best-effort property that makes confident classification or blocking of
Ardents require active analysis or meaningful collateral blocking of ordinary
traffic. It is not a promise of invisibility or guaranteed indistinguishability.
_Avoid_: Invisible traffic, guaranteed HTTPS disguise

**Control Plane**:
The mechanisms and authorities that govern network discovery, naming,
bootstrap, software releases, compatibility, and emergency changes.
_Avoid_: The data path, invisible governance

**Network Epoch**:
An expiring, content-addressed, threshold-authenticated statement of shared
network identity, compatibility, eligibility inputs, Role Domains, and freshness.
Its distributors provide identical bytes but do not define their authority.
_Avoid_: Bootstrap server response, peer list, permanent address list

**Transit Grant Issuer**:
A State-selected Node duty that uses one finite, purpose-scoped online key to
issue one-use Transit Grants without possessing Network State authority. Its
State-bound public profile permits exactly one Initiator ingress, while its
private root, durable budget, and request reconciliation end with that exact
duty generation.
_Avoid_: Network State signer, enrollment registrar, Route planner, Target service

**Node Record**:
A Node-authenticated, expiring declaration of its key, supported capabilities,
transports, declared operator family, and finite capacity. Publication makes a
Node discoverable, not automatically eligible, independent, or trusted.
_Avoid_: User profile, route assignment, reputation account

**Candidate View**:
The logical complete, canonically ordered Node/evidence set committed by one
exact Network Epoch and Route Profile, including authenticated global counts and
concentration summaries. Full auditors may materialize and recalculate it; one
ordinary endpoint need not download it in full.
_Avoid_: Downloaded peer list, Route, bootstrap peer opinion, User history

**Candidate Materialization**:
The deterministic shards or indexed records and inclusion proofs fetched by one
endpoint under a Candidate View commitment. The endpoint verifies the requested
material and selection indices, not global completeness; threshold state and
independent full auditors cover the global commitment. Withholding retries the
same index elsewhere or fails explicitly and never causes silent resampling.
_Avoid_: Personalized Candidate View, distributor-selected route, reputation

**Public-control Candidate**:
A proposed public Control Plane roster, threshold operation, Candidate View,
package, and evidence set that can be inspected independently. It is qualified
only after real independent custodians, builders, and full auditors corroborate
their control boundaries; project keys, VPS, CI, Docker, and a Product Owner
walkthrough are not such evidence.
_Avoid_: Project multi-key, self-certified decentralization, alpha promotion

**Time Confidence**:
The endpoint's bounded evidence that freshness decisions are safe, derived from
monotonic elapsed time, a non-decreasing accepted watermark, Network Epoch
bounds, and optional authenticated time observations.
_Avoid_: Trusted wall clock, one NTP server, exact location time

**Release Safety State**:
Expiring authenticated public metadata proving that an active build remains
recognized and not revoked, independently of where those same bytes were
obtained. A still-valid cached state permits restart during distributor outage;
expired or conflicting state blocks new network work explicitly.
_Avoid_: Vendor ping, auto-install permission, package-store opinion

**Alpha Enrollment Pin**:
A one-cohort, one-release commitment delivered to an invited participant
independently of distribution, used only to authenticate the first Portable
bundle before local Release Safety State exists. It authorizes no successor and
provides no public or independent release-control claim.
_Avoid_: GitHub checksum, bootstrap certificate, permanent release key

**Work Safety Lease**:
The finite authenticated interval during which an existing Route, Service
Connection, publication, or Contributor duty may continue. It ends no later
than the earliest applicable Network Epoch, Release Safety, protocol/build,
Service Instance Credential, Name-bound Destination Binding, or other
role-specific terminal bound, including Role Domain Assignment. A fresh
authenticated state may extend it before expiry; stale, uncertain, or revoked
state never does. New leg attachment or recovery requires a current Common
Readiness Base and applicable credential, not merely an unexpired old stream.
_Avoid_: Immortal session, cached trust forever, update grace without deadline

**Common Readiness Base**:
The prerequisites shared by network capabilities: authenticated active build,
current non-revoked Release Safety State, compatible Network Epoch, sufficient
Time Confidence, and finite local resources. It contains no Initiator,
Publisher, naming, or Contributor path by itself.
_Avoid_: Target Connect Ready, process running, universal network readiness

**Capability Readiness**:
An authenticated local state saying that one named function, such as Target
Connect, Private Name Resolution, Publish, or Contribute, can be attempted under
an exact Network Epoch and qualified profile. Each capability adds its own
role-specific prerequisites to the Common Readiness Base; one capability never
inherits another's Entry path merely because both run in one process.
_Avoid_: Process running, UI ready, universal network readiness
