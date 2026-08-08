---
id: R-001
title: What does the Interactive Route protect?
status: decided
owner: product research
started: 2026-08-07
reviewed: 2026-08-07
---

# R-001 — Interactive Route claim

## Decision this unlocks

Define a falsifiable privacy and anonymity contract for the low-latency route
before comparing routing families, libraries, protocols, or implementation
languages. The result must say what each relevant actor can observe, what
collusion breaks the claim, how the claim is tested, and what remains exposed.

## Current contract

- [Product vision](../../product/vision.md)
- [Network functional map](../../product/functional-map.md)
- [Threat model](../../security/threat-model.md)
- [R-002: Live Application Interface](r-002-live-application-interface.md)
- [Domain language](../../../CONTEXT.md)

**Scope clarification, accepted 2026-08-08:** this record fixes the eventual
claim and its falsification contract. Carrier Lab exercises only controlled
per-role knowledge, Target/payload protection, and explicit failure to decide
whether the route candidate is viable. It earns no Route Qualification and
makes no public Endpoint Location Privacy or anonymity claim. The complete
matrix becomes mandatory only for the corresponding public promotion horizon.

Already fixed: the Interactive Route is the low-latency Route Profile for one
live, authenticated Service Connection; Application Data is protected end to
end from carrier Nodes; the User and Service should not receive each other's
ordinary location as part of their relationship; and security and performance
are coequal gates.

### P2-D1 — No broad-observer promise for the Interactive Route

**Product Owner decision, accepted 2026-08-07:** the Interactive Route does not promise that the
Interactive Route resists a Broad Traffic Observer able to compare timing and
volume near both endpoints or across enough network locations to correlate one
low-latency Service Connection.

The accepted outer claim remains:

- the Service does not learn the User's ordinary network location from Ardents;
- the User does not learn the Service Instance's ordinary network location from
  Ardents;
- any one ordinary Node, acting only from its role-local view and not also
  controlling/observing an endpoint or second network position, is not directly
  given both ordinary endpoint locations, a Target-to-origin binding, or
  plaintext Application Data under the final Interactive Route conditions;
- none of these statements is a blanket claim that a User, Service, or
  connection is globally anonymous or untraceable.

A delayed, padded, or cover-traffic-heavy Route Profile is a separate optional
product decision under R-005. It is not silently included in the Interactive
Route and is justified only by a concrete Application job, measurable observer
advantage, and acceptable performance and resource budget.

Consequences:

- routing candidates are not rejected merely because a broad observer can
  correlate low-latency timing and volume;
- the product must disclose this limitation wherever it presents the
  Interactive Route privacy claim;
- payload encryption, multi-hop routing, and Isolation Contexts cannot be
  described as protection against broad traffic correlation by themselves;
- padding or cover traffic is not a hidden V1 requirement or an unmeasured
  anonymity setting;
- R-023 may set useful low-latency performance budgets without pretending that
  those budgets also buy broad-observer resistance.

### P2-D2 — Local privacy without an invisibility promise

**Product Owner decision, accepted 2026-08-07:** the ideal is for Ardents use to
look like ordinary traffic, but the Interactive Route cannot honestly promise
invisibility or guaranteed indistinguishability to a network administrator,
ISP, or other Local Traffic Observer.

A network-only Local Traffic Observer adjacent to one endpoint is assumed to
know that endpoint's ordinary network location. It may also observe:

- destination addresses of the endpoint's external connections, including an
  ordinary entry Node or Bridge;
- connection timing, direction, duration, packet sizes or aggregate volume,
  retries, and repeated-use patterns;
- enough protocol or behavioral signals to suspect, infer, or classify Ardents
  use.

The Ardents protocol must not directly expose at that vantage point:

- the selected Service Name or Service Target;
- the opposite endpoint's ordinary network location;
- plaintext Application Data;
- the full Route or the identities of all participating Nodes.

These are protocol-disclosure limits, not an anti-inference guarantee. A Local
Traffic Observer may still correlate timing and volume with another observation,
classify Ardents behavior, block or throttle visible peers, or learn information
from an endpoint or Application compromised outside this network-only model.

Transport Camouflage remains a real censorship-resistance goal: Ardents avoids
one mandatory stable network fingerprint and aims to make confident
classification or blanket blocking require active analysis or meaningful
collateral blocking of ordinary traffic. R-009 now fixes replaceable Bridge and
transport-agility behavior; R-013 experiments must measure concrete mechanisms.
It remains best-effort and never becomes
a statement that the observer "sees only HTTPS" or cannot identify Ardents.

