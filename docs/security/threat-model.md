# Threat model

Status: **accepted claim registry and C0 audit input; no implementation qualified**

Last reviewed: 2026-08-31

## Scope

This threat model covers the current [C0 product scope](../product/scope.md) and
the stronger public-product claims retained for later decisions. It is not a
statement that the C0 implementation passed audit or qualification.

For C0, the Network candidate includes headless commands and maintained
Network, Endpoint, Service, naming, enrollment, Release, and Custody Modules.
The separate Application/Browser candidate is reviewed for seam, privilege,
artifact, and dependency correctness but receives no browser-isolation,
DNS/DoH-protection, or Application Location Privacy claim. Non-executable
compatibility evidence and historical campaign implementations are outside the
candidate corpus.

Installation, capability readiness, Time Confidence, diagnostics, update, drain,
and public-launch concentration are covered by the accepted
[product operating model](../product/operating-model.md).

It does not assume offline delivery, replicated application content, a bundled
application runtime, a User identity system, or a second high-latency route.
Those receive separate threat models only if their product contracts are later
accepted.

## Protected assets

- confidentiality, integrity, and Service Target authenticity of Application
  Data on a Service Connection;
- Forward Secrecy of honestly completed Service Connections against later
  compromise of Service/Node long-term keys and recorded ciphertext;
- ordinary network location of the User and Service Instance;
- unlinkability between distinct Isolation Contexts to the extent promised by a
  Route Profile;
- Service Authority and Service Instance Key secrecy, public bounded Instance
  Credential integrity/generation, and Service Target continuity;
- Name Authority secrecy, Service Name binding, resolution integrity, and
  recovery state;
- canonical Namespace consistency across honest compatible clients;
- Name Lease allocation, renewal, expiry, and subordinate-delegation integrity;
- Recovery Policy integrity, Recovery Authority independence, and visible
  Recovery Pending state;
- privacy of the User-location-to-name query association and Isolation Context
  separation during resolution;
- Service Name presentation integrity and resistance to namespace confusion;
- Namespace-rule integrity, naming accessibility, and explicit incompatible-fork
  state without exposing production query logs;
- endpoint-local grants, Application Interface authority, and network metadata;
- route, discovery, bootstrap, and Bridge availability;
- separation between a Direct-Origin Source's requester-origin view and every
  overlapping or derived Route/destination-aware view until the authenticated
  exposure lease and dependent work terminate;
- containment of both endpoint Applications when an Application-level location-
  privacy claim is presented;
- honest-workload latency, throughput, fairness, and endpoint resource
  availability under load;
- Control Plane integrity, software provenance, and real operator diversity;
- release/update freshness, rollback resistance, Authority Vault preservation,
  and diagnostic-data minimization.

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
  substitution, resource exhaustion, cross-context linkage, or opposite-endpoint
  disclosure through DNS, callback/SSRF, WebSocket/WebRTC, QUIC, or direct egress;
- a malicious or censor-controlled bootstrap, Candidate Materialization,
  authenticated-time, or update source that observes requester origin, biases
  retries, withholds state, or later attempts a Route/Resolution role;
- a fully compromised local endpoint or Service host;
- infrastructure seizure, legal coercion, operator disappearance, and network
  partition;
- dependency, build, signing, or update-channel compromise;
- local clock rollback, malicious time sources, stale-state resurrection, and
  update freeze or rollback;
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
and a Service Name, Service Target, or opposite endpoint. P5-D3 makes this
concrete as five symmetric logical carrier positions: User Entry, User Interior,
Rendezvous, Service Interior, and Service Entry. The route family is selected
below; concrete components, cryptography, wire protocol, and implementation
parameters remain open and unqualified.

R-004 P5-D6 through P5-D9 close four cross-cutting boundaries: Initiator,
Rendezvous, Responder, and Introduction identities occupy disjoint stable Role
Domains; endpoints select locally from authenticated Candidate Materializations;
observable failure never tests a proposal against a hidden Entry Set; and
same-connection recovery proves endpoint-only continuity using fresh route state.
The selected routing family is Tor-shaped split circuits, while every concrete
component and build remains unqualified.

P2-D4 limits that knowledge-separation claim to one malicious ordinary Node's
role-local view when the adversary controls/observes no endpoint or second
position/probe source. A Node plus a controlled endpoint can actively confirm a
known Target through timing/volume, and correlated Nodes may combine views; both
are explicit non-claims. End-to-end payload confidentiality,
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
The claim covers only traffic submitted to Ardents. A user-visible
Application-level claim additionally requires qualified Network-Isolated
Application Boundaries at both endpoint Applications; a generic adapter with
ordinary-network ingress or egress has only the narrower carrier claim.

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

### Direct-source separation claim

1. **Information:** an authenticated Direct-Origin Source that sees an
   Endpoint's origin and public artifact request must not also receive that
   Endpoint's Route position or exact Name/Target resolution view during the
   overlapping exposure.
2. **Adversary:** one malicious source/ordinary Node identity or known family
   that withholds, retries, changes role, or collides with retained state.
3. **Conditions:** globally advertised source-only duty is incompatible with
   Route/Resolution assignment; before direct contact the source is absent from
   retained Entry/Interior/Introduction/prepared-role and live work; afterward
   one installation-wide bounded exposure set excludes it until every derived
   state/work terminal bound; sequences and replacement are precommitted and
   finite.
4. **Measurement:** qualification restarts with retained endpoint-adjacent
   state, forces pre- and post-authentication family collisions, attempts later
   Route/Resolution selection, withholds each source in order, exhausts the set,
   and inspects every role and local selection result.
5. **Limitation:** the source still sees origin, public artifact, timing, and
   probable Ardents use; an unauthenticated external/CDN source has no provable
   family, hidden common control remains possible, and the source can deny
   availability.

### Application network-isolation condition

1. **Information:** ordinary endpoint location must not be disclosed by direct
   network activity induced in either endpoint Application by its opposite peer.
