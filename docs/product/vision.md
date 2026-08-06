# Product vision

Status: **proposed product contract**

Last reviewed: 2026-08-06

## Vision

Ardents enables people and developers to create private worlds—relationships,
spaces, sites, and applications—that do not depend on one provider and do not
require exposing a universal identity or network location.

Ardents is an internal application network. It is not primarily an Internet
proxy, cryptocurrency, hosting marketplace, or messenger. Messaging, naming,
private sites, and application execution are coordinated surfaces of one
product.

## Product boundary

The carrier is public and open to independent Network Contributors. Privacy is
created by a sufficiently broad anonymity set, independent paths, endpoint
isolation, traffic protection appropriate to the operation, and explicit
control-plane governance.

Private Services and Spaces can be undiscoverable or capability-gated. A private
service does not require the underlying carrier to be a small private network.

## People we serve

### Person

A Person wants to communicate and use applications without registering a phone,
email, wallet, or universal public profile. Network internals should remain
invisible during normal use.

### Developer

A Developer wants to publish and update a site or application without operating
a stable public origin and without becoming an anonymity-protocol expert.

### Space steward

A steward wants to create a private collaboration boundary, invite and revoke
members, delegate names, install services, and choose local abuse policy without
becoming a global identity authority.

### Network Contributor

A Contributor wants to provide bounded relay, storage, bridge, or execution
resources without receiving access to protected content or ownership of users.

## Product surfaces

These are responsibility boundaries, not necessarily separate binaries:

- **Ardents Client** — identity, names, private applications, contacts,
  messaging, Spaces, permissions, and route selection.
- **Developer Studio** — names, packaging, privacy lint, releases, replication,
  updates, and eventually stateful services.
- **Space Console** — membership, delegation, installed services, and local
  admission policy.
- **Contributor Node** — explicit resource roles, limits, health, updates, and
  graceful exit.
- **Network Transparency** — control roots, operator diversity, governance, and
  measurable privacy posture.

## Core product promises

1. Protected content is confidential and authenticated end to end.
2. A Service Name is human-readable, location-independent, recoverable, and
   verifiable without requiring a wallet.
3. Person, Device, Persona, Contact, service, and route identities are not one
   globally linkable identifier.
4. A visitor and publisher do not learn each other's network location within the
   declared Route Profile.
5. Losing or blocking one ordinary relay or Replica does not require a central
   operator to restore a service.
6. Recovery and revocation do not require exposing a universal identity.
7. Censorship, malicious infrastructure, Sybil pressure, seizure, and governance
   capture are normal design inputs, not exceptional incidents.
8. Every anonymity claim states its observer model and honest limitation.

## First tracer: Named Private Site + Anonymous Mailbox

The first product slice must prove the whole value chain:

1. A Developer obtains a human-readable Service Name and defines recovery.
2. The Developer builds, inspects, signs, and publishes an immutable Site Bundle.
3. Independent Replicas retain the release and current service metadata.
4. A Person enters the name; the Client verifies resolution and opens the bundle
   through an Interactive Route.
5. The Person grants a bounded mailbox capability and sends a protected message
   through a Shielded Route.
6. The service replies while the Person is offline; the Client receives it later.
7. The Developer publishes an update and can recover control without changing
   the Service Name.
8. The journey still succeeds when one ordinary relay is blocked and one Replica
   is unavailable.

This tracer does not require a general stateful backend, large groups, payments,
or a clearnet exit.

## Build versus adopt

Ardents should build only the product semantics and integrations that make its
promise unique. Proven community components are preferred for cryptography,
secure storage, transport primitives, sandboxing, serialization, and standard
protocol machinery when evidence shows that their threat model and maintenance
model fit.

No dependency is accepted because it is familiar, already present in `old`, or
popular. No component is rejected merely because it was not written by this
project.

## Explicit non-goals for the first product

- clearnet exit, VPN, or general anonymous Internet access;
- mandatory blockchain, public wallet linkage, token, or governance coin;
- global username or proof-of-personhood registry;
- opaque cryptographic addresses as the ordinary human experience;
- public social feed or large public communities;
- generic cloud or arbitrary decentralized compute;
- configurable anonymity knobs that silently create fingerprintable users;
- an unmeasured claim of protection from a global passive observer;
- compatibility with the architecture or wire contracts in `old`.

## What would falsify this direction

The product direction must be reconsidered if research shows that at least one
of the following is unavoidable:

- human-readable naming necessarily creates a globally linkable owner or query
  graph;
- a useful Interactive Route cannot meet acceptable latency while hiding both
  endpoints from the declared adversary;
- safe application isolation requires capabilities so restrictive that useful
  private applications cannot be built;
- the contributor population cannot plausibly create independent operator,
  network, and jurisdiction diversity;
- recovery, multi-device operation, forward secrecy, and unlinkable Personas
  cannot coexist under a usable human workflow.