Consequences:

- ordinary Interactive Route privacy does not depend on successful protocol
  disguise;
- Service identifiers, origin reachability, and Application Data cannot appear
  in locally observable plaintext merely because camouflage fails;
- transport agility and Bridges can improve circumvention without silently
  strengthening the Interactive Route anonymity claim;
- the product reports blocking or degraded reachability rather than claiming an
  undetectable connection;
- active probing and exact classification resistance belong to R-009 and the
  active-attack boundary in P2-D6.

### P2-D3 — Multi-hop Route Knowledge Separation

**Product Owner decision, accepted 2026-08-07:** the Interactive Route must use
multiple separately operated Node roles to separate knowledge. A direct
User-to-Service connection or one trusted proxy cannot satisfy the profile.

The accepted per-role view is:

- a User-adjacent entry role may know the User endpoint's ordinary network
  location, its next Node, timing and volume, and a short-lived opaque route
  handle, but not the selected Service Name, Service Target, or Service Instance
  location;
- a Service-adjacent entry role may know the Service Instance's ordinary network
  location, its adjacent Node, timing and volume, and a short-lived opaque route
  handle, but not the User location or a link between that origin and the Service
  Name or Service Target;
- an interior or Rendezvous role may know its immediate previous and next Nodes,
  its required role data, timing and volume, and a short-lived opaque route
  handle, but neither endpoint's ordinary location nor the Service Name or
  Service Target;
- an Introduction role may additionally hold an expiring service-specific opaque
  slot and sealed invitation. Its operator may independently know a public
  Service Name or Service Target as any User can, but the role receives neither
  endpoint's ordinary location and cannot link that knowledge to either origin;
- no carrier role receives plaintext Application Data or the full Route;
- one Node performing multiple roles must not combine their views to bypass the
  same single-Node limit.

This began as a product information-flow contract rather than a decision to
adopt Tor, onion routing, layered-encryption details, a fixed path shape, or
exactly three hops. R-004 was tasked with comparing mechanisms and the smallest
route shape that enforces the contract; R-023 measures the resulting setup
latency, throughput, tail latency, CPU, memory, and failure cost.

R-004 P5-D3 later fixes the resulting baseline as five symmetric logical carrier
positions after the three-position candidate was shown to expose the complete
carrier sequence to its Rendezvous. This refinement still does not select Tor,
onion construction, cryptography, a library, or a wire protocol.

R-004 P5-D4 later fixes a separate Introduction Path: the selected Rendezvous
does not forward the connection invitation, and Introduction carries no
Application Data or offline-delivery semantics.

R-004 P5-D5 later fixes selector authority and layered lifetimes: each endpoint
selects its own long-lived Entry Set and medium-lived Interior Set, while the User
selects a fresh Rendezvous for each new Service Connection. R-024 and P5-D6
through P5-D9 refine the boundary: one installation-and-entry-regime Entry Set
may expose the same endpoint across contexts, but channels, keys, Interiors,
destinations, sessions, continuity, and route-derived failure state do not cross
Isolation Contexts. Disjoint Role Domains, one Candidate View, non-oracular
selection, and endpoint-only continuity close the hidden-leg contradictions.

Consequences:

- a direct P2P path or single relay cannot be labeled or silently substituted as
  an Interactive Route;
- Service discovery, Introduction, and Rendezvous may use only bounded opaque
  route data at a carrier role; R-003 and R-004 prevent a stable
  Target-to-origin mapping from appearing there;
- different Node IDs or network addresses are not evidence of separate control;
  actual operator, network, software, and jurisdictional diversity remain R-011
  evidence and P2-D4 conditions;
- physical co-location or one operator controlling multiple roles is treated as
  collusion rather than independent multi-hop protection;
- the five-position baseline is derived from the accepted disclosure and
  symmetry contracts rather than copied from a reference system; its production
  mechanism remains subject to security and performance evidence.

### P2-D4 — One-Node guarantee and honest collusion limit

**Product Owner decision, accepted 2026-08-07; adversary scope clarified
2026-08-08:** V1 guarantees protocol Route Knowledge Separation against one
malicious ordinary Node acting only from its carrier role-local view. It does not
promise to resist every pair or larger colluding set, or an endpoint-adjacent
Node that also controls/observes an endpoint, an active confirmation source, or
a direct pre-Route contact from that endpoint.

