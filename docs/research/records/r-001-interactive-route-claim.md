---
id: R-001
title: What does the Interactive Route protect?
status: active
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

Already fixed: the Interactive Route is the low-latency Route Profile for one
live, authenticated Service Connection; Application Data is protected end to
end from carrier Nodes; the User and Service should not receive each other's
ordinary location as part of their relationship; and security and performance
are coequal gates.

### P2-D1 — No broad-observer promise for the Interactive Route

**Product Owner decision, accepted 2026-08-07:** V1 does not promise that the
Interactive Route resists a Broad Traffic Observer able to compare timing and
volume near both endpoints or across enough network locations to correlate one
low-latency Service Connection.

The accepted outer claim remains:

- the Service does not learn the User's ordinary network location from Ardents;
- the User does not learn the Service Instance's ordinary network location from
  Ardents;
- any one ordinary Node does not learn both ordinary endpoint locations and
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
collateral blocking of ordinary traffic. R-009 must turn this into measurements
for replaceable Bridges and transports. It remains best-effort and never becomes
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
- no carrier role receives plaintext Application Data or the full Route;
- one Node performing multiple roles must not combine their views to bypass the
  same single-Node limit.

This is a product information-flow contract, not a decision to adopt Tor, onion
routing, layered-encryption details, a fixed path shape, or exactly three hops.
R-004 must compare mechanisms and the smallest route shape that enforces the
contract. R-023 must measure the resulting setup latency, throughput, tail
latency, CPU, memory, and failure cost.

Consequences:

- a direct P2P path or single relay cannot be labeled or silently substituted as
  an Interactive Route;
- Service discovery and Rendezvous may use only short-lived opaque route data
  at a carrier role; R-003 and R-004 must prevent a stable target-to-origin
  mapping from appearing there;
- different Node IDs or network addresses are not evidence of separate control;
  actual operator, network, software, and jurisdictional diversity remain R-011
  evidence and P2-D4 conditions;
- physical co-location or one operator controlling multiple roles is treated as
  collusion rather than independent multi-hop protection;
- path length is selected later from the accepted security and performance
  contracts, not copied from a reference system.

### P2-D4 — One-Node guarantee and honest collusion limit

**Product Owner decision, accepted 2026-08-07:** V1 guarantees the Interactive
Route anonymity and Route Knowledge Separation claim against any one malicious
ordinary Node. It does not promise to resist every pair or larger colluding set.

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

An anonymity failure does not become a payload-security failure. Even if every
carrier Node colludes, the Service Connection must retain end-to-end Application
Data confidentiality and integrity and authenticate the exact Service Target,
provided the endpoints, Service Authority, and accepted cryptography remain
uncompromised. Colluding Nodes may still correlate metadata, block, delay, drop,
or manipulate traffic; P2-D6 fixes the required detection and failure behavior
for active attacks.

The mitigation contract is:

- route selection avoids correlated operator, network, software supply-chain,
  and jurisdictional control using the best available evidence;
- the product exposes uncertainty and concentration rather than asserting that
  nominally different Nodes are independent;
- R-011 defines the evidence, privacy cost, and failure thresholds for this
  selection;
- R-004 compares how candidate route shapes expose strategically useful role
  combinations;
- stronger resistance using multipath, mixing, padding, or cover traffic is a
  separate R-005 decision with its own Application need and performance budget.

Consequences:

- the baseline claim is "one malicious ordinary Node," not "up to one known bad
  operator" or "any two Nodes";
- collusion may be invisible to the endpoint, so successful connection setup
  cannot certify that the route was independently controlled;
- path diversity reduces risk but cannot prove anonymity for a particular
  connection;
- confidentiality, authenticity, anonymity, and availability remain separate
  claims with separate failure conditions;
- P2-D4 does not select multipath or a stronger Route Profile for V1.

## Remaining decisions

1. **P2-D5 — Malicious endpoint:** what a malicious User or Service can learn
   from Application Data, connection behavior, and repeated use.
2. **P2-D6 — Active attacks:** required resistance to tagging, replay, delay,
   redirection, route manipulation, and target substitution.
3. **P2-D7 — Conditions and falsification:** required honest behavior,
   diversity, measurements, failure thresholds, and user-facing limitation
   text for the final claim matrix.

## Evaluation rule

Every accepted row in the final claim matrix must state:

1. the information protected;
2. the adversary and its capabilities;
3. the conditions required for the statement to hold;
4. an experiment or analysis that can falsify it;
5. what remains visible, linkable, blockable, or attackable.

The record remains active until every relevant actor has a complete row. No
routing technology can be selected merely by using the words `anonymous`,
`onion`, `mix`, or `decentralized`.

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
failure, and repeated connections. R-023 supplies the honest-workload budget;
R-011 later tests whether claimed operator and network diversity exists.

## Alternatives rejected by accepted decisions

- **P2-D1 — Broad-observer resistance as a V1 promise:** rejected because it is
  not part of the accepted low-latency contract and would be dishonest without
  a separate measurable mechanism and cost.
- **P2-D1 — No location-privacy promise:** rejected because it would remove the
  central product value of reaching a Service without revealing ordinary
  endpoint locations to the opposite endpoint or one ordinary Node.
- **P2-D2 — Guaranteed ordinary-traffic disguise:** rejected because network
  addresses, traffic behavior, fingerprints, and active probing can expose or
  strongly suggest Ardents use.
- **P2-D2 — No camouflage objective:** rejected because avoiding one fixed
  fingerprint and raising the cost of blanket blocking are necessary in the
  accepted hostile environment even though they cannot create invisibility.
- **P2-D3 — Direct P2P or one trusted proxy:** rejected because one Node can then
  combine endpoint location with destination or origin knowledge.
- **P2-D3 — Adopt Tor and a fixed three-hop path now:** rejected because the
  accepted decision is the observable knowledge boundary. R-004 and R-023 must
  compare route families and costs before selecting a mechanism or hop count.
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

## Disposition

- State: `active`.
- P2-D1 accepted: the Interactive Route has no Broad Traffic Observer
  timing-and-volume correlation-resistance claim.
- P2-D2 accepted: a Local Traffic Observer receives no direct Service
  destination, opposite endpoint location, Application Data, or full Route from
  the protocol, but may observe connection metadata and classify Ardents use.
- Transport Camouflage is best-effort: no single mandatory stable fingerprint,
  but no invisibility or guaranteed ordinary-traffic disguise.
- P2-D3 accepted: the Interactive Route is multi-hop for Route Knowledge
  Separation; no one ordinary Node learns the full Route, plaintext, or a link
  between endpoint location and a Service Name, Service Target, or opposite
  endpoint.
- Tor, onion routing, path shape, and hop count remain unselected.
- P2-D4 accepted: V1 anonymity covers one malicious ordinary Node, not arbitrary
  collusion; Correlated Control spanning enough role views may link endpoints.
- End-to-end Application Data protection and Service Target authentication
  remain required even if every carrier Node colludes.
- P2-D5, the malicious-endpoint boundary, is next.
- No routing family, protocol, library, implementation language, ADR, or code is
  selected.