2. **Adversary:** a scanner connecting to an ordinary Publisher listener, or a
   malicious Service response/content or User request that triggers DNS,
   external fetch, callback/webhook/SSRF, WebSocket/WebRTC, QUIC, or an arbitrary
   socket.
3. **Conditions:** the complete client and published Application/helper process
   trees run in Network-Isolated Application Boundaries with only scoped local
   IPC/loopback, no ordinary ingress/listeners, deny-by-default egress,
   per-Isolation-Context origin/storage, and no clearnet fallback. A generic
   adapter is explicitly outside this claim.
4. **Measurement:** every supported endpoint pairing scans/connects to both
   process trees and attempts every egress class from both sides while controlled
   interfaces are observed; any listener, packet, DNS query, fallback, or cross-
   context storage reuse fails.
5. **Limitation:** this does not sanitize content, hide Application credentials,
   fingerprints, behavior, timing, or plaintext from the intended peer, and an
   OS/endpoint compromise defeats containment.

## Threat and response matrix

| Adversary | Representative attack | Required product response | Honest limitation |
|---|---|---|---|
| Censor / DPI | Block known Nodes, bootstrap sources, or protocol fingerprints; probe suspected Bridges | Multiple authenticated bootstrap sources, replaceable Bridges, transport agility, bounded rotation, and explicit blocked state | No fixed protocol disguise or address remains unblockable forever |
| Local Traffic Observer | Observe the adjacent endpoint's location, external peer addresses, timing, direction, duration, volume, retries, and long-lived patterns; attempt to classify Ardents use | Encrypt protocol and Application Data; hide the selected Service Name or Service Target, opposite endpoint location, and full Route; prohibit direct Service fallback; avoid one mandatory stable fingerprint | Ardents use may still be classified or inferred, and low-latency traffic may be correlated with observations elsewhere |
| Malicious Direct-Origin Source | Observe requester origin/public artifact, force retries or retained-state collision, then seek a Route or Destination Resolution view | Globally separate source-only assignment; pre-contact retained/live-role exclusion; bounded installation-wide exposure set; finite endpoint-precommitted source/candidate sequences; effective post-exclusion capacity gate; explicit unavailability | First/external contact reveals origin/artifact/timing; unknown-source family and hidden common control cannot be proven; source can deny availability |
| Broad Traffic Observer | Correlate both endpoint traffic statistically | Make the lack of an Interactive Route correlation-resistance claim visible; measure any later stronger Route Profile separately | Interactive traffic is expected to remain timing- and volume-correlation-sensitive |
| Malicious infrastructure Node | Combine endpoint location, Service Name or Service Target, Route, or payload knowledge; tag, modify, inject, delay, replay, drop, redirect, downgrade, bias selection, or retain metadata | Multi-hop Route Knowledge Separation; authenticated fresh protocol state; end-to-end target authentication and payload integrity; fail-closed downgrade rejection; short-lived opaque route handles; bounded retry; role separation; diversity analysis | The Node can always deny, delay, or shape traffic; timing and volume tags may aid correlation without producing a distinguishable integrity violation |
| Active endpoint confirmation | Operate one endpoint-adjacent Node plus the opposite endpoint/probe source; generate a distinctive timing/volume pattern for a known Target and correlate it with the origin edge | Expose this combined adversary as outside the Interactive Route claim; characterize both directions in Qualification evidence; never imply a protocol field is required for successful inference | The low-latency no-cover V1 profile may reveal the endpoint origin statistically even though the Node role receives no Target-to-origin binding |
| Correlated Control | Combine the permitted views of nominally different Nodes, especially both endpoint-adjacent roles, and correlate timing or volume | Avoid correlated route positions using operator, network, software, and jurisdiction evidence; expose uncertainty; test concentration under R-011 | V1 makes no anonymity guarantee against every pair or larger set; hidden common control cannot always be detected |
| Sybil / flooding actor | Capture discovery or exhaust connection, rendezvous, descriptor, naming, or all anonymously admitted Service slots | Bounded queues/lifetimes, Anonymous Cost without identity or payment, diversified selection, local staged admission, cost-to-deny evidence, established-work isolation, and visible overload | No proof of personhood, fair allocation, rightful control, free slot, or immunity from a better-resourced/valid admitted attacker; cheap sustained denial blocks an open-Service availability claim |
| Malicious Service | Fingerprint requests, link credentials or behavior, return exploit content, or lie at the Application layer | Hide User origin, Route, Isolation Context, and network-generated stable User identifiers; isolate network state; authenticate the Service Target; keep content semantics outside the carrier | The Service receives intended Application Data, timing, volume, and behavior and can link what the Application reveals |
| Malicious User | Probe Service behavior, exploit the Application, exhaust its exposed operations, or attempt to discover its origin or authority | Hide Service Instance origin, Route, and Service Authority; expose only the authenticated Service Target and Application response; enforce carrier resource limits | The User already knows the supplied Service Name or Service Target and sees all Application output intended for it |
| Application network escape | Find an ordinary Publisher listener, cause client DNS/external-resource/WebRTC access, or provoke Publisher callback/webhook/SSRF/direct socket access | Require qualified Network-Isolated Application Boundaries on both endpoint Application process trees for an Application-level claim; allow only scoped local IPC/loopback, deny ordinary ingress/egress and clearnet fallback, isolate origin/storage per context | Generic adapters remain compatible but have only the carrier claim; content, fingerprints, behavior, and compromised OS remain identifying |
| Malicious same-user local Application | Attach to another app's loopback/IPC, steal or replay a bearer, exploit PID reuse/restart, inspect another app's context/Service, overrun queues, or request downgrade | Bind grants and fresh sessions to an OS-enforced or launcher-brokered Application Principal/process tree; test sibling isolation; treat indistinguishable apps as one trust domain; keep custody separate, resources finite, and route policy explicit | A generic same-user adapter may receive no malicious-sibling isolation claim; code controlling the Endpoint Owner/OS boundary still defeats local protections |
| Compromised Service host | Copy its private Instance Key and public bounded Credential, observe Users' Application Data, or also obtain a co-located Service Authority | Stop renewal and supersede the stolen key/credential with a higher bounded generation while the root remains safe; bind every connection/recovery terminal lifetime to credential validity and learned supersession deadlines; replace the Target and rebind the Service Name if Service Authority is exposed | A compromised live Service reads intended plaintext and may impersonate its Target until credential expiry or authenticated supersession is learned; copying the public Credential alone grants no power, partitions can delay supersession, and co-locating the root removes bounded containment |
| Operator loss / seizure | Remove Nodes, inspect state, or partition reachability | No plaintext at carrier Nodes, replaceable roles, bounded state, alternate paths, and explicit unavailable results | Real availability still requires independent capacity and a live Service Instance |
| Supply-chain attacker | Ship a malicious official endpoint or protocol update | Reproducible artifacts, threshold release authorization, role-separated update metadata, staged updates, rollback protection, transparent root transitions, and independent attestations | A malicious release threshold or compromised build process can still ship harmful code; reproducibility helps detection but does not make execution safe by itself |
| Governance capture | Control naming, bootstrap, compatibility, releases, or emergencies | No administrative name seizure; versioned inspectable rules and transitions; finite technical reservations; bounded multiparty power; explicit incompatibility and fork procedure | A decentralized data path does not remove Control Plane governance, and a captured majority may still create a visible incompatible network |