The collusion boundary is capability- and position-dependent:

- Correlated Control of both endpoint-adjacent roles may combine their timing,
  volume, and endpoint-location views to link a User and Service;
- another colluding set breaks the anonymity claim when its merged views connect
  an endpoint location to the opposite endpoint, Service Name, or Service
  Target;
- two colluding interior Nodes that still lack endpoint, Service Name, or
  Service Target information do not gain it merely by colluding, but V1 makes
  no universal promise covering every possible pair;
- different Node IDs, addresses, or advertised operators are not proof that
  roles are independently controlled.
- a malicious Service Entry can act as a User and generate a distinctive
  timing/volume probe for a known Target, and a malicious User Entry can combine
  its edge view with a controlled Service. That carrier-Node-plus-endpoint active
  confirmation can link an origin even though the carrier role receives no
  Target field. It is an explicit low-latency non-claim, not a second Route
  Knowledge Separation guarantee.

An anonymity failure does not become a payload-security failure. Even if every
carrier Node colludes, the Service Connection must retain end-to-end Application
Data confidentiality and integrity and authenticate the exact Service Target,
provided the endpoints and accepted cryptography remain uncompromised during the
connection. Fresh authenticated ephemeral endpoint/session and per-leg keys must
also provide Forward Secrecy: later compromise of Service Authority, Service
Instance Key, Node long-term keys, or recorded ciphertext does not decrypt an
honestly completed connection after best-effort key erasure. Colluding Nodes may
still correlate metadata, block, delay, drop, or manipulate traffic; P2-D6 fixes
the required detection and failure behavior for active attacks. A live endpoint
compromise sees plaintext/session keys, and no post-compromise healing is claimed
inside an already compromised connection.

The mitigation contract is:

- route selection avoids correlated operator, network, software supply-chain,
  and jurisdictional control using the best available evidence;
- a globally advertised Direct-Origin Source is never Route- or Resolution-
  eligible in the same Role Domain Assignment, while any ordinary identity or
  known family contacted directly is retained in the installation-wide Direct
  Source Exposure Set and excluded for the complete exposure/work bound;
- the product exposes uncertainty and concentration rather than asserting that
  nominally different Nodes are independent;
- R-011/R-024 define the common Candidate View, privacy boundary, concentration
  evidence, and explicit uncertainty for this selection;
- R-004 selects the Tor-shaped split-circuit family and records how its role
  combinations expose strategically useful views;
- stronger resistance using multipath, mixing, padding, or cover traffic is a
  separate R-005 decision with its own Application need and performance budget.

Consequences:

- the baseline claim is "one malicious ordinary Node's role-local protocol
  view," not "up to one known bad operator," "one Node plus a controlled
  endpoint," or "any two Nodes";
- the one-Node claim does not depend on a public Service Target remaining secret;
  independently knowing the Target does not add a protocol field to the carrier
  role, but actively probing that Target from a controlled endpoint and
  correlating role-local traffic is outside the claim and may succeed;
- collusion may be invisible to the endpoint, so successful connection setup
  cannot certify that the route was independently controlled;
- path diversity reduces risk but cannot prove anonymity for a particular
  connection;
- confidentiality, authenticity, anonymity, and availability remain separate
  claims with separate failure conditions;
- P2-D4 does not select multipath or a stronger Route Profile for V1.

### P2-D5 — Endpoint Location Privacy, not Application anonymity

**Product Owner decision, accepted 2026-08-07:** Ardents protects the ordinary
network location of a User and Service Instance from the opposite endpoint. It
does not promise to anonymize Application Data or behavior visible to the
intended User or Service.

A malicious Service may receive and use:

- all plaintext Application Data intentionally sent to it, including accounts,
  credentials, cookies, tokens, identifiers, content, and protocol metadata;
- connection timing, direction, volume, lifecycle, repeated-use patterns, and
  client or Application fingerprints visible at its endpoint;
- any real-world identity or relationship that the Application voluntarily or
  accidentally reveals;
- the ability to return malicious, deceptive, or fingerprinting Application
  Data.

Ardents does not provide that Service with the User's ordinary network location,
the Route, Node identities, the local Isolation Context, or a network-generated
stable User identifier.

A malicious User may receive and use:

