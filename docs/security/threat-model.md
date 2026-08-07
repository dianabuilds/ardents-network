# Threat model

Status: **proposed; Interactive Route contract decided, no implementation qualified**

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
under the P2-D6 contract.

P2-D5 limits the endpoint claim to Endpoint Location Privacy. An intended
Service necessarily receives its plaintext Application Data and connection
behavior; a User receives the Service's output and knows the selected Service
Name or Service Target. Ardents adds no stable User ID, exposes no Isolation
Context or Route to the Service, and exposes no Service Instance origin, Route,
or Service Authority to the User. Application credentials, content,
fingerprinting, timing, and behavior can still identify or link participants.

P2-D6 requires fail-closed authenticity, integrity, freshness, and Route Profile
binding. Modified, injected, replayed, redirected, or downgraded protocol data
must not become an accepted Service Connection or Application Data. A malicious
Node can still delay, drop, block, and shape traffic. When attack and ordinary
failure are indistinguishable, Ardents reports indeterminate failure rather than
an accusation; bounded recovery never replays an Application operation.

P2-D7 makes this a qualification gate rather than a design assertion. A
specific implementation candidate must reproduce the accepted User-edge,
Service-edge, Node-role, malicious-endpoint, Isolation Context, and active-attack
tests before it can present the Interactive Route claim as implemented. Any
forbidden disclosure or silently accepted substitution, modification, replay,
redirect, or downgrade fails Route Qualification. Broad Traffic Observer and
sufficiently placed collusion correlation remain explicit excluded cases rather
than hidden passes.

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
| Malicious infrastructure Node | Combine endpoint location, Service Name or Service Target, Route, or payload knowledge; tag, modify, inject, delay, replay, drop, redirect, downgrade, bias selection, or retain metadata | Multi-hop Route Knowledge Separation; authenticated fresh protocol state; end-to-end target authentication and payload integrity; fail-closed downgrade rejection; short-lived opaque route handles; bounded retry; role separation; diversity analysis | The Node can always deny, delay, or shape traffic; timing and volume tags may aid correlation without producing a distinguishable integrity violation |
| Correlated Control | Combine the permitted views of nominally different Nodes, especially both endpoint-adjacent roles, and correlate timing or volume | Avoid correlated route positions using operator, network, software, and jurisdiction evidence; expose uncertainty; test concentration under R-011 | V1 makes no anonymity guarantee against every pair or larger set; hidden common control cannot always be detected |
| Sybil / flooding actor | Capture discovery or exhaust connection, rendezvous, descriptor, and naming capacity | Bounded queues and lifetimes, quotas or anonymous costs, diversified selection, local admission, and visible overload | No global proof of personhood; accessibility and concentration costs remain |
| Malicious Service | Fingerprint requests, link credentials or behavior, return exploit content, or lie at the Application layer | Hide User origin, Route, Isolation Context, and network-generated stable User identifiers; isolate network state; authenticate the Service Target; keep content semantics outside the carrier | The Service receives intended Application Data, timing, volume, and behavior and can link what the Application reveals |
| Malicious User | Probe Service behavior, exploit the Application, exhaust its exposed operations, or attempt to discover its origin or authority | Hide Service Instance origin, Route, and Service Authority; expose only the authenticated Service Target and Application response; enforce carrier resource limits | The User already knows the supplied Service Name or Service Target and sees all Application output intended for it |
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

## Interactive Route qualification gate

Route Qualification requires a controlled topology that records the tested
build, configuration, workload, Route Profile, role placement, and observation
conditions. The evidence must include:

- traffic captures at both endpoint edges and every ordinary Node role;
- inspection of the live and retained state available to each Node role while
  each eligible role is malicious in turn;
- malicious User and Service observations, including Application Interface
  results, diagnostics, route artifacts, and repeated connections;
- distinct-Isolation-Context comparisons for forbidden network-state reuse;
- pre- and post-establishment target substitution, modification, injection,
  replay, redirect, downgrade, truncation, and forbidden-reordering attempts.

The candidate fails if protected information appears in a forbidden view or an
active violation becomes a successful connection or accepted Application Data.
Correlation by a Broad Traffic Observer or by a colluding set outside the
one-Node claim does not fail this qualification, but the excluded capability and
remaining exposure must be visible anywhere the claim is presented. Passing
qualifies only the tested candidate and conditions, not later builds or a whole
route family by implication.