Name Authority and Service Authority are separate compromise boundaries. A
Publisher does not need Name Authority for ordinary operation. If Service
Authority is compromised while Name Authority remains safe, the name can later
bind a replacement Target; if both are stored inside the compromised boundary,
that recovery claim is lost. A malicious but valid Name Authority update can
redirect name-based Users, and authenticating the resulting Target does not
repair the poisoned binding. A direct Service Target destination remains pinned
and does not follow the name.

V1 has one canonical network-wide Namespace. A resolver, local configuration,
or naming provider cannot assign a different destination to the same complete
Service Name or silently fall back to another namespace, ordinary DNS, search,
or a local alias. Conflicting, stale, invalid, partitioned, or unavailable
canonical state produces an explicit resolution failure until one authenticated
binding can be established. This consistency requirement does not imply one
administrator and does not turn the Namespace into a directory.

No administrator grants or adjudicates root Service Names. The first valid claim
accepted in deterministic shared Namespace order creates a time-bounded Name
Lease for its Name Authority; concurrent state cannot create two resolver-
selected controllers. A valid parent may issue subordinate names only inside its
subtree. No project, registrar, legal claimant, trademark process, or manual
panel can delete, seize, block, transfer, or reassign a canonical Lease. It proves
current network control, not human identity, trademark rights, endorsement,
legitimate use, or permanent property.

Claim, renewal, resolution, recovery, and naming-state capacity use bounded
Anonymous Cost and local resource admission. They cannot require money, a global
account, identity document, IP or source reputation, stable identity, cross-
context linking, wallet, token balance, or governance coin. Cost raises mass-
abuse work but proves no personhood, fairness, rightful control, or protection
from a better-resourced squatter. Every candidate must test copied pending claims,
front-running, withholding, flooding, enumeration, partitioned ordering, and
renewal censorship. R-010 selects and measures a mechanism; if none meets the
accepted accessibility, privacy, performance, convergence, and decentralization
gates, Ardents revisits root names rather than adding a central allocator.

A Lease is Active, then enters finite Grace unless renewed by its current Name
Authority, and finally becomes Released. Grace preserves resolution and exclusive
renewal but exposes a warning; Released state resolves nothing and permits a new
claim. Reclaim creates a new Name Generation, so replayed records, signatures,
renewals, delegations, and cached proof from any earlier generation remain
invalid. Revisions are monotonic inside a generation. A child can end earlier
but cannot outlive its parent; parent Release disables every descendant and a new
parent generation revives none. Unproven current generation, revision, Lease, or
parent state fails explicitly. Exact clocks, durations, cache freshness, and
convergence still require hostile-network research.

Routine Name Authority rotation or transfer preserves the Name Generation and
gives future control only to its authenticated successor. Recovery exists only
through an optional Recovery Policy committed before the incident. That policy
survives ordinary transitions, while changing or disabling it is delayed and
visible under the prior policy. An accepted threshold recovery fails name
resolution closed during bounded Recovery Pending, cannot be bypassed or
cancelled by the current authority alone, and requires a fresh successor-signed
Name Record before resolution resumes. Without usable recovery material there is
no administrator who can restore the name. A captured recovery threshold can
deny service and eventually take control after the visible delay; Ardents cannot
identify a morally or legally rightful human controller.

Private Resolution protects the association between User location and the exact
Service Name against one malicious ordinary Node, not the secrecy of the name.
The endpoint-adjacent role may see User location and traffic metadata but not the
name or a publicly testable name-derived lookup value. A naming participant may
see the name or lookup identifier and count, repeat, or dictionary-test it, but
receives no User location or network-generated stable User identifier. Query
sessions and derived state cannot link Isolation Contexts. Colluding entry and
naming roles, Correlated Control, or a Broad Traffic Observer may correlate
timing, volume, retries, cache behavior, and query popularity. A blocked private
path fails closed without direct public resolver, DNS, HTTP, alternate namespace,
or less-private fallback.

Only a finite, transparent, protocol-versioned set of Protocol-reserved Names
may exist, solely for parsing, compatibility, or protocol safety. A local Node,
Application, or gateway may refuse a name under visible local policy, but cannot
change its canonical Name Record or meaning. Namespace rules, compatibility
inputs, and accepted transition evidence are publicly inspectable without query
logs; no single operator may alter canonical state. Capture, rollback, partition,
or incompatible rule change is explicit conflicting, unavailable, or forked
state, never a silently selected alternate Namespace.