- the Service Name or Service Target it supplied or authenticated;
- all Application Data and behavior the Service intentionally exposes;
- connection timing, direction, volume, lifecycle, and availability visible at
  its endpoint;
- repeated probes or Application operations allowed by local and Service policy.

Ardents does not provide that User with the Service Instance's ordinary network
location, the Route, carrier Node identities, or the Service Authority.

Isolation Contexts prevent Ardents from reusing forbidden network-visible state
across local contexts. They cannot unlink two connections when Application Data,
credentials, content, fingerprints, timing, volume, or behavior already links
them. A compromised User endpoint or Service host falls outside the
network-only protection boundary for secrets, plaintext, and local network
information present there.

Consequences:

- the correct claim is Endpoint Location Privacy, not total User or Application
  anonymity;
- Ardents does not inspect, rewrite, sanitize, or suppress Application Data to
  make it anonymous;
- browser fingerprinting resistance, cookie policy, account separation, content
  safety, and malware defense belong to the Application or a Reference
  Application. A claim-bearing Ardents Application additionally runs inside the
  qualified Network-Isolated Application Boundary: its complete process tree
  has no ordinary-network listener, DNS, direct fetch, WebRTC, QUIC, callback,
  or socket path and can reach the network only through scoped local Ardents
  IPC/loopback. A generic adapter remains supported but receives only the
  narrower carrier-traffic claim;
- the network must not add a stable User ID, expose Isolation Context, or reveal
  route diagnostics that increase endpoint linkability;
- Service abuse prevention and authorization remain Application concerns, while
  shared carrier exhaustion remains R-010;
- P2-D5 does not weaken Service Target authentication or carrier payload
  protection.

### P2-D6 — Fail-closed active-attack handling

**Product Owner decision, accepted 2026-08-07:** target authentication, Route
Profile binding, protocol freshness, control data, and Application Data
integrity fail closed. A detected active violation never becomes an accepted
Service Connection, valid Application Data, or a silent downgrade.

The mandatory rejection boundary covers:

- Service Target substitution or redirection;
- downgrade to another Route Profile, target, namespace, direct Service path,
  or ordinary network;
- modification, injection, replay, rollback, truncation presented as clean
  completion, or reordering beyond the reliable ordered stream contract;
- forged route, rendezvous, discovery, or connection-control state;
- reuse of expired or connection-scoped protocol material outside its accepted
  freshness boundary.

The observable failure contract remains the bounded P1-D5 Connection Results:

- failure to authenticate the exact selected target is target authentication
  failure when the endpoint has evidence for that class;
- an integrity violation after establishment terminates the affected Service
  Connection and is observed as connection loss;
- when manipulation, ordinary failure, censorship, and path loss cannot be
  distinguished, the result is indeterminate failure rather than an accusation;
- no violation causes silent destination fallback, a weaker Route Profile, or
  automatic replay of an Application operation;
- bounded route recovery may replace Carrier Channels and retransmit protected
  stream bytes only when it preserves the same accepted Service Connection,
  never duplicates bytes at the Application Interface, and never reissues an
  Application operation.

Availability remains an honest limitation. A malicious Node can always delay,
drop, block, throttle, or shape the traffic it carries. Ardents may select a
different route and expose degraded or unavailable state, but cannot guarantee
delivery or reliably distinguish every attack from churn, congestion, or
failure.

Tagging has two boundaries:

- a tag that changes authenticated protocol or Application Data must be rejected
  by the integrity and freshness contract;
- timing, deliberate delay, loss, packet sizing, or volume patterns may survive
  without a detectable integrity violation and aid correlation by colluding or
  broad observers. Ardents must minimize and measure this exposure but does not
  promise to detect every such tag under the P2-D1 and P2-D4 limits.

Active discovery or probing of Bridges belongs to R-009 because it tests
Transport Camouflage and entry availability rather than changing the
Interactive Route endpoint claim.

Consequences:

- fail-closed security deliberately permits denial of service rather than
  accepting unauthenticated or corrupted state;
- active failure never reveals Node identities or a Route trace to the
  Application merely to provide a more specific diagnosis;
- an endpoint reports only evidence-supported Connection Results and does not
  claim to identify an attacker it cannot observe;
- replay protection at the network layer does not create Application-level
  exactly-once delivery or semantic replay protection;
- cryptographic construction, nonce format, epochs, counters, and concrete
  protocol messages remain later design choices;
