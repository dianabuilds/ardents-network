# Ardents Product Context

Ardents is a private application network for people, communities, developers,
and independent network contributors. This glossary defines product language;
it intentionally contains no protocol or implementation choices.

## People and identity

**Person**:
A human using Ardents through one or more Devices. A Person has no public,
universal identifier in the network.
_Avoid_: Account, Principal, wallet

**Developer**:
A Person or team that creates and publishes a Private Service.
_Avoid_: Hosting customer, server owner

**Network Contributor**:
A person or organization providing bounded network resources without becoming
the owner of users, services, or protected content.
_Avoid_: Trusted operator, Authority

**Device**:
A replaceable user-controlled endpoint authorized for selected Personas.
_Avoid_: Person, Node identity

**Persona**:
A context-specific identity used in one relationship, Space, or Private Service.
Different Personas are unlinkable by default.
_Avoid_: Global profile, account, permanent Principal

**Recovery Root**:
Person-controlled authority used only to recover or authorize Devices and
Personas, never as an ordinary network identity.
_Avoid_: Account password, network address, session key

**Credential**:
Optional privacy-preserving evidence disclosed to satisfy one policy.
_Avoid_: Identity, permission, universal humanity score

**Capability**:
Finite authority scoped to an exact Service, Space, resource, and class of
action.
_Avoid_: Global role, account type, Credential

## Relationships and spaces

**Contact**:
A verified relationship between two Personas, normally established through an
out-of-band invitation or comparison.
_Avoid_: Follower, public address-book entry

**Space**:
A private collaboration boundary with its own members, Personas, names,
capabilities, and installed Private Services.
_Avoid_: Realm, global community

**Invite**:
A bounded secret or unlinkable proof for discovering or joining one Contact,
Space, or Private Service.
_Avoid_: Public registration token, permanent access key

## Private services and naming

**Private Service**:
A site or application reached through a Service Name while hiding publisher and
visitor location within a declared Route Profile.
_Avoid_: Clearnet website, central backend

**Unlisted Service**:
A Private Service that anyone knowing its exact Service Name may open, while
Ardents provides no directory or search for finding it. Knowing the name is
discovery, not proof of authorization or secrecy.
_Avoid_: Public listing, secret Service Name, capability-gated Service

**Service Name**:
A human-readable hierarchical name that remains stable while a Private Service
moves, rotates keys, or changes Replicas.
_Avoid_: Onion address, IP address, opaque public key

**Namespace**:
A delegated naming boundary with explicit registration, renewal, transfer,
recovery, visibility, and subname policy.
_Avoid_: One global flat username registry, DNS zone owned by one provider

**Name Registry**:
The verifiable source of ownership, expiry, and resolver truth for Namespaces
and Service Names.
_Avoid_: Central domain database, service directory

**Resolver**:
A verifiable mapping from a Service Name to current service metadata without
making network location part of the name.
_Avoid_: Origin address, trusted DNS server

**Service Target**:
The machine-verifiable identity behind a Service Name. It is normally hidden
from human-facing interfaces.
_Avoid_: User-facing domain, server IP

**Site Bundle**:
An immutable release of site content or client application code that can be
authenticated and replicated independently.
_Avoid_: Mutable web root, deployment directory

**Service Instance**:
One replaceable execution instance serving stateful requests for a Private
Service.
_Avoid_: The Service, permanent origin server

## Delivery and routing

**Replica**:
An independently operated holder of protected service, application, or discovery
material that is not trusted to reinterpret it.
_Avoid_: Primary database, authoritative server

**Application Data**:
An opaque payload whose meaning belongs to a Private Service rather than
Ardents; it is not inherently a message, file, command, or social interaction.
_Avoid_: Built-in message, Mailbox item, Node identity

**Rendezvous**:
A temporary meeting point through which two endpoints connect without learning
each other's network location.
_Avoid_: Origin endpoint, reverse proxy

**Route Profile**:
A standardized product contract describing latency, traffic shaping, routing,
and observer-resistance for one operation.
_Avoid_: Anonymous toggle, advanced network settings

**Interactive Route**:
A low-latency Route Profile for browsing and interactive sessions that does not
claim strong protection against global timing correlation.
_Avoid_: Fully anonymous route

**Shielded Route**:
A higher-cost Route Profile for asynchronous operations requiring stronger
sender-receiver metadata unlinkability.
_Avoid_: Fast route, guaranteed invisibility

**Bridge**:
A non-public or replaceable entry path used when ordinary network participation
is blocked or fingerprinted.
_Avoid_: Core relay, trusted gateway