A canonical V1 Service Name is a lowercase ASCII dot hierarchy with the parent
on the right. Unicode, IDNA, and Punycode cannot create canonical alternatives.
The explicit `ardents://` Service Link separates Ardents resolution from DNS;
the similar dotted shape grants no DNS trust and triggers no public-TLD lookup or
fallback. ASCII removes cross-script and normalization ambiguity but does not
prevent misleading labels, ASCII lookalikes, or social engineering. Destination
trust surfaces therefore show the complete canonical name or link.

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
- a hostile process under the same desktop user attempting sibling IPC/loopback
  attachment, bearer theft/replay, PID reuse, process/Endpoint restart, and
  another Application's Service, Isolation Context, diagnostics, or authority;
- external scan/connect attempts plus controlled DNS, external-fetch,
  callback/SSRF, WebSocket/WebRTC, QUIC, and direct-socket attempts against/from
  the complete client and published Application/helper process trees, with both
  endpoint network boundaries observed;
- clean/restart Direct-Origin Source sequences with retained Entry/Interior/
  Introduction state, pre- and post-authentication identity/family collisions,
  later forbidden role selection, withholding, finite exhaustion, and effective
  post-exclusion reserve;
- Role Domain duties near assignment `not-after`, reassignment while old duties
  drain, and emergency termination, proving no identity/family cross-domain
  eligibility overlap;
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

The controlled endpoint topology covers all four Windows 11/Ubuntu LTS
`x86-64` User-to-publisher operating-system pairings and measures both
Application Data directions separately. User, Publisher, and every ordinary
Node role execute on separate physical machines or isolated VMs through recorded
controlled links; loopback, shared-memory transfer, in-process Nodes, and hidden
same-host fast paths are invalid qualification evidence. The candidate retains
its production Route shape and all cryptographic, authentication, isolation,
resource, and fail-closed behavior. Direct baselines bracket applicable Ardents
batches on the same endpoints and end-to-end impairment profile but can never
become production fallback. Uncontrolled Internet runs are supplementary only.

Official V1 endpoint qualification uses frozen, fully patched Windows 11 and
Ubuntu LTS `x86-64` images; Ubuntu LTS is the sole Linux baseline. User and
Publisher reference endpoints each run with `4` dedicated vCPU threads,
`8 GiB RAM`, SSD-backed storage, no GPU requirement, and no CPU or RAM
overcommit. Built-in OS security remains enabled. Exact image, CPU, microcode,
hypervisor, storage, power mode, and resource-cap evidence is retained. Other
Linux distributions and architectures cannot inherit the V1 claim.

The normal performance envelope is imposed below Carrier Transports: User access
is `100/20 Mbit/s`, Publisher and ordinary Node links are symmetric
`100 Mbit/s`, base User-to-Publisher network RTT is `80 ms`, independent loss is
`0.1%` per direction, and additional per-direction jitter has `p95 <= 10 ms`.
The harness injects no complete interruption or reordering in this normal
profile.
Candidate-induced delay, loss, reordering, retry traffic, and overhead remain
observable; transport choice cannot move them outside the qualification result.
Degraded, churn, blocking, and hostile workloads remain separate threat cells.

Controlled impairment cannot be tuned to a candidate result. Each cell freezes
a versioned immutable network manifest, complete generator parameters, and an
ordered seed assignment before execution. The generator operates below Carrier
Transports; qualification does not replay a transport-shaped fixed packet trace.
Candidate packetization, retransmission, congestion, loss, delay, reordering,
and retry remain observable behavior. Direct controls receive the same
end-to-end profile and seed discipline but no internal Ardents Route segment.
Configured and observed evidence is retained. Only a confirmed candidate-
independent failure to apply or verify the manifest may invalidate evidence
under P3-D6a; candidate-induced effects remain results.

Measurement attribution cannot hide work or fabricate elapsed time. Pass/fail
durations use one host's monotonic clock; cross-host wall clocks only correlate
logs. Raw one-second CPU, RSS, and directional carrier values are retained
without smoothing, while latency, failure, recovery, security, and queue limits
use native-resolution events and exact high-water evidence. The isolated charged
boundary includes the complete Ardents process tree and every helper. Traffic is
counted at controlled ingress and egress boundaries; candidate counters are not
authoritative. Candidate-independent missing attribution may invalidate evidence
under P3-D6a, but process escape, measurement interference, hidden work, and
candidate resource use fail the candidate.

Qualification evidence is itself an integrity and privacy boundary. The public,
content-addressed Qualification Evidence Bundle binds the exact candidate and
conditions to every scheduled raw result, invalidation, deterministic per-cell
calculation, and overall verdict. It is append-only; selected successes,
rewritten or deleted failures, unexplained redaction, an unavailable bundle, or
a report that cannot be reproduced from raw inputs cannot support a passing
claim. The qualification fixture uses only synthetic activity and credentials
without authority outside the isolated test environment; real User or Developer
traffic, production secrets, and persistent private authority never enter the
bundle.

Requalification scope cannot launder security impact. Every changed candidate
has a new identity, and a partial rerun is valid only when a scope fixed before
results proves that omitted cells cannot depend on the change. Core, protocol,
routing, naming, cryptographic, isolation, admission, recovery, transport,
relevant dependency or runtime, threat-contract, measurement-semantic, unknown,
or shared changes require the complete mandatory matrix. No favorable KPI or
less-than-`10%` regression can offset an absolute security, privacy, isolation,
authentication, integrity, or evidence-integrity failure.

Payload shortcuts are also hard failures. Fresh `32-byte` connection canaries,
nonce-bearing `512-byte` HTTP requests with complete `64 KiB` incompressible
responses, and distinct pre-generated incompressible transfer streams are
validated end to end for exact bytes, order, and integrity. An early first byte
cannot survive a later invalid response. Caching, compression, deduplication,
external resources, replayed canaries, and benchmark-specific paths cannot earn
qualification. HTTP remains confined to the controlled tracer Application and
does not become a carrier trust boundary or network semantic.

