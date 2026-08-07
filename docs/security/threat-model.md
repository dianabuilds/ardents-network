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

- a Local Traffic Observer or ISP seeing one endpoint's ordinary location and
  external network connections;
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

The baseline is a low-latency, multi-hop route for live Service Connections.
P2-D1 fixes its outer claim: hide the User's ordinary network location from the
Service and the Service Instance's ordinary location from the User. P2-D3 adds
Route Knowledge Separation: no one ordinary Node receives the full Route,
plaintext Application Data, or a link between an endpoint's ordinary location
and a Service Name, Service Target, or opposite endpoint. The routing family
and hop count remain open.

P2-D4 limits that anonymity claim to any one malicious ordinary Node. Ardents
does not claim resistance to every pair or larger colluding set. Correlated
Control spanning both endpoint-adjacent views may link the User and Service
through timing and volume; other combinations break the claim when their merged
views cross the same knowledge boundary. End-to-end payload confidentiality,
integrity, and Service Target authentication remain required even if all carrier
Nodes collude, although those Nodes may still deny service or manipulate traffic
subject to the later P2-D6 contract.

It does **not** claim resistance to a Broad Traffic Observer correlating timing
and volume near both endpoints or across enough network locations. R-005 must
first justify a concrete Application job before a delayed, padded, or
cover-traffic-heavy profile becomes part of the product.

### Bridge entry

A Bridge provides a replaceable entry path when ordinary network participation
is blocked. Together with replaceable transports it may provide Transport
Camouflage, but this is a best-effort circumvention property rather than an
anonymity or indistinguishability guarantee.

## Threat and response matrix

| Adversary | Representative attack | Required product response | Honest limitation |
|---|---|---|---|
| Censor / DPI | Block known Nodes, bootstrap sources, or protocol fingerprints; probe suspected Bridges | Multiple authenticated bootstrap sources, replaceable Bridges, transport agility, bounded rotation, and explicit blocked state | No fixed protocol disguise or address remains unblockable forever |
| Local Traffic Observer | Observe the adjacent endpoint's location, external peer addresses, timing, direction, duration, volume, retries, and long-lived patterns; attempt to classify Ardents use | Encrypt protocol and Application Data; hide the selected Service Name or Service Target, opposite endpoint location, and full Route; prohibit direct Service fallback; avoid one mandatory stable fingerprint | Ardents use may still be classified or inferred, and low-latency traffic may be correlated with observations elsewhere |
| Broad Traffic Observer | Correlate both endpoint traffic statistically | Make the lack of an Interactive Route correlation-resistance claim visible; measure any later stronger Route Profile separately | Interactive traffic is expected to remain timing- and volume-correlation-sensitive |
| Malicious infrastructure Node | Combine endpoint location, Service Name or Service Target, Route, or payload knowledge; tag, delay, replay, drop, redirect, bias selection, or retain metadata | Multi-hop Route Knowledge Separation, end-to-end target authentication and payload protection, short-lived opaque route handles, bounded retry, role separation, and diversity analysis | The Node sees its immediate peers and traffic metadata; guarantees depend on the accepted collusion model and real diversity |
| Correlated Control | Combine the permitted views of nominally different Nodes, especially both endpoint-adjacent roles, and correlate timing or volume | Avoid correlated route positions using operator, network, software, and jurisdiction evidence; expose uncertainty; test concentration under R-011 | V1 makes no anonymity guarantee against every pair or larger set; hidden common control cannot always be detected |
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
- A Local Traffic Observer may know the adjacent endpoint's ordinary location
  and connection metadata, but the protocol does not directly expose the
  selected Service Name or Service Target, opposite endpoint location,
  Application Data, or full Route.
- An Interactive Route is never a direct endpoint-to-endpoint path or one
  trusted proxy. It uses multiple separately operated Node roles for Route
  Knowledge Separation without prescribing a routing algorithm or hop count.
- An endpoint-adjacent Node may know that endpoint's ordinary location, and an
  interior or Rendezvous role may know its adjacent Nodes. No ordinary Node also
  receives the Service Name or Service Target, opposite endpoint location, full
  Route, or plaintext Application Data for that connection.
- Ordinary Nodes use only the role data and short-lived opaque route handles
  required for the connection. Combining incompatible role views in one Node
  must not bypass the single-Node claim.
- Different Node IDs do not prove independent control. The claim depends on
  actual non-collusion and diversity measured under R-011.
- The Interactive Route anonymity claim covers one malicious ordinary Node. It
  makes no blanket claim against two or more colluding Nodes, and does not imply
  that every colluding pair necessarily holds useful combined views.
- Correlated Control spanning both endpoint-adjacent roles may link the User and
  Service through traffic metadata. An endpoint cannot always detect or report
  that this correlation occurred.
- Carrier collusion does not weaken end-to-end Application Data confidentiality,
  integrity, or Service Target authentication while endpoints, Service
  Authority, and accepted cryptography remain uncompromised. It can still break
  anonymity or availability.
- Transport Camouflage is best-effort. Ardents avoids one mandatory stable
  fingerprint but never claims invisibility or guaranteed indistinguishability
  from ordinary Internet traffic.
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
R-001 P2-D1 through P2-D4 fix its broad-observer, local-observer, single-Node,
and collusion limits.
No production architecture should be selected before R-001, R-003, R-004,
R-007, R-009, and R-023 make the observer, naming, routing, failure, bootstrap,
and performance contracts testable.
