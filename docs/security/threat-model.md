# Threat model

Status: **proposed; research must turn goals into measurable contracts**

Last reviewed: 2026-08-07

## Scope

This threat model covers the minimum network product: joining Ardents,
publishing a live Service, resolving an exact Service Name, establishing an
Interactive Route, exchanging Application Data, recovering from path failure,
and contributing infrastructure.

It does not assume offline delivery, replicated application content, a bundled
application runtime, a User identity system, or a second high-latency route.
Those receive separate threat models only if their product contracts are later
accepted.

## Protected assets

- confidentiality, integrity, and Service Target authenticity of Application
  Data on a Service Connection;
- ordinary network location of the User and Service Instance;
- unlinkability between distinct Isolation Contexts to the extent promised by a
  Route Profile;
- Service Authority secrecy and Service Target continuity;
- Service Name binding, resolution integrity, and recovery state;
- local Application Interface authority and network metadata;
- route, discovery, bootstrap, and Bridge availability;
- Control Plane integrity, software provenance, and real operator diversity.

## Adversaries

We assume the presence of:

- a local or ISP observer seeing one endpoint's traffic;
- a censor blocking, probing, throttling, and fingerprinting network entry;
- malicious, unreliable, or colluding entry, relay, discovery, rendezvous, and
  naming infrastructure;
- a broad passive observer capable of timing and volume correlation across
  multiple network locations;
- Sybil actors able to create many infrastructure identities;
- a malicious User Application or Service attempting metadata access, target
  substitution, resource exhaustion, or cross-context linkage;
- a fully compromised local endpoint or Service host;
- infrastructure seizure, legal coercion, operator disappearance, and network
  partition;
- dependency, build, signing, or update-channel compromise;
- capture of naming, bootstrap, protocol release, or emergency governance.

We do not infer independent ownership, network, jurisdiction, or software merely
because Nodes advertise different identifiers.

## Baseline Route Profile

### Interactive Route

The baseline is a low-latency route for live Service Connections. Its intended
claim is to hide the User's ordinary network location from the Service, hide the
Service Instance's ordinary location from the User, and prevent any one ordinary
intermediary from learning both locations and plaintext Application Data under
the conditions accepted by R-001.

It does **not** claim strong unlinkability against a broad observer correlating
timing and volume. R-005 must first justify a concrete Application job before a
delayed or cover-traffic-heavy profile becomes part of the product.

### Bridge entry

A Bridge provides a replaceable entry path when ordinary network participation
is blocked. It is a circumvention mechanism, not an anonymity guarantee by
itself.

## Threat and response matrix