Paired direct controls cannot hide a candidate failure. Applicable goodput
batches have verified `60-second` direct transfers before and after; both must be
positive and within `max/min <= 1.10`, and their arithmetic mean supplies only
the explicitly referenced baseline. A zero, corrupt, incomplete, mismatched, or
over-drift control invalidates the complete batch while retaining every result.
Only a confirmed candidate-independent harness or environment failure permits a
complete replacement. Candidate-caused drift fails the candidate, and no direct
path is available to production traffic.

Qualification state is also a security boundary. Clean and routine startup and
cold and warm request fixtures are declared and verified for every attempt. Cold
state contains no prepared data for the exact Service Name, Service Target,
Isolation Context, and Route Profile; warm state may reuse only current
authenticated target and reusable Route state for that same tuple and never an
open Service Connection or Application response cache. The fixture manifest and
verification remain evidence. A candidate-independent reset failure may
invalidate an attempt under P3-D6a, but candidate retention, reconstruction,
hidden warm-up, response caching, or cross-context reuse is a candidate failure;
where it violates isolation or authentication, it is a hard security failure.

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
- Possession of Service Authority is sufficient to replace or impersonate its
  Service Target; suspected root loss or compromise requires Target replacement.
- An online Service Instance normally holds one private, generation-scoped
  Service Instance Key and its public bounded monotonic Credential. The host
  proves possession of the key in publication and endpoint handshakes. Copying
  the Credential alone grants no power; key compromise permits impersonation
  within that Credential's scope until expiry or learned authenticated
  supersession, but neither raw root export nor permanent Target replacement.
  Co-locating Service Authority is supported only with an explicit warning and
  forfeits that containment.
- A Service Connection binds the exact Instance Key/Credential proof and has a
  terminal `not-after` no later than Credential validity and its Work Safety
  Lease. Learned authenticated supersession may make new leg/recovery work stop
  earlier. A partition can delay learning supersession, so instant revocation is
  not claimed; expiry remains the unconditional finite bound.
- An Authority Recovery Bundle contains encrypted root material plus
  authority-owned monotonic commitments and signing watermarks, never Local
  Grants or runtime Instance Keys. An isolated test restore cannot sign. A real
  restore remains `authority locked` and export-only until authenticated current
  state permits a strictly higher generation/revision; unavailable or conflicting
  state never authorizes stale signing.
- Ordinary uninstall preserves rollback/freshness watermarks. With a non-empty
  Authority Vault it either retains the Vault or blocks until an explicit
  Owner-chosen Recovery Bundle export verifies; it never invents a secret or
  destination. Erasing authority or watermarks is a separately confirmed
  destructive purge with an unrecoverability warning.
- Connection, per-Service Administration, and Authority Custody are
  non-collapsing Local Grant boundaries. Service Administration can use an
  already authorized public Credential and matching non-exportable Instance Key
  for publication/configuration but cannot create/import/export/rotate an
  Authority, issue a Credential, or export either root or Instance key.
- Name Authority is distinct from Service Authority and controls only the
  authenticated Service Name binding. It is unnecessary for ordinary
  publication or resolution; compromising it permits malicious name rebinding
  but does not replace the cryptographic authority of an explicitly supplied
  Service Target.
- A Service Name is discovery, not Service authorization or human identity.
- The same complete canonical Service Name has one network-wide Name Record;
  local aliases and external namespaces cannot silently substitute another
  meaning when resolution fails.
- A Name Lease is time-bounded. No registrar, resolver, or manual dispute choice
  can override the accepted deterministic claim order or silently create
  concurrent controllers for the same complete Service Name.
- No administrator, project, legal or trademark claimant, or manual panel can
  seize, block, transfer, or reassign a canonical Name Lease. Protocol-reserved
  Names are finite, transparent, versioned, and limited to technical safety.
- Naming operations use bounded Anonymous Cost without money, a global account,
  identity document, IP reputation, stable identity, wallet, token, or cross-
  context link. It is not proof of personhood, fairness, legitimate use, rightful
  control, or complete anti-squatting protection.
- Grace preserves only the current Name Generation and emits a warning. Released
  state resolves nothing; reclaim cannot revive earlier records, signatures,
  delegations, descendants, or cached proof.
- A subordinate Lease cannot outlive its parent. Parent Release disables every
  descendant, including across a later claim of the same parent text.
- Routine Name Authority rotation or transfer leaves one successor with future
  control and no concurrent old-key authority. Recovery exists only under a
  precommitted generation-scoped Recovery Policy.
- Recovery Pending stops name resolution and cannot be silently bypassed by
  rotation, transfer, policy removal, or a stale Name Record. Recovery completion
  requires a fresh successor-authenticated record before resolution resumes.
- Private Resolution gives no one ordinary Node both User location and the exact
  Service Name or publicly testable lookup value. It supplies no name-secrecy,
  non-enumerability, collusion-resistance, or Broad Traffic Observer claim.
- Private Reachability Resolution gives no role-local ordinary Node both endpoint
  origin and exact Service Target/descriptor lookup value, including for a
  Target Link. Destination-aware roles are restricted to the Rendezvous Domain
  and excluded by identity/known family from the same connection's Rendezvous;
  no direct descriptor fallback exists. Node-plus-endpoint active confirmation
  remains outside this claim.
- Resolution creates no network-generated stable User identifier and shares no
  linkable query session or derived state across Isolation Contexts. Failure never
  triggers a direct public or less-private naming fallback.
- A local name filter changes only visible local policy, never canonical meaning.
  Namespace rules and accepted transitions are inspectable without query logs;
  incompatible forks remain explicit, and no operator silently selects another
  Namespace for the User.
- A Service Link identifies Ardents explicitly. Parsing or resolution failure
  cannot reinterpret its name as DNS, another namespace, Unicode, IDNA, or
  Punycode; visually similar ASCII names remain distinct destinations.
- A Target Link is a separately tagged, versioned, network-bound type containing
  the machine Target but no origin or mutable reachability. It cannot be parsed
  as a Service Name, and possessing it grants discovery only, not Application
  authorization or a weaker Route.