The qualification verdict is conjunctive across every mandatory platform,
endpoint side, Application Data direction, and scenario cell. Results are not
pooled across cells, and all applicable performance and security requirements
must pass together. One valid forbidden disclosure, accepted active violation,
cross-context reuse, authentication failure, integrity failure, silent
downgrade, or Route bypass hard-fails the candidate regardless of aggregate
performance. Failures and timeouts remain evidence. A run may be replaced only
for a confirmed harness or declared reference-environment failure independent
of candidate behavior, with the original artifacts and invalidation reason
retained. A failed build may remain research but cannot carry the qualified V1
Interactive Route claim.

The controlled endpoint topology covers all four Windows/Linux
User-to-publisher operating-system pairings and measures both Application Data
directions separately. User, Publisher, and every ordinary Node role execute on
separate physical machines or isolated VMs through recorded controlled links;
loopback, shared-memory transfer, in-process Nodes, and hidden same-host fast
paths are invalid qualification evidence. The candidate retains its production
Route shape and all cryptographic, authentication, isolation, resource, and
fail-closed behavior. Direct baselines bracket applicable Ardents batches on the
same endpoints and end-to-end impairment profile but can never become production
fallback. Uncontrolled Internet runs are supplementary only.

Qualification sampling cannot hide tail failures. Normal short-event cells use
`100` attempts and at least `99%` success unless a specific accepted scenario
sets another floor; recovery uses at least `20` episodes and `95%` success where
expected unless a stricter rule applies. Five independent runs cover each
10-minute workload. Percentiles use nearest rank without interpolation, failed
latency orders as positive infinity, failed goodput is zero, and every eligible
sample remains evidence. A smaller smoke suite never establishes the security
or privacy claim.

## Security invariants

- A Node identity is never a User identity or Service Target.
- Possession of the V1 Service Authority is sufficient to impersonate its
  Service Target; suspected loss or compromise requires target replacement.
- A Service Name is discovery, not Service authorization or human identity.
- Name Records and Service Descriptors never contain an ordinary public origin
  address.
- Carrier Nodes cannot reinterpret or forge Application Data accepted by an
  endpoint as belonging to the authenticated Service Connection.
- A Service Connection is live: a partial write, clean Service Connection
  close, or explicit failure never means that an Application operation was
  retained, received, or completed.
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
- Target authentication, Route Profile binding, protocol freshness, and
  integrity fail closed. Modified, injected, replayed, redirected, or downgraded
  data is never accepted as a valid connection or Application Data.
- A forbidden endpoint, Local Traffic Observer, or single-Node disclosure, or a
  silently accepted active violation, fails Interactive Route Qualification for
  that implementation candidate.
- Route Qualification applies only to the tested build, configuration,
  conditions, and adversary boundary; design terminology and a previously
  qualified release are not evidence for an untested candidate.
- Interactive Route conditions, excluded Broad Traffic Observer and collusion
  cases, and remaining endpoint and traffic-metadata exposure are user-visible
  parts of the claim, not internal test notes.
- A detected target substitution produces target authentication failure when the
  evidence supports it. Integrity loss after establishment terminates the
  connection; indistinguishable causes remain connection loss or indeterminate
  failure rather than a fabricated attack diagnosis.
- No active failure causes silent fallback to another target, namespace, Route
  Profile, direct Service path, or ordinary network, and no recovery reissues an
  Application operation. Carrier-level retransmission is allowed only to
  preserve the same reliable ordered Service Connection without duplicate byte
  presentation.
- A malicious Node can always delay, drop, block, or shape traffic. Bounded route
  recovery cannot prove that a failure was accidental. P3-D4a nevertheless
  requires the same Service Connection to resume within `p95 <= 5 s` after one
  eligible ordinary-Node or Carrier Channel failure when a qualifying alternate
  Route remains, and to terminate explicitly by `15 s` otherwise; broader
  availability is not guaranteed.
- Ordinary-churn qualification repeats that eligible recovery three times in
  one 10-minute run. Each next event strikes the current Route after the prior
  recovery canary, while failed resources remain unavailable. Three is not a
  runtime quota and cannot justify terminating an otherwise healthy connection.