- R-007/R-024 specify bounded recovery and evidence for failure classes; R-009
  specifies active probing and blocked entry; concrete R-013/R-023 candidates
  measure tagging exposure.

### P2-D7 — Route Qualification and falsification gate

**Product Owner decision, accepted 2026-08-07:** the Interactive Route is a
test-gated product contract. A specific implementation candidate earns Route
Qualification only after reproducible controlled-network tests show that every
required observer, endpoint, and Node-role boundary holds and that active
attacks fail closed. Until then Ardents remains research and must not present
that implementation publicly as an anonymous network.

The qualification environment must:

- capture the network traffic visible at the User edge, Service edge, and every
  ordinary Node role in a controlled topology;
- inspect the live and retained protocol state available to each ordinary Node
  role, testing one malicious role at a time rather than assuming role
  separation from the design;
- run malicious-User and malicious-Service cases and inspect every network
  field, local result, diagnostic, and route artifact exposed to them;
- run both controlled endpoint Applications inside the declared Network-
  Isolated Application Boundary, then attempt external ingress scanning, DNS,
  direct fetch, WebSocket/WebRTC/QUIC/socket egress, callbacks, and SSRF from
  every helper and child process; any ordinary-network success hard-fails the
  Application-level location claim;
- run a hostile same-desktop-user sibling against each Application Principal,
  including IPC/loopback attachment, bearer theft and replay, PID reuse,
  restart, cross-Service/context access, diagnostics, and authority requests;
- exercise Role Domain reassignment through stop-new-work, drain, quarantine,
  and later eligibility, proving that no old and new duty overlaps;
- exercise every Direct-Origin Source class, restart, retained-state collision,
  unexpected post-authentication family collision, finite retry/set exhaustion,
  and attempted later Route/Resolution selection. A globally source-only family
  or any still-exposed contacted family appearing in a protected role hard-fails
  the candidate;
- run a separately labelled active-confirmation characterization in both
  directions: one endpoint-adjacent malicious Node also controls the opposite
  endpoint/probe source and emits a precommitted distinctive timing/volume
  pattern. Retain correlation accuracy and false-positive evidence. Success is an
  expected limitation of this profile, not a qualification pass; any public
  wording that includes this combined adversary fails the claim review;
- attempt target substitution, modification, injection, replay, redirection,
  downgrade, and forbidden reordering before and after connection
  establishment;
- retain enough topology, workload, build, configuration, capture, and result
  evidence to reproduce the conclusion.

The required conditions are part of the claim, not test fine print:

- the protected endpoint, its Application, the Service Authority, and accepted
  cryptography are uncompromised where the row depends on them;
- an Application-level endpoint-location claim requires the tested Network-
  Isolated Application Boundary on both controlled endpoint Application process
  trees; a generic HTTP/SOCKS/stream adapter is outside that stronger claim;
- the single-Node claim assumes no Correlated Control of additional roles, no
  endpoint, second observation/probe source, or direct-origin observation
  controlled by that Node adversary, and that the remaining required roles
  follow the tested protocol;
- the Local Traffic Observer occupies one endpoint edge and does not also own
  the endpoint or a second observation position;
- the connection uses the accepted Interactive Route without a direct path,
  weaker Route Profile, alternate namespace, target, or ordinary-network
  fallback;
- distinct Isolation Contexts do not deliberately reuse Application identity or
  Application Data that links them.

Any forbidden disclosure in the captured traffic, role state, endpoint output,
or diagnostics fails qualification. Any substituted target, modified or
replayed data, redirect, downgrade, or invalid control state accepted as a
successful connection or valid Application Data also fails qualification. A
Broad Traffic Observer correlation or correlation by a sufficiently placed
colluding set is not a failed test because those adversaries are explicitly
outside the Interactive Route anonymity claim; that exclusion must be visible
wherever the claim is shown.

Passing qualifies only the tested implementation candidate under the recorded
conditions. It is not a proof for an untested build, configuration, route
family, future release, compromised endpoint, or excluded adversary.

## Final claim matrix