- Every Service Connection retains its immutable Destination Binding. Name/Link
  input binds authenticated Name generation/revision→Target into the Work Safety
  Lease. Same-Target renewal/Grace may refresh it, but learned Recovery Pending,
  Release, or rebind to another Target stops new leg/recovery work and closes by
  a finite deadline. The stream never retargets. Explicit Target/Target-Link
  input remains pinned and intentionally receives no Name recovery after Service
  Authority compromise.
- Name Records and Service Descriptors never contain an ordinary public origin
  address.
- Carrier Nodes cannot reinterpret or forge Application Data accepted by an
  endpoint as belonging to the authenticated Service Connection.
- Every Service Connection uses fresh authenticated ephemeral endpoint/session
  keys bound to the Target, Instance Key/Credential proof, protocol, Route
  Profile, Isolation Context, and transcript; every carrier leg uses fresh
  independent ephemeral keys. After best-effort erasure, later compromise of
  Service Authority, Instance Key, Node long-term keys, or recorded ciphertext
  cannot decrypt an honestly completed connection.
- Forward Secrecy does not protect a live compromised endpoint, promise
  post-compromise healing inside an existing connection, or guarantee erasure
  from memory dumps, swap, hibernation, crash artifacts, or snapshots. A safe
  restart/new connection is required after compromise remediation.
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
- The exact Route Profile is authenticated into the Service Connection. Profile
  negotiation cannot silently substitute a weaker observer, path, padding,
  mixing, or Carrier Channel contract, and unsupported capability combinations
  fail explicitly.
- Route and session state, prepared paths, peer-selection state, caches, and
  recovery handles cannot cross Route Profiles or Isolation Contexts. A new
  Route Implementation inherits no Route Qualification from another profile or
  previously qualified Adapter.
- Strengthening a Route Profile may replace route shape, introduction,
  rendezvous, multipath, mixing, padding, cover traffic, or Carrier Channel
  Adapters below the Route Module Interface. It cannot weaken or reinterpret the
  Application Interface, authenticated Service Target, Service Connection byte
  stream, Connection Result, or Application-operation replay boundary.
- Every locally authorized Application receives a distinct default Isolation
  Context; missing explicit input never selects a global shared context.
- Isolation Contexts are local policy boundaries, not network-visible User or
  Service identities, and different contexts do not share linkable route or
  session state. They may share only immutable public state, bounded
  installation/domain/regime Entry exposure, and the installation-wide Direct
  Source Exposure Set; context creation cannot reset either exposure boundary.
- Endpoint Location Privacy covers only traffic submitted through Ardents. A
  claim-bearing private Application profile contains both endpoint Application/
  helper process trees in Network-Isolated Application Boundaries, exposes no
  ordinary listener, denies ordinary DNS and sockets, isolates origin/storage by context, and never uses clearnet
  fallback. Generic adapters have no Application-level claim.
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
  Knowledge Separation without prescribing a routing algorithm.
- Its baseline data path is `User -> User Entry -> User Interior -> Rendezvous ->
  Service Interior -> Service Entry -> Service`. Each position is assigned to a
  different ordinary Node for that attempt. A shorter route is an unqualified
  profile or downgrade, not an optimization of the accepted baseline.
- An endpoint-adjacent Node may know that endpoint's ordinary location, while an
  interior or Rendezvous role may know its adjacent Nodes but receives neither
  endpoint origin, Service Name, nor Service Target from its role.
- An Introduction role may hold an expiring service-specific opaque slot. Its
  operator may independently know a public Service Name or Service Target as any
  User can, but the role receives neither endpoint origin and cannot combine that
  state with a Service-adjacent entry view for the same Service or connection
  attempt. Independent Target knowledge alone is not an anonymity violation; any
  Target-to-origin link is.
- The selected Rendezvous is not part of the Introduction Path and receives no
  invitation, Introduction Node, or introduction slot from its role. Introduction
  carries only sealed, expiring, single-use setup material and never Application
  Data or retained offline messages.
- Each endpoint selects its own Entry and Interior positions. An Entry Set is a
  small, long-lived installation × adjacent Role Domain × entry-regime resource
  shared across that endpoint role's Isolation Contexts because the same Entry
  already observes the same ordinary endpoint location. V1 permits at most one
  set for each activated Initiator, Responder, or Introduction domain and
  ordinary/Bridge regime; co-resident client and Publisher roles use separate
  domain sets. Applications, Services, Targets, generations, contexts,
  destinations, and Bridge Invites cannot manufacture another set. Each ordinary
  Entry or Bridge key is eligible for one adjacent domain, and an Invite carries
  or references its epoch-bound domain proof. Contexts still separate channels,
  keys, Interior choices, Rendezvous, destinations, queries, sessions,
  continuity, and failure state. One failure cannot force an Entry to be replaced
  by a fresh untried candidate; authenticated ineligibility or bounded sustained
  unavailability is required.
- Initiator Carrier, Rendezvous, Responder Carrier, and Introduction are disjoint
  stable Role Domains. One Node identity and every honestly declared operator
  family occupy one authenticated finite assignment. New duty is allowed only
  when its maximum Entry/role/drain lifetime fits before assignment `not-after`.
  Reassignment stops new work and quarantines identity/family until every
  old-domain duty terminates; emergency may close work but never overlap domains.
  Multiple Node IDs never make one family independent. If five distinct eligible positions
  cannot be assigned without violating domains or known family constraints, the
  route fails rather than asking a Service to reveal which proposal was unsafe.
- A globally advertised Direct-Origin Source identity/known family is
  incompatible with every Route and Destination Resolution assignment. Before
  any direct contact, the source is absent from retained Entry/Interior/
  Introduction/prepared-role state and live Route/Resolution work. An ordinary
  carrier candidate contacted as a source enters one bounded installation-wide
  local exposure set and stays excluded until the exposure lease and all derived
  state/work terminate. Source/candidate sequences and set growth are finite and
  endpoint-precommitted; unexpected collision or exhaustion is explicit
  unavailability unless a bounded Owner-approved Entry replacement was already
  permitted and counted. External/CDN hidden control remains a non-claim.