- One accepted overlapping-failure workload stops the current Route and then,
  within `1 s` before recovery completes, stops a distinct resource used by the
  in-progress replacement attempt. If a further qualifying Route remains, the
  same connection recovers within `p95 <= 8 s` or terminates explicitly by
  `15 s`, both measured from the first interruption. A second failure or
  internal retry never resets the clock. Failed resources stay unavailable;
  attacker-driven churn beyond this pair retains separate explicit limits.
- Controlled impairment does not authorize a hidden reconnect or security
  shortcut. With `300 ms` base RTT, independent `5%` loss in each direction,
  and `100 ms` `p95` additional jitter but no complete interruption, the same
  connection must retain its accepted target, Route Profile, Isolation Context,
  ordering, and bounded queues for 10 minutes while meeting the P3-D4b2a
  goodput and `5 s` maximum no-progress gap. A complete interruption remains a
  recovery event rather than an impaired-live success.
- Impairment and recovery remain inside the endpoint resource boundary. Each
  complete client or publisher Ardents process tree keeps `p95 RSS <= 512 MiB`,
  mean CPU `<= 50%` of one logical core, `p95` one-second CPU `<= 100%` of one
  core, and the accepted `256 KiB` directional connection queue cap during
  every 10-minute degraded or recovery workload. Temporary Route, Carrier
  Channel, timer, task, handle, queued-copy, and cryptographic state cannot
  accumulate with completed or abandoned attempts. Process splitting, hidden
  reconnects, dropped outcomes, or weakened security cannot make a run pass.
- Degradation and recovery cannot create unbounded endpoint traffic
  amplification. The impaired-live carrier ratio is at most `2.0`; one recovery
  episode adds at most `8 MiB` of endpoint traffic over a paired no-failure run;
  and each endpoint network direction keeps `p95` one-second carrier bitrate at
  or below `min(25 Mbit/s, 80% of its declared usable link budget)`. Parallel
  and abandoned attempts, retransmission, control, padding, security, liveness,
  and background bytes count. A quiet direction or episode cannot offset a
  burst elsewhere, and required protection cannot be suppressed to pass.
- Anonymous incomplete establishment attempts cannot evict established
  publisher work. Under the accepted 10-minute `1,000` attempts/s and
  `20 Mbit/s` inbound flood on a `100 Mbit/s` link, all `256` established
  connections remain usable, the active set retains its P3-D5a useful-work
  floors, inactive canaries succeed, and publisher RSS/CPU stay within
  `1 GiB`/one core. Attempt state is finite and cleaned up across all 600,000
  attempts. The defense cannot depend on IP, a global User account, or a stable
  network-generated User identity, and cannot bypass authentication, privacy,
  isolation, queues, or fail-closed handling.
- The same flood cannot make finite available publisher capacity practically
  inaccessible to ordinary anonymous Users. With `240` established connections
  and `16` free slots, at least `95%` of `600` honest attempts authenticate the
  exact target and pass a canary, connection latency has `p95 <= 8 s`, and every
  attempt returns explicitly by `15 s`, while all P3-D5a floors remain active.
  Any mandatory client admission check adds at most one logical-core CPU-second,
  `64 MiB` peak memory, and `1 MiB` traffic and cannot require money, an account,
  IP or source reputation, a stable identifier, or linking across Services or
  Isolation Contexts. Full capacity may produce an explicit bounded capacity
  result, never eviction, false success, or a hang.
- A hostile client that completes anonymous admission is not assumed to remain
  distinguishable from an honest client. With all `256` publisher slots split
  between `128` honest and `128` valid admitted hostile connections, unread
  hostile input and non-reading hostile receivers reach hard queue and
  backpressure boundaries without breaking the honest useful-work and canary
  floors or the `1 GiB`/one-core publisher limits. Harness labels, IP, accounts,
  stable identities, privileged state, and cross-context linkage are forbidden
  classifiers. While capacity remains full, an explicit capacity-unavailable
  result by `15 s` is acceptable; eviction, false success, hang, or unbounded
  admission queue is not. Ardents does not claim per-person fairness, creation
  of a free slot, or Sybil-resistant new admission under this condition.
- Integrity mechanisms reject protocol-level tagging that changes authenticated
  data, but cannot promise to detect every timing-, delay-, or volume-based tag.
  Such correlation remains within the P2-D1 and P2-D4 limitations.