| Information | Adversary | Conditions | Falsification | Honest limitation |
|---|---|---|---|---|
| User ordinary location, Route, Isolation Context, and any network-generated stable User ID | Malicious intended Service | User endpoint and its local Application are uncompromised; the tested Application process tree is Network-Isolated; exact target authentication holds; no direct or weaker-route fallback | Run the Service maliciously; inspect its Application Data, endpoint results, network-visible fields, traffic, logs, and repeated connections; independently probe every ordinary ingress/egress path of the User Application | The Service receives plaintext Application Data, timing, volume, and behavior. Credentials, content, fingerprints, behavior, or an Application outside the isolation boundary can identify or link the User; endpoint compromise defeats local protection |
| Service Instance ordinary location, Route, and Service Authority | Malicious User | Service endpoint and local Service Application are uncompromised; the complete published Application process tree is Network-Isolated; exact target authentication holds; no direct or weaker-route fallback | Run the User maliciously; inspect its Application Interface results, Service output, network-visible fields, traffic, diagnostics, and repeated connections; independently probe every ordinary ingress/egress path of the Service Application | The User knows the selected Service Name or Service Target and receives intended Service output and behavior; compromised content/host or an Application outside the isolation boundary can reveal more |
| Direct protocol-state link between either endpoint location and a Service Name, Service Target, or opposite endpoint; full Route; plaintext Application Data | Any one malicious ordinary Node, tested in every eligible role | The adversary has only that Node's role-local view, controls/observes no endpoint, direct-origin contact, or second position/probe source, and no additional Node is under Correlated Control | Make each eligible Node and allowed role combination malicious in turn and inspect all traffic, live state, retained state, handles, logs, source exposure, and failure paths available to it | The Node sees immediate peers, required role data, timing, direction, volume, and opaque handles. Node-plus-endpoint/direct-source active confirmation, a second observation, and sufficient Correlated Control are outside this claim |
| Not protected: endpoint origin against targeted timing/volume confirmation | One endpoint-adjacent malicious Node that also controls/observes the opposite endpoint or an active probe source | Not promised by the Interactive Route | In both directions, emit precommitted distinctive probes for a known Target, correlate them at the adjacent Node, and retain accuracy/false-positive evidence | A public Target is probeable and a low-latency no-cover profile may reveal the origin statistically even though no carrier field carries the Target-to-origin binding |
| Selected Service Name or Service Target, opposite endpoint location, Application Data, and full Route | Passive or active Local Traffic Observer at either endpoint edge | Observer does not control the endpoint or a second observation position; no direct fallback; accepted encryption and target authentication hold | Capture, classify, block, replay, and manipulate all edge traffic in both endpoint-edge cases; search captures and observable protocol behavior for protected values | The observer knows the adjacent endpoint location and may see peer addresses, timing, direction, duration, volume, retries, and a classifiable Ardents fingerprint |
| Application Data confidentiality, integrity, Forward Secrecy; exact Service Target, Instance Key/Credential proof, Route Profile, and fresh protocol state | One malicious Node or all carrier Nodes colluding and acting actively; later long-term-key compromise | Both endpoints and accepted cryptography remain uncompromised during the connection; ephemeral keys are erased best-effort after completion | Capture carrier state and ciphertext; attempt substitution, modification, injection, replay, redirection, downgrade, truncation, forbidden reordering, then compromise Service/Node long-term keys after close and attempt retrospective decryption | Carrier Nodes can deny, delay, drop, throttle, or shape traffic. Live endpoint compromise reads plaintext/session keys; swap, hibernation, dumps, or snapshots may defeat erasure; no in-connection post-compromise healing is promised |
| Absence of network-generated linkage across distinct Isolation Contexts | Malicious Service and carrier observations available within the other accepted rows | Contexts are distinct and do not deliberately reuse linking Application Data, credentials, or behavior | Repeat equivalent connections in separate contexts and compare all route, session, handle, identifier, cache, and endpoint-visible state for forbidden reuse | Application Data, fingerprints, timing, volume, and endpoint observation may still link the contexts |
| Not protected: relationship unlinkability from timing and volume near both endpoints | Broad Traffic Observer, or a colluding set whose combined views cross the knowledge boundary | Not promised by the Interactive Route | A successful timing-and-volume correlation is recorded as an expected limitation, not misreported as a passed anonymity test or a protocol-integrity failure | Low-latency Interactive traffic remains correlation-sensitive; a stronger profile requires a separate R-005 contract |

## Closure

R-001 has no remaining product decisions. Its matrix was the acceptance contract
against which R-004 selected the Route family and against which later concrete
implementation candidates seek Route Qualification. Closing this record decides
the claim; it does not validate an implementation.

## Evaluation rule