- Destination-aware Name/Target/descriptor lookup and publication use a
  Destination Resolution role restricted to Rendezvous-domain identities, never
  an endpoint-adjacent domain. For one exact destination/context the endpoint
  excludes every resolution identity and known family from that connection's
  Rendezvous. The query is also hidden from the Entry over a separately isolated
  private path with bounded retries. A Target Link never authorizes direct
  descriptor lookup. Hidden family/Sybil control and Node-plus-endpoint active
  confirmation remain explicit limits.
- The Network Epoch commits one logical complete, canonically ordered Candidate
  View: root, length, publication cutoff/input-log root, and global eligible
  count/capacity/concentration summaries. A pre-cutoff valid Node Record is
  included or receives a publicly verifiable deterministic rejection/revocation
  reason. A captured threshold may still deny or fork the entire log; transparency
  makes omission evidence visible rather than preventing governance capture.
- Every compatible endpoint selects locally from deterministic Candidate
  Materializations under that common View. It verifies chosen indices, records,
  eligibility, and proofs, not global completeness. At least two full auditors,
  independent from each other, the epoch signer threshold, and the audited
  Candidate operator families, recompute the complete View, input inclusion,
  and global summaries and publish control-independence evidence. A bootstrap
  source, signer, Service, proposed Node, or infrastructure operator does not
  choose an endpoint's complete Route.
- Endpoint-chosen precommitted sampling and local verification prevent a
  distributor from supplying a valid personalized subset. Withholding retries
  the same chosen index at another source or produces unavailability; it never
  triggers silent resampling to a different candidate. Fetching is batched
  independently of a destination and no distributor receives the complete
  selected Route.
- Role Domain assignment is deterministic over precommitted identity/family
  material and public anti-grinding randomness. A Node, signer, distributor, or
  operator cannot choose a domain after seeing one connection or manually place
  a Node; Sybil identities and dishonest family declarations remain explicit
  limits rather than being called solved.
- The User selects a fresh Rendezvous for each new Service Connection. Its state
  may survive bounded Introduction retry or qualifying leg replacement only for
  that same live attempt or connection and never crosses a completed connection
  or Isolation Context. A Service may reject an ineligible proposal but does not
  choose the User's replacement.
- Introduction roles rotate as a finite overlapping set so withdrawing old
  reachability does not create a planned outage. If eligible distinct positions
  are unavailable, route construction fails rather than weakening separation.
- No ordinary Node receives the full Route or plaintext Application Data for the
  connection, and external knowledge of a public Target grants no additional
  route state, endpoint location, authority, or protocol privilege.
- Ordinary Nodes use only the role data and short-lived opaque route handles
  required for the connection. Combining incompatible role views in one Node
  must not bypass the single-Node claim.
- Different Node IDs do not prove independent control. The claim depends on
  actual non-collusion and diversity measured under R-011.
- Public beta/stable capacity is measured after the profile's maximum mandatory
  local exclusion union. Every domain/subrole retains at least three/five
  effective families and `40%`/`25%` family-share limits plus workload reserve.
  `12`/`20` are only theoretical pre-exclusion Route-family floors; actual
  supply follows `Σ_d(3+x_d)`/`Σ_d(5+x_d)` and may be higher.
- The Interactive Route one-Node claim covers only one malicious ordinary Node's
  role-local view with no endpoint, direct-origin observation, or second
  observation/probe source under the same adversary. It makes no blanket claim
  against Node-plus-endpoint/source active confirmation or two or more colluding
  Nodes, and does not imply that every combined set necessarily holds useful
  views.
- Correlated Control spanning both endpoint-adjacent roles may link the User and
  Service through traffic metadata. An endpoint cannot always detect or report
  that this correlation occurred.
- Carrier collusion does not weaken end-to-end Application Data confidentiality,
  integrity, or Service Target authentication while endpoints and session
  cryptography remain uncompromised during the connection. It can still break
  anonymity or availability; later long-term-key compromise is covered only by
  the stated Forward Secrecy conditions.
- Target authentication, Route Profile binding, protocol freshness, and
  integrity fail closed. Modified, injected, replayed, redirected, or downgraded
  data is never accepted as a valid connection or Application Data.
- A forbidden endpoint, Local Traffic Observer, or role-local single-Node
  disclosure inside the declared claim, or a silently accepted active violation,
  fails Interactive Route Qualification. A separate bidirectional
  Node-plus-endpoint active-confirmation scenario retains correlation accuracy
  and false positives as mandatory limitation evidence; its expected success is
  not misreported as a qualified anonymity result.
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
- Continuity repair uses fresh route handles, keys, and generation state bound to
  the accepted Target, profile, context, connection transcript, and delivered
  byte range. It may repair one carrier or leg, or establish a fresh Rendezvous
  through Introduction, but cannot duplicate Application bytes, replay an
  Application operation, or extend the non-resetting `15 s` recovery deadline.
- Every Application Interface operation requires an endpoint-local grant scoped
  to an OS-enforced or launcher-brokered Application Principal/process tree,
  optional Service, and allowed operations; a desktop account, PID, loopback
  port, or copyable bearer alone is insufficient. Applications that cannot be
  distinguished are one local trust domain. Connection or publication access
  never implies raw Service Authority export.
- Local Grant revocation immediately denies new work and invalidates descendant
  session capabilities. Custody/admin sessions close immediately; data
  connections close immediately unless an explicit finite drain-then-revoke was
  selected beforehand, and that drain cannot exceed its Work Safety Lease.
  Stored local policy may survive restart, but process/session bearer state does
  not; fresh OS-local principal binding is required.
- An Endpoint Owner controls only one endpoint. No Local Grant, Endpoint Owner,
  Node operator, or sponsor is a network-wide administrator or approval root.