- Endpoint Location Privacy is not Application anonymity. The intended Service
  reads its Application Data and observes connection behavior; the User reads
  the Service response and knows the selected Service Name or Service Target.
- Ardents exposes no User origin, Route, Isolation Context, or network-generated
  stable User identifier to a Service, and no Service Instance origin, Route, or
  Service Authority to a User.
- Isolation Context separation prevents forbidden network-state reuse but cannot
  unlink credentials, content, fingerprints, timing, volume, or behavior visible
  to an Application.
- Compromise of a User endpoint or Service host defeats protections for secrets,
  Application Data, and local network information available on that endpoint;
  the carrier cannot repair a compromised Application or Device.
- Transport Camouflage is best-effort. Ardents avoids one mandatory stable
  fingerprint but never claims invisibility or guaranteed indistinguishability
  from ordinary Internet traffic.
- A Service Connection is a logical Application-facing stream above replaceable
  transport-specific Carrier Channels. Loss or replacement of a Carrier Channel
  does not itself authorize a new Service Connection, target, Isolation Context,
  Route Profile, or Application operation.
- Continuity state may bind old and new Carrier Channels at the endpoints only
  as required to preserve one connection. It cannot become a stable network
  identity, cross Isolation Contexts, expose the full Route to an ordinary Node,
  or bypass target authentication and fresh Route validation.
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
- Stronger endpoint hardware may raise finite local capacity above the accepted
  performance floors, but extra capacity grants no Node role, trust, authority,
  route-selection priority, cross-context access, or security exception.
- Automatic scale-up requires a qualified profile that increases open and active
  connections with aggregate useful load while retaining at least `20%` of
  every declared CPU, memory, and usable-link parent budget. The first profile
  that misses any accepted gate or reserve is saturation and cannot be selected
  automatically for that tested envelope. An owner may cap lower; a finite
  higher experimental override is unqualified and cannot relax resource,
  isolation, backpressure, authentication, or Route Knowledge Separation.
- Exact endpoint hardware and configured local limits are not required network
  metadata. Traffic and admission behavior may still permit rough capacity
  inference, which is not presented as hidden by the protocol.
- Slow consumers cause bounded stream backpressure, not unbounded queues or
  silent Application Data loss. Required client and publisher profiles cap
  locally queued logical Application Data at `256 KiB` per connection and
  direction, with `16 MiB` client and `64 MiB` publisher aggregate caps in each
  direction. Attributable local OS/IPC buffering is inside that accounting;
  child scopes cannot multiply it, process-resident copies remain subject to
  the whole-process-tree RSS ceiling, and non-process buffers require separate
  bounded OS-resource evidence. A full queue accepts no further bytes, does not
  evict the connection, and cannot spill without bound to memory or disk;
  overload and fairness outcomes remain explicit.
- Under controlled equal-priority active load, aggregate success cannot hide
  starvation: every eligible connection must meet the accepted delivered-data
  floor and maximum no-progress gap. Unequal policy, degraded paths, and hostile
  load remain separate explicit contracts rather than silent exceptions.
- Full-capacity tests keep non-active authenticated connections usable while the
  active subset meets its budgets; local handles backed by silently evicted
  network state do not count as open connections.
- Security mechanisms and performance optimizations are measured together;
  neither may bypass target authentication, isolation, least privilege, or
  resource bounds. Active endpoint carrier overhead counts required security and
  liveness bytes rather than suppressing them to meet the accepted ratio.
- Bounded route and Carrier Channel retry does not create unbounded queues or
  amplification. Carrier retransmission may preserve the same ordered stream,
  but cannot duplicate bytes at the Application Interface or reissue an
  Application operation.
- Bootstrap, naming, protocol releases, software distribution, and emergency
  policy are separate Control Plane roots.
- Payload protection is not metadata protection, and independent Node IDs are
  not proof of independent control.

## Open security research

The prioritized questions live in [the network research queue](../research/questions.md).
R-006 fixes the V1 target lifecycle, R-002 fixes the Application Interface, and
R-001 P2-D1 through P2-D7 close the Interactive Route observer, Node, collusion,
endpoint, active-attack, and Route Qualification contract. No implementation
has passed that gate.
No production architecture should be selected before R-003, R-004, R-007,
R-009, and R-023 make the naming, routing, failure, bootstrap, and performance
contracts testable against the closed R-001 claim.
