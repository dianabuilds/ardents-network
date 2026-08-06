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
Service Connections without embedding Ardents networking logic. Optional SDKs
may wrap this boundary but do not define it.
_Avoid_: Mandatory SDK, application runtime, network wire protocol

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
A human-readable name that resolves to a Service Target and may remain stable
while a Service migrates or replaces a compromised target.
_Avoid_: Onion address, IP address, search keyword

**Unlisted Service**:
A Service that anyone knowing its exact Service Name may attempt to open while
Ardents provides no index, search, or recommendation mechanism for finding it.
Knowing the name is discovery, not authorization or secrecy.
_Avoid_: Public listing, secret name, capability-gated Service

**Namespace**:
A naming boundary with an explicit policy for registration, delegation,
renewal, transfer, recovery, and disputes.
_Avoid_: Service directory, global username registry

**Name Record**:
Authenticated naming data that binds a Service Name to a current Service Target
without publishing an ordinary network location.
_Avoid_: DNS A record, service listing, origin address

**Resolver**:
The product function that verifies a Service Name and returns its current
Service Target or an explicit resolution failure.
_Avoid_: Search engine, trusted DNS server

**Service Descriptor**:
Authenticated, time-bounded network metadata used to contact a Service Target
without revealing an ordinary origin address.
_Avoid_: Name Record, IP endpoint, application profile

## Connections and routing

**Service Connection**:
A live protected channel between an Application and a Service Target. It carries
opaque Application Data and does not imply offline delivery, history, or any
application interaction model.
_Avoid_: Node-to-node message, conversation, Mailbox

**Application Data**:
Opaque bytes exchanged for an Application; only that Application defines
whether they represent HTTP, chat, files, commands, or another protocol.
_Avoid_: Built-in message, network command, Node identity

**Isolation Context**:
An application-supplied boundary stating which connections must not become
linkable through shared routing state.
_Avoid_: User account, global Persona, cosmetic privacy setting

**Route Profile**:
A testable product contract for latency, path behavior, observer resistance,
and honest limitations of a class of Service Connections.
_Avoid_: Anonymous mode, routing algorithm

**Interactive Route**:
The low-latency Route Profile intended for live applications. It does not imply
resistance to a global timing-and-volume observer.
_Avoid_: Fully anonymous route, clearnet connection

**Rendezvous**:
A temporary network role through which two endpoints establish a Service
Connection without learning each other's ordinary network location.
_Avoid_: Origin server, reverse proxy

**Bridge**:
A non-public or replaceable entry path used when ordinary ways of joining
Ardents are blocked or fingerprinted.
_Avoid_: Trusted gateway, permanent bootstrap Node

**Control Plane**:
The mechanisms and authorities that govern network discovery, naming,
bootstrap, software releases, compatibility, and emergency changes.
_Avoid_: The data path, invisible governance