| Adversary | Representative attack | Required product response | Honest limitation |
|---|---|---|---|
| Censor / DPI | Block known Nodes, bootstrap sources, or protocol fingerprints; probe suspected Bridges | Multiple authenticated bootstrap sources, replaceable Bridges, transport agility, bounded rotation, and explicit blocked state | No fixed protocol disguise or address remains unblockable forever |
| Local observer | Observe entry peer, timing, volume, and long-lived patterns | Encrypted entry, bounded entry policy, multi-hop route, connection isolation, and no direct Service fallback | The observer sees Ardents use and may correlate low-latency traffic with observations elsewhere |
| Broad passive observer | Correlate both endpoint traffic statistically | Make the lack of a baseline global-correlation claim visible; measure any later stronger Route Profile separately | Interactive traffic is expected to remain timing- and volume-correlation-sensitive |
| Malicious infrastructure Node | Tag, delay, replay, drop, redirect, bias selection, or retain metadata | End-to-end target authentication and integrity, endpoint-selected safe paths, replay protection, bounded retry, role separation, and diversity analysis | Guarantees depend on the accepted collusion model and real diversity |
| Sybil / flooding actor | Capture discovery or exhaust connection, rendezvous, descriptor, and naming capacity | Bounded queues and lifetimes, quotas or anonymous costs, diversified selection, local admission, and visible overload | No global proof of personhood; accessibility and concentration costs remain |
| Malicious Service | Fingerprint requests, link Application identities, return exploit content, or lie at the application layer | Isolation Context, minimal network metadata, authenticated target, and clear Application boundary | The Service receives application plaintext and can link information that the Application voluntarily sends |
| Malicious local Application | Reuse authority, inspect another app's state, overrun queues, or request unsafe route downgrade | Local interface authentication, per-Application authority, resource bounds, isolation, and explicit route policy | Code controlling the local endpoint can defeat local protections |
| Compromised Service host | Copy the V1 Service Authority, observe Users' application data, or continue impersonating the Service after migration | Treat the Service Target as compromised, create a replacement authority and target, and rebind the Service Name; never claim same-target revocation | A compromised live Service reads intended plaintext; the old target remains impersonable, and recovery depends on trustworthy name replacement |
| Operator loss / seizure | Remove Nodes, inspect state, or partition reachability | No plaintext at carrier Nodes, replaceable roles, bounded state, alternate paths, and explicit unavailable results | Real availability still requires independent capacity and a live Service Instance |
| Supply-chain attacker | Ship a malicious official endpoint or protocol update | Reproducible artifacts, signed releases, staged updates, rollback protection, transparent roots, and later independent review | One widely trusted distribution root remains power until diversified |
| Governance capture | Control naming, bootstrap, compatibility, releases, or emergencies | Separate power map, bounded quorum, transparency, expiry, recovery, and fork procedure | A decentralized data path does not remove Control Plane governance |

## Claim format

No document or interface may say only “anonymous,” “private,” “secure,” or
“decentralized.” A durable claim must state:

1. **Information:** what is protected or kept available?
2. **Adversary:** from whom?
3. **Conditions:** required honest Nodes, traffic, diversity, time, and endpoint
   behavior.
4. **Measurement:** what experiment or analysis can falsify the claim?
5. **Limitation:** what remains visible, linkable, or attackable?

## Security invariants

- A Node identity is never a User identity or Service Target.
- Possession of the V1 Service Authority is sufficient to impersonate its
  Service Target; suspected loss or compromise requires target replacement.
- A Service Name is discovery, not Service authorization or human identity.
- Name Records and Service Descriptors never contain an ordinary public origin
  address.
- Carrier Nodes cannot reinterpret or forge Application Data accepted by an
  endpoint as belonging to the authenticated Service Connection.
- A Service Connection is live: a partial write, clean transport close, or
  explicit failure never means that an Application operation was retained,
  received, or completed.
- Connection failures expose only supported product-level classes, never Node
  identities or route topology; an indistinguishable cause is reported as
  indeterminate rather than guessed.
- Offline delivery, replicated content, and application history do not appear
  without a separate retention, deletion, abuse, and metadata contract.
- Route downgrade and loss of endpoint authentication are explicit and cannot
  occur silently.
- Every locally authorized Application receives a distinct default Isolation
  Context; missing explicit input never selects a global shared context.
- Isolation Contexts are local policy boundaries, not network-visible User or
  Service identities, and different contexts do not share linkable route or
  session state.
- Isolation Context separation does not defeat correlation through Application
  Data, timing, volume, or observation of the local endpoint's network traffic.
- Bounded retry does not create unbounded queues, amplification, or duplicate
  application operations by implication.
- Bootstrap, naming, protocol releases, software distribution, and emergency
  policy are separate Control Plane roots.
- Payload protection is not metadata protection, and independent Node IDs are
  not proof of independent control.

## Open security research

The prioritized questions live in [the network research queue](../research/questions.md).
R-006 fixes the V1 target lifecycle. No production architecture should be
selected before R-002, R-001, R-003, R-004, R-007, and R-009 make the
connection, observer, naming, routing, failure, and bootstrap contracts testable.