Every accepted row in the final claim matrix must state:

1. the information protected;
2. the adversary and its capabilities;
3. the conditions required for the statement to hold;
4. an experiment or analysis that can falsify it;
5. what remains visible, linkable, blockable, or attackable.

Every relevant actor now has a row. No routing technology can be selected merely
by using the words `anonymous`, `onion`, `mix`, or `decentralized`, and no
implementation inherits Route Qualification from its design vocabulary.

## Evidence plan

### Primary sources

Compare official threat models and protocol/design documents for Tor, I2P, and
Nym, followed by primary traffic-analysis research where those systems make
different claims. Extract adversary capability, protected information,
collusion threshold, latency/resource cost, and explicit non-guarantees rather
than copying system labels.

### Measurement

For each candidate route family, define a reproducible topology and collect the
endpoint, intermediary, and observer views of connection setup, steady traffic,
failure, and repeated connections. Execute the P2-D7 active-attack suite and
retain the evidence needed for Route Qualification. R-023 supplies the
honest-workload budget; public-launch concentration evidence later tests whether
claimed operator and network diversity actually exists.

## Alternatives rejected by accepted decisions

- **P2-D1 — Broad-observer resistance as a V1 promise:** rejected because it is
  not part of the accepted low-latency contract and would be dishonest without
  a separate measurable mechanism and cost.
- **P2-D1 — No location-privacy promise:** rejected because it would remove the
  central product value of reaching a Service without directly revealing
  ordinary endpoint locations to the opposite endpoint or a role-local ordinary
  Node. The accepted value does not include active traffic confirmation by a
  Node-plus-endpoint observer.
- **P2-D2 — Guaranteed ordinary-traffic disguise:** rejected because network
  addresses, traffic behavior, fingerprints, and active probing can expose or
  strongly suggest Ardents use.
- **P2-D2 — No camouflage objective:** rejected because avoiding one fixed
  fingerprint and raising the cost of blanket blocking are necessary in the
  accepted hostile environment even though they cannot create invisibility.
- **P2-D3 — Direct P2P or one trusted proxy:** rejected because one Node can then
  combine endpoint location with destination or origin knowledge.
- **P2-D3 — Adopt Tor and a fixed three-hop path immediately:** rejected because
  the accepted decision first had to be the observable knowledge boundary.
  R-004 subsequently selected a Tor-shaped five-position split-circuit family,
  not Tor wholesale or a fixed three-hop path, while concrete mechanisms remain
  R-013/R-023 work.
- **P2-D3 — Treat different Node IDs as independence:** rejected because one
  operator or correlated infrastructure can control nominally distinct roles.
- **P2-D4 — Resist every pair of colluding Nodes in V1:** rejected as a blanket
  low-latency claim. Strategically placed views can correlate both endpoint
  sides, and stronger resistance requires a separate measurable mechanism and
  cost.
- **P2-D4 — Ignore collusion because it cannot be proven:** rejected because
  route selection, concentration evidence, and honest limitation can still
  reduce and expose risk.
- **P2-D4 — Treat anonymity failure as payload compromise:** rejected because
  end-to-end confidentiality, integrity, and target authentication do not depend
  on carrier Nodes remaining independent.
- **P2-D5 — Total anonymity from a malicious Service:** rejected because the
  intended Service necessarily receives Application Data and can link identity
  or behavior that the Application reveals.
- **P2-D5 — Carrier inspection or sanitization for anonymity:** rejected because
  it would violate opaque Application Data, end-to-end protection, and the
  Application responsibility boundary.
- **P2-D5 — Expose endpoint or route diagnostics for troubleshooting:** rejected
  because the opposite endpoint does not need origin, Node, Route, Isolation
  Context, or authority information to use a Service Connection.
- **P2-D6 — Continue after a detected integrity or authentication violation:**
  rejected because availability cannot justify accepting unauthenticated,
  corrupted, replayed, or downgraded state.
- **P2-D6 — Label every drop or delay as an attack:** rejected because churn,
  congestion, censorship, failure, and malicious behavior can be observationally
  indistinguishable.
- **P2-D6 — Silently create a new Service Connection and replay an Application
  operation:** rejected because the network cannot infer Application idempotency
  or remote completion. Bounded Carrier Channel replacement inside the same
  reliable Service Connection is not this rejected behavior.
- **P2-D6 — Promise detection of every timing tag:** rejected because a Node can
  shape low-latency timing and volume without modifying authenticated bytes.