- A qualified public V1 Contributor runs on a dedicated host/installation and
  exposes no User connection or Service publication role. Development
  co-residence is unqualified and supplies no public capacity/independence
  evidence. An Endpoint excludes every Contributor identity and declared family
  it controls from its own Route selection.
- Client+Publisher co-residence is allowed only with distinct grants and
  Initiator/Responder/Introduction Entry Sets. The host, OS, compromise, and Local
  Traffic Observer may correlate those roles. Standalone capacity floors are not
  additive; simultaneous use requires its own combined qualification profile.
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
- A fresh or restarted endpoint accepts only threshold-authenticated, expiring,
  version-compatible Network Epoch state. Authorization is independent of
  distribution: package, cache, mirror, peer, or imported file may carry the same
  authenticated bytes but cannot make different bytes authoritative.
- A dynamic Transit Grant signer is a distinct State-authenticated purpose key,
  never an Epoch authority or holder of a State private key. Its exclusive
  durable duty root fixes one finite global budget and a bounded Request-ID
  idempotency ledger; rollback, corruption, scope substitution, withdrawal, or
  exhaustion fails closed. Compromise can spend only the remaining current-duty
  budget and cannot authorize State, Route selection, Target, Namespace,
  Release, or enrollment.
- Endpoint Transit Grant acquisition persists one exact target-free request and
  one-use TLS key before exchange. Reconciliation may repeat only that Request
  ID and byte-identical tuple. Once Node presentation begins, every success or
  ambiguity burns the attempt and erases the key; it cannot replay an
  Application operation or create an implicit replacement request.
- A directly contacted bootstrap, Candidate Materialization, authenticated-time,
  or Release Safety distributor may observe requester origin, public artifact,
  timing, and probable Ardents use. For every mandatory pre-Route artifact class,
  public beta/stable requires at least three/five effective authenticated
  independent source families with no family above `40%`/`25%`; finite sequences
  and explicit exhaustion make none silently indispensable. The same source
  families may serve several classes but count once toward global source supply.
  External/CDN/file distribution without authenticated family evidence never
  counts as independence. Multiple sources reduce indispensability but do not
  turn first contact into an anonymity claim.
- Freshness does not depend on an unauthenticated wall clock. Time Confidence
  combines monotonic runtime, persisted non-decreasing security watermarks,
  authenticated epoch bounds, and optional independent authenticated time
  observations. Uncertain, conflicting, stale, rollback, forked, or incompatible
  state blocks only the affected capability and is never silently accepted.
- Every live Route, Service Connection, publication, and Contributor duty has a
  finite Work Safety Lease ending no later than all applicable epoch,
  Release Safety, protocol/build, credential, and role-specific terminal bounds.
  Authenticated refresh may extend it before expiry; stale, clock-uncertain, or
  revoked state cannot. New leg/recovery work requires current safety state, and
  terminal expiry closes work explicitly rather than preserving old trust.
- Release, epoch, namespace, qualification, and emergency authority are distinct.
  A public baseline requires multiparty custody and expiring emergency actions;
  project-only development keys describe a centralized unqualified test network.
- Updates are authenticated by role-separated, versioned, expiring metadata with
  hashes, sizes, platform bindings, rollback protection, and explicit root
  transition. Every new public executable digest needs the `3-of-5` Targets
  threshold and binds retained source/dependency inputs, SBOM, applicable
  qualification identity, and two matching build attestations from builders
  independent of each other and of the release-Targets threshold;
  snapshot/timestamp delegates cannot introduce code. Security watermarks and
  authority state are never rolled back with program files. An atomic update
  drains, switches, self-tests, and either commits or returns to the last
  compatible safe build without claiming seamless live-connection preservation.
- Protocol migration (`announced`, `overlap-supported`, `preferred`, `required`,
  `retired`) is separate from build safety (`current/superseded`, `vulnerable`,
  `revoked`). A normal required transition waits for qualified independent
  capacity and drain reserve in every Role Domain and required control/discovery
  role. The `90-day` protocol overlap can be bypassed only by an expiring
  `4-of-5` emergency for a credible exploitable flaw, compromised primitive/key,
  or demonstrated safety incompatibility, with explicit possible unavailability;
  build revocation has no overlap entitlement.
- Release checks/downloads carry no installation identifier, account, Service
  list, rollout cohort, `from-version`, delta, or exact build history. Private-
  only mode has no direct fallback; direct-allowed and offline import are explicit
  alternatives. A direct source still sees IP, platform, exact digest/release,
  timing, repetition, and download history. Once Release Safety expires or a
  build is revoked, Ardents itself cannot be the repair route; only a configured
  external privacy proxy, explicit direct disclosure, or offline import remains.
- Diagnostics are Local-Grant-scoped, finite, and local by default. A connection
  Application, one Service administrator, Endpoint Owner aggregate, and one
  Contributor role receive distinct bounded views; Authority Custody is separate.
  Export is explicit and previewable and omits Authorities, Instance Keys, raw
  Credentials, Names, Targets, Local Grants, Bridge secrets/Invites, Entry
  membership, payload, continuity state, and complete Route histories. Telemetry
  and automatic upload are absent by default.
- Payload protection is not metadata protection, and independent Node IDs are
  not proof of independent control.

## Open security research

The prioritized questions live in [the network research queue](../research/questions.md).
R-001 through R-012 and R-024 now close the product-level target, naming, route,
failure, isolation, bootstrap, control, lifecycle, update, and privacy contracts.
They select a route family and operating boundaries, not a production component
or a qualified implementation.

The remaining security work is evidence: R-013 must compare concrete protocols,
cryptography, transports, storage, language, and dependency choices; R-023 must
complete role-specific workloads and qualify the exact implementation. Hostile
bootstrap/direct-source, clock, rollback, fork, Role Domain transition, drain,
update, Application Principal/network isolation, anonymous admission,
uninstall/purge, Sybil/concentration, and recovery drills plus independent
review are public-release gates. Until those pass, the
project is an explicitly unqualified research network and cannot claim implemented
anonymity merely because its documents are internally consistent.
