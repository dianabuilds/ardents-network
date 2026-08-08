# Ardents Network Context

Ardents is a public, independently operated network for location-private
application services. This glossary defines the network product boundary and
contains no protocol, library, or implementation-language choices.

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
optional SDKs may wrap it but do not define it.
_Avoid_: Mandatory SDK, application runtime, network wire protocol

**Connection Interface**:
The least-privileged part of the Application Interface for opening or accepting
Service Connections and exchanging bytes. It cannot expose Service Authority or
Service administration.
_Avoid_: Message API, Service Administration Interface

**Service Administration Interface**:
The separately authorized part of the Application Interface for managing
Service Authority, publication, and Service configuration. Access to the
Connection Interface does not grant access to it.
_Avoid_: Control Plane, Connection Interface

**Local Grant**:
An endpoint-local permission scoped to an Application, optional Service, and
allowed operations. It is not a network credential or approval from Ardents.
_Avoid_: Global admin token, network account, Service Authority

**Service**:
An application-defined function reachable inside Ardents through a Service
Target and, optionally, a Service Name.
_Avoid_: Node, clearnet host, hosted process

**Service Instance**:
One replaceable running instance that accepts live connections for a Service.
_Avoid_: Service identity, permanent origin

**Node**:
Infrastructure that performs one or more network roles such as entry, relay,
discovery, rendezvous, or bridge. A Node identity is not an application address.
_Avoid_: User, Service, peer contact

**Reference Application**:
A small application used to prove the network contract without becoming a
mandatory part of that contract.
_Avoid_: Network core, universal client runtime

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
The durable secret authority whose holder controls one Service Target. Its loss
or compromise requires replacing that target rather than claiming safe revocation.
_Avoid_: User identity, Node key, Service Instance

**Service Name**:
A lowercase ASCII, dot-hierarchical human-readable name in the canonical
Namespace, controlled by a Name Authority, that resolves to a Service Target and
may remain stable while a Service migrates or replaces a compromised target.
_Avoid_: Onion address, IP address, search keyword

**Service Link**:
The explicitly Ardents-scoped shareable form of a Service Name, such as
`ardents://blog.alice`. It is not an ordinary web or DNS address.
_Avoid_: DNS URL, public domain, implicit hostname

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

**Service Descriptor**:
Authenticated, time-bounded network metadata used to contact a Service Target
without revealing an ordinary origin address.
_Avoid_: Name Record, IP endpoint, application profile

## Connections and routing

**Service Connection**:
A live protected, reliable, ordered, bidirectional byte stream bound to an
authenticated Service Target whose lifetime can span bounded replacement of
underlying Carrier Channels. It carries opaque Application Data without message
boundaries and implies no offline delivery, semantic completion, or automatic
Application-operation replay.
_Avoid_: Node-to-node message, conversation, Mailbox

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
A testable product contract for latency, throughput, resource cost, path
behavior, observer resistance, and honest limitations of a class of Service
Connections.
_Avoid_: Anonymous mode, routing algorithm

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
Observer.
_Avoid_: Fully anonymous route, clearnet connection

**Route Knowledge Separation**:
The Interactive Route property in which several separately operated Node roles
each receive only adjacent and role-specific information, so no ordinary Node
can link an endpoint's ordinary location to a Service Name, Service Target, or
the opposite endpoint. Different Node IDs are not proof of separate control.
_Avoid_: Direct P2P path, single trusted proxy, fixed Tor circuit

**Correlated Control**:
Effective control or coordinated observation of multiple Node roles by one
adversary, regardless of distinct Node IDs or advertised operators. It breaks
the single-Node anonymity claim when the combined views span a Route Knowledge
Separation boundary.
_Avoid_: Node count, advertised independence, distinct Node IDs

**Rendezvous**:
A temporary network role through which two endpoints establish a Service
Connection without learning each other's ordinary network location.
_Avoid_: Origin server, reverse proxy

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