- **P2-D7 — Claim anonymity from design or terminology:** rejected because the
  protected views and fail-closed behavior must be demonstrated on a specific
  implementation candidate under reproducible conditions.
- **P2-D7 — Treat excluded broad observation or collusion as a hidden pass:**
  rejected because those are honest limitations that must be visible, not
  omitted from qualification results.
- **P2-D7 — Bury conditions in test documentation:** rejected because Users and
  Developers must see the conditions and limitations wherever the Interactive
  Route privacy claim is presented.

## Disposition

- State: `decided`.
- P2-D1 accepted: the Interactive Route has no Broad Traffic Observer
  timing-and-volume correlation-resistance claim.
- P2-D2 accepted: a Local Traffic Observer receives no direct Service
  destination, opposite endpoint location, Application Data, or full Route from
  the protocol, but may observe connection metadata and classify Ardents use.
- Transport Camouflage is best-effort: no single mandatory stable fingerprint,
  but no invisibility or guaranteed ordinary-traffic disguise.
- P2-D3 accepted: the Interactive Route is multi-hop for Route Knowledge
  Separation; one ordinary Node's role-local view, when that adversary controls
  or observes no endpoint, second position, or active probe source, receives no
  full Route, plaintext, or direct endpoint-origin-to-Name/Target/opposite-
  endpoint binding. Node-plus-endpoint timing/volume confirmation is an explicit
  non-claim.
- R-004 P5-D3 subsequently fixes a symmetric five-position logical data path and
  later selects the Tor-shaped split-circuit family; Tor naming, exit routing,
  concrete onion construction, cryptography, libraries, and wire protocol remain
  unselected.
- R-004 P5-D4 subsequently separates Introduction from the Rendezvous data path;
  C1 Rendezvous-forwarding remains only an unqualified performance experiment.
- R-004 P5-D5 subsequently fixes endpoint-owned selection, long-lived Entry,
  medium-lived Interior, connection-scoped Rendezvous, and overlapping
  Introduction rotation without fixing numeric durations or set sizes.
- P2-D4 accepted: V1 Route Knowledge Separation covers one malicious ordinary
  Node's role-local view, not arbitrary collusion or Node-plus-endpoint active
  confirmation. Direct-Origin Source duty is globally separated and contacted
  source families are locally exposure-excluded; an adversary that nevertheless
  combines a direct-origin observation with a carrier role is outside the claim
  and fails the accepted role/exposure architecture.
- End-to-end Application Data protection and Service Target authentication
  remain required even if every carrier Node colludes. Fresh authenticated
  ephemeral keys and best-effort erasure additionally require Forward Secrecy
  against later Service/Node long-term-key compromise; exact AKE and suite remain
  R-013 work.
- P2-D5 accepted: Ardents provides Endpoint Location Privacy against a malicious
  opposite endpoint but does not anonymize Application Data, credentials,
  fingerprints, timing, volume, or behavior visible to it.
- The Application-level claim requires a qualified Network-Isolated Application
  Boundary on both endpoint process trees. Generic adapters remain useful but
  carry only the narrower claim for bytes actually submitted to Ardents.
- A Service receives no User origin, Route, Isolation Context, or network-generated
  stable User ID; a User receives no Service Instance origin, Route, or Service
  Authority.
- P2-D6 accepted: authentication, integrity, freshness, and Route Profile binding
  fail closed; detected violations never become accepted data or silent downgrade.
- Delay, denial, traffic shaping, and indistinguishable attack remain honest
  limitations; bounded recovery never replays an Application operation.
- P2-D7 accepted: Route Qualification requires reproducible traffic, Node-state,
  endpoint, Application Principal, Application-network isolation, Direct-Origin
  Source, Role Domain transition, and active-attack tests; a forbidden disclosure or
  silently accepted substitution, modification, replay, redirect, or downgrade
  fails the candidate.
- Broad Traffic Observer and sufficiently placed collusion correlation are
  explicit non-claims, not false test failures; their limits remain visible.
- R-001 is closed as a product contract. No implementation has yet earned Route
  Qualification, so Ardents cannot yet present itself publicly as an anonymous
  network implementation.
- R-004 selects the Route family and ADR-0005 records its domain/exposure
  boundary. No concrete protocol, library, cryptography, implementation language,
  production code, or qualified implementation is selected.
