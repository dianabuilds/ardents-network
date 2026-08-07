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
- endpoint-local grants, Application Interface authority, and network metadata;
- route, discovery, bootstrap, and Bridge availability;
- honest-workload latency, throughput, fairness, and endpoint resource
  availability under load;
- Control Plane integrity, software provenance, and real operator diversity.

## Adversaries

We assume the presence of:

- a local or ISP observer seeing one endpoint's traffic;
- a censor blocking, probing, throttling, and fingerprinting network entry;
- malicious, unreliable, or colluding entry, relay, discovery, rendezvous, and
  naming infrastructure;
- a Broad Traffic Observer capable of timing and volume correlation near both
  endpoints or across enough network locations to correlate a connection;
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

The baseline is a low-latency route for live Service Connections. P2-D1 fixes
its outer claim: hide the User's ordinary network location from the Service,
hide the Service Instance's ordinary location from the User, and prevent any
one ordinary intermediary from learning both locations and plaintext
Application Data. Later R-001 decisions must still define the exact conditions,
intermediary knowledge, and collusion boundary.

It does **not** claim resistance to a Broad Traffic Observer correlating timing
and volume near both endpoints or across enough network locations. R-005 must
first justify a concrete Application job before a delayed, padded, or
cover-traffic-heavy profile becomes part of the product.

### Bridge entry

A Bridge provides a replaceable entry path when ordinary network participation
is blocked. It is a circumvention mechanism, not an anonymity guarantee by
itself.

## Threat and response matrix

| Adversary | Representative attack | Required product response | Honest limitation |
|---|---|---|---|
| Censor / DPI | Block known Nodes, bootstrap sources, or protocol fingerprints; probe suspected Bridges | Multiple authenticated bootstrap sources, replaceable Bridges, transport agility, bounded rotation, and explicit blocked state | No fixed protocol disguise or address remains unblockable forever |
| Local observer | Observe entry peer, timing, volume, and long-lived patterns | Encrypted entry, bounded entry policy, multi-hop route, connection isolation, and no direct Service fallback | The observer sees Ardents use and may correlate low-latency traffic with observations elsewhere |
| Broad Traffic Observer | Correlate both endpoint traffic statistically | Make the lack of an Interactive Route correlation-resistance claim visible; measure any later stronger Route Profile separately | Interactive traffic is expected to remain timing- and volume-correlation-sensitive |
| Malicious infrastructure Node | Tag, delay, replay, drop, redirect, bias selection, or retain metadata | End-to-end target authentication and integrity, endpoint-selected safe paths, replay protection, bounded retry, role separation, and diversity analysis | Guarantees depend on the accepted collusion model and real diversity |
| Sybil / flooding actor | Capture discovery or exhaust connection, rendezvous, descriptor, and naming capacity | Bounded queues and lifetimes, quotas or anonymous costs, diversified selection, local admission, and visible overload | No global proof of personhood; accessibility and concentration costs remain |
| Malicious Service | Fingerprint requests, link Application identities, return exploit content, or lie at the application layer | Isolation Context, minimal network metadata, authenticated target, and clear Application boundary | The Service receives application plaintext and can link information that the Application voluntarily sends |
| Malicious local Application | Reuse authority, inspect another app's state, overrun queues, or request unsafe route downgrade | Narrowly scoped Local Grants, separate Authority custody, resource bounds, isolation, and explicit route policy | Code controlling the local endpoint can defeat local protections |
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
- The Interactive Route never claims resistance to timing-and-volume correlation
  by a Broad Traffic Observer; payload encryption is not traffic-analysis
  resistance.
- Every Application Interface operation requires an endpoint-local grant scoped
  to the Application, optional Service, and allowed operations; connection or
  publication access never implies raw Service Authority export.
- An Endpoint Owner controls only one endpoint. No Local Grant, Endpoint Owner,
  Node operator, or sponsor is a network-wide administrator or approval root.
- Joining, connecting, and publishing require no central administrator approval;
  disappearance of one Endpoint Owner cannot block independent endpoints.
- Compromise of an Endpoint Owner grants no network-wide administrative power,
  although the compromised endpoint remains capable of ordinary network attacks
  and loses the Service Authorities it holds.
- Resource budgets are finite and hierarchical; creating Local Grants, Services,
  Isolation Contexts, or connections never multiplies an ancestor budget.
- Slow consumers cause bounded stream backpressure, not unbounded queues or
  silent Application Data loss; overload and fairness outcomes remain explicit.
- Security mechanisms and performance optimizations are measured together;
  neither may bypass target authentication, isolation, least privilege, or
  resource bounds.
- Bounded retry does not create unbounded queues, amplification, or duplicate
  application operations by implication.
- Bootstrap, naming, protocol releases, software distribution, and emergency
  policy are separate Control Plane roots.
- Payload protection is not metadata protection, and independent Node IDs are
  not proof of independent control.

## Open security research

The prioritized questions live in [the network research queue](../research/questions.md).
R-006 fixes the V1 target lifecycle, R-002 fixes the Application Interface, and
R-001 P2-D1 fixes the outer limit of the Interactive Route claim.
No production architecture should be selected before R-001, R-003, R-004,
R-007, R-009, and R-023 make the observer, naming, routing, failure, bootstrap,
and performance contracts testable.
