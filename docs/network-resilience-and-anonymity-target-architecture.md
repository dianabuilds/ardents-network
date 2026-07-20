# Network Resilience And Anonymity Target Architecture

Status: **proposed target architecture**

Date: 2026-07-19

Primary owner: `Network Foundation / Messaging`

Contributing owners: `Identity`, `Discovery`, `Policy`, `Diagnostics`,
`Data Substrate` and `Node Runtime`.

## 0. Краткая Схема Решения

Документ написан на техническом английском, принятом в сетевой части
репозитория. Этот раздел фиксирует русскоязычную карту решения.

Главный принцип:

```text
надёжность соединения != маскировка транспорта != анонимность маршрута
```

Это три разные задачи:

- **transport** отвечает, как Waku-соединение проходит через доступную сеть:
  TCP, WSS/HTTPS, позднее QUIC;
- **reliability** отвечает, как сообщение получает подтверждение, повтор,
  дедупликацию и терминальный результат;
- **anonymity** отвечает, кто может связать источник с конечным Waku selector.

Целевой пакет проходит такие слои:

```text
данные продукта
  -> ardents-private/1
       шифрует содержание и скрывает продуктовый смысл
  -> ardents-route/1, если выбран onion/mix
       скрывает маршрут и разрывает связь источник <-> selector
  -> Waku Relay / Store / Filter / Lightpush
       остаётся реальным сетевым carrier
  -> TCP / WSS / QUIC
       обеспечивает прохождение через конкретную среду
```

Режимы приватности:

- `direct`: текущая сильная E2E-защита содержания без сетевой анонимности;
- `onion`: три хопа — guard, middle, egress; низкая задержка, но timing
  correlation остаётся возможной;
- `mix_balanced`: ingress, три mix-слоя и egress, фиксированные пакеты,
  задержки и cover traffic;
- `mix_strong`: та же схема с более дорогими общими параметрами задержки и
  фонового трафика.

Хоп — это один Ardents privacy relay, снимающий только свой слой Sphinx.
Обычная пересылка GossipSub внутри Waku не считается дополнительным privacy
hop.

HTTPS/WSS здесь решает доступность и устойчивость к блокировкам. Onion routing
разделяет знание между узлами. Mixnet дополнительно мешает сопоставлять пакеты
по времени и объёму. Ни один из этих механизмов не заменяет остальные.

Безопасное поведение по умолчанию:

```text
запрошенная гарантия недоступна
  -> очередь в пределах deadline
  -> явный degraded/unavailable
  -> никакого скрытого перехода в direct
```

То есть документ строит не «абсолютную анонимность», а проверяемый инструмент,
где каждая гарантия имеет условия готовности, остаточный риск и release gate.

## 1. Document Role

This document defines a post-stabilization target for a reliable Ardents
network that must operate in censored, unstable, actively probed, and
metadata-hostile environments.

It combines:

- an architecture decision;
- a threat model;
- a technical protocol direction;
- a shared vocabulary;
- an implementation and qualification plan.

It does not claim that the current runtime already provides onion routing,
mixing, traffic-flow anonymity, or censorship-resistant HTTPS camouflage.
The currently implemented privacy contract remains `ardents-private/1` as
defined in `docs/network-privacy-protocol.md`.

This document becomes normative for implementation only after it is accepted
and the affected requirements, domain documents, QA scenarios, and process
plan are updated. Until then it records the intended target without converting
future behavior into false runtime truth.

## 2. Decision Summary

The target architecture makes the following decisions:

1. `Waku` remains the canonical network foundation and the actual carrier for
   Ardents `v1` evolution.
2. `ardents-private/1` remains the end-to-end semantic confidentiality
   envelope. Onion or mix forwarding wraps it; it does not replace or weaken
   it.
3. Transport selection, delivery reliability, and anonymity are independent
   state axes. A transport change must not silently change the anonymity
   guarantee.
4. Low-latency onion forwarding and delayed mix forwarding use one mature,
   audited Sphinx-compatible packet engine. Ardents must not invent an onion
   cipher, mix packet format, or cryptographic primitive.
5. Every privacy hop remains a Waku participant. Hop-to-hop packets are Waku
   messages; no parallel generic transport substrate is introduced.
6. The default anonymous path has three route relays. The strong mix path has
   an ingress gateway, three stratified mix relays, and an egress service.
7. Strong modes use fixed-size cells, common epoch parameters, replay
   protection, route diversity, and single-use reply blocks. A mode without
   its mandatory protections must not report itself as ready.
8. HTTPS/WSS is the primary censorship-tolerant transport profile. Production
   endpoints use publicly trusted, automatically rotated certificates.
   Self-signed certificates are test-only and are not a camouflage strategy.
9. No privacy profile silently downgrades to `direct`. Policy may allow an
   explicit downgrade for a particular operation, but the safe default is to
   fail closed and explain the loss of capability.
10. Reliability is end-to-end. Waku publish or Lightpush acknowledgement is
    carrier acceptance, not recipient delivery.

### 2.1 Architectural Fit

- `Network Foundation / Messaging` owns route construction, route forwarding,
  Waku provider proxying, route replay, and delivery truth.
- `Discovery` owns the common route-directory knowledge and trust-aware relay
  eligibility.
- `Identity` owns relay identity, route capabilities, and protected key
  resolution.
- `Policy` owns profile admission, downgrade permission, resource rules, and
  relay eligibility decisions.
- `Diagnostics` owns redacted explanation, not route or packet truth.
- `Data Substrate` owns object availability, chunking semantics, and durable
  data effects; it does not become a mix relay implementation.
- no new top-level `Anonymity` domain or generic transport plugin system is
  introduced;
- the existing `ardents-private/1`, Waku participation, transport controller,
  and WSS profile are extended under their current owners rather than replaced;
- legacy package layering is not reused as target architecture.

## 3. Why The Current Boundary Is Insufficient

`ardents-private/1` already provides a strong semantic confidentiality
boundary:

- product payload enters Waku encrypted;
- product meaning is absent from readable content topics;
- channel selectors are capability-derived;
- retained ciphertext does not give relay operators decryption authority;
- replay, expiry, size, signature, grant, and revocation checks fail closed.

Its explicit non-claims remain important:

- participation in Waku or Ardents can be observed;
- a directly connected peer can observe its counterpart;
- timing, volume, topology, carrier topic, opaque-selector equality, and
  padded-size bucket remain observable;
- Store, Filter, and Lightpush providers can associate a requesting peer with
  an opaque content topic;
- a stable PeerID can link activity across sessions.

An opaque topic solves semantic disclosure but does not solve correlation. A
stable random-looking identifier is still a stable handle.

Waku also has availability boundaries that Ardents must not misrepresent:

- Store is bounded retrieval support, not durable data availability;
- Lightpush acknowledgement proves receipt by one provider, not network-wide
  propagation or recipient delivery;
- GossipSub delivery can duplicate, reorder, delay, or lose messages;
- discovery and bootstrap mechanisms can be censored, poisoned, overloaded, or
  narrowed into an attacker-controlled local view.

Therefore the target must add metadata privacy and end-to-end delivery
semantics above Waku without treating Waku itself as an anonymity network.

## 4. Goals

The target solution must:

- remain operational when UDP is blocked, individual peers fail, DNS is
  censored, routes flap, or selected providers are malicious;
- hide payload and product semantics from non-capability holders;
- prevent one ordinary privacy relay from learning both source and final Waku
  selector;
- offer a low-latency onion profile and a stronger traffic-analysis-resistant
  mix profile;
- keep mobile/light clients first-class through Relay, Store, Filter, and
  Lightpush proxying;
- make delivery state, degraded guarantees, switching, and recovery
  operator-visible without leaking routes, selectors, or secrets;
- use mature dependencies for transport, cryptography, packet formats, and
  observability;
- bound CPU, memory, disk, bandwidth, queueing, retries, cover traffic, and
  amplification;
- fail closed when the requested guarantee cannot be maintained;
- provide evidence for every release claim through packet capture,
  integration, E2E, mutation, chaos, performance, and long-running tests.

## 5. Non-Goals And Honest Non-Claims

The target does not promise:

- absolute anonymity;
- protection after compromise of the sender or recipient endpoint;
- protection from an authorized participant reading data its capability
  permits;
- guaranteed unlinkability if all route relays on a path collude;
- unlimited resistance to a global active adversary;
- useful mix anonymity with too few independent users, relays, or cover cells;
- that TLS, WSS, a certificate, or an ordinary-looking website alone makes
  Ardents traffic indistinguishable from all web traffic;
- durable blob availability from Waku Store;
- exactly-once network delivery.

The product may claim only properties that the active profile, current
directory, observed topology, cover budget, and retained test evidence support.

## 6. Canonical Vocabulary

### 6.1 Foundation And Transport

**Network foundation**:
The system that makes Ardents a real network. For `v1` this is Waku.
Avoid using `backend`, `generic transport core`, or `plugin bus` for this role.

**Carrier**:
The Waku protocol path that transports an Ardents packet: Relay, Store,
Filter, or Lightpush.

**Underlying transport**:
The concrete libp2p connection family below Waku, such as TCP, secure
WebSocket, or QUIC.

**Transport profile**:
An allowed set of underlying transports and endpoint exposure rules, for
example `tcp_only`, `tcp_wss`, or a future `tcp_quic`.

**Transport camouflage**:
A censorship-resistance technique that makes an allowed transport resemble a
common protocol such as HTTPS. It is not payload encryption and not anonymity.
Avoid the unqualified term `masking`.

**Bridge**:
An ingress Waku peer whose address is distributed out of band or through a
restricted directory and whose transport is intended to survive blocking or
active probing.

### 6.2 Route And Anonymity

**Privacy relay**:
A Waku-backed node admitted to forward `ardents-route/1` packets.

**Hop**:
One privacy relay processing step on a route. GossipSub forwarding inside Waku
does not add an Ardents privacy hop.

**Route**:
The ordered privacy relays selected for one packet or delivery attempt.

**Circuit**:
A short-lived reusable route context with fixed entry policy and bounded
lifetime/cell count. It is not a TCP connection.

**Entry guard**:
A deliberately sticky first privacy relay. It knows the source connection but
must not learn the final selector.

**Middle relay**:
A privacy relay that learns only its immediate route predecessor and successor
or their bucket-level equivalents.

**Egress service**:
The final privacy relay that performs a Waku Publish, Store, Filter, or
Lightpush operation. It can learn the final opaque selector and operation
category but must not learn the source endpoint.

**Onion routing**:
Layered route encryption in which each hop removes only its own layer and
learns only the next hop. Low latency makes it vulnerable to end-to-end timing
correlation.

**Mixnet**:
An onion-routed packet network that additionally uses common packet geometry,
delay/reordering, and cover traffic to reduce timing and volume correlation.
Onion encryption without mixing is not a mixnet.

**Mix layer**:
One stratum in which a packet selects one eligible mix relay before continuing
to the next stratum.

**Sphinx packet**:
A fixed-geometry nested packet whose per-hop transformation hides the
remaining route and supports unlinkable forwarding and single-use reply
blocks. Ardents consumes a mature implementation; it does not implement
Sphinx cryptography itself.

**Cell**:
One fixed-size `ardents-route/1` packet. Large logical messages are fragmented
into cells before route construction.

**SURB**:
A Single-Use Reply Block that lets another party send one response without
learning the recipient's route or location. Reuse is forbidden.

**Cover cell**:
A valid cell that is externally indistinguishable from a data cell but carries
no product operation.

**Anonymity set**:
The set of participants or packets from which an observer cannot distinguish
the actual source or destination under the stated threat model.

**Unlinkability**:
The inability of an observer to reliably associate two relevant facts, such as
source IP and final content topic.

### 6.3 Product State Axes

**Privacy profile**:
The active metadata-protection contract: `direct`, `onion`,
`mix_balanced`, or `mix_strong`.

**Operational mode**:
The active defensive/recovery posture, such as `steady`,
`restricted_defense`, or `recovery`.

**Delivery state**:
End-to-end progress of one logical delivery. It is independent of transport
and privacy profile.

These axes must never be collapsed into one field called `mode`.

The current documents use `standard`, `low_exposure`, `defense`, and
`recovery` for adaptive privacy/participation posture. If this target is
accepted, those values must be migrated into the operational-posture axis;
they must not be treated as equivalents of `direct`, `onion`, or `mix`.
That migration requires coordinated updates to
`network-privacy-architecture.md`,
`network-privacy-requirements.md`,
`network-transport-architecture.md`, diagnostics contracts, and tests before
code adopts the new vocabulary.

## 7. Threat Model

### 7.1 Sensitive Assets

- application plaintext;
- product message class and operation meaning;
- channel and capability material;
- the relation `source endpoint -> final selector`;
- the relation `sender -> recipient`;
- timing and volume patterns;
- route and guard selection;
- mailbox and Store/Filter interest;
- node and service availability;
- diagnostics that could reconstruct any of the above.

### 7.2 Adversaries

| Adversary | Capability | Required target response |
| --- | --- | --- |
| Local passive observer | Sees IPs, ports, TLS handshake metadata, timing, sizes | WSS/HTTPS transport, fixed cells in anonymous profiles, padding/cover according to profile |
| Active censor/prober | Blocks protocols, scans bridges, replays handshakes, forces failures | Multiple signed bootstrap sources, authenticated hidden WSS upgrade, ordinary unauthenticated response, no silent downgrade |
| Ordinary Waku Relay | Sees Waku participation, carrier/shard, route/content topic equality, time and size | Encrypted payload, opaque selectors, route buckets, fixed route cells |
| Store/Filter/Lightpush provider | Links requesting PeerID/IP to opaque selector | Use egress proxy in onion/mix profiles so provider sees egress, not origin |
| One malicious privacy relay | Observes its input/output and can delay/drop/replay | Sphinx per-hop protection, replay detection, route diversity, E2E ACK/retry |
| Colluding entry and egress | Correlates source and destination | Mix delays/cover reduce correlation; no absolute guarantee |
| Sybil/local-view attacker | Adds relays or gives a client a unique directory | Threshold-signed common epoch document, admission, family/operator diversity |
| Global passive observer | Observes all links and timing | Only mix profiles target meaningful resistance; guarantee depends on cover, traffic, honest hop, and measured parameters |
| Active global observer | Drops, delays, tags, floods, or partitions | Integrity, replay defense, loop probes, bounded retry, fail-closed profile truth; residual risk remains |
| Compromised capability holder | Reads and emits valid channel traffic | Identity signatures, Policy, revocation, new channel secret, quotas |
| Compromised endpoint | Reads local plaintext and keys | Outside network anonymity guarantee; local hardening and incident response apply |

### 7.3 Minimum Knowledge-Separation Invariant

For `onion` and both `mix` profiles, no single non-colluding route relay may
learn both:

- the origin network endpoint or directly identifying ingress context; and
- the final Waku selector or provider operation.

The entry may know the source but not the final selector. The egress may know
the final opaque selector but not the source. Middle relays know neither.

## 8. Independent Runtime Axes

The runtime state is the product of four independent decisions:

```text
TransportProfile x PrivacyProfile x OperationalMode x DeliveryPolicy
```

Example:

```text
tcp_wss x onion x restricted_defense x reliable
```

This means:

- changing QUIC to WSS does not permit changing `mix_balanced` to `direct`;
- losing mix directory quorum does not imply TCP failure;
- retry policy does not change selector derivation;
- an operator can see exactly which guarantee or capability was lost.

## 9. Target Layering

```text
+-------------------------------------------------------------+
| Product domains: Discovery, Publication, Data, applications |
+-------------------------------------------------------------+
| Delivery frame: delivery_id, attempt, ACK, fragmentation     |
+-------------------------------------------------------------+
| ardents-private/1: capability, E2E encryption, signature     |
+-------------------------------------------------------------+
| optional ardents-route/1: Sphinx path, SURB, fixed cell      |
+-------------------------------------------------------------+
| Waku: Relay / Store / Filter / Lightpush                     |
+-------------------------------------------------------------+
| libp2p security and multiplexing                             |
+-------------------------------------------------------------+
| TCP | WSS over HTTPS | future QUIC                           |
+-------------------------------------------------------------+
```

The fully sealed `ardents-private/1` envelope is the payload of the route
packet. Its associated data remains bound to the final Waku pubsub and content
topics. An entry or middle relay cannot change the final destination without
causing final authentication failure.

`ardents-route/1` owns only privacy forwarding. It never interprets discovery,
publication, data, conversation, service, or blob semantics.

## 10. Profile Topologies

### 10.1 Direct

```text
Origin
  -> Waku Relay/Lightpush
  -> Waku network
  -> recipient Relay/Filter/Store
```

Properties:

- current `ardents-private/1` confidentiality;
- lowest latency and overhead;
- origin PeerID/IP can be linked to direct providers;
- timing and volume correlation remain;
- no sender-recipient unlinkability claim.

### 10.2 Onion

```text
Origin
  -> Entry Guard
       -> Middle
            -> Egress Service
                 -> final Waku operation
                      -> Recipient
```

Every arrow between privacy relays is a Waku-carried route cell.

Properties:

- three distinct privacy relays;
- fixed-size Sphinx cells;
- no deliberate mixing delay beyond bounded queue jitter;
- source and final selector separated against one non-colluding relay;
- vulnerable to a global or entry/egress timing correlator;
- suitable for interactive flows that cannot tolerate seconds of mix delay.

### 10.3 Mix

```text
Origin
  -> Ingress Gateway
       -> Mix Layer 1
            -> Mix Layer 2
                 -> Mix Layer 3
                      -> Egress Service
                           -> final Waku operation
```

At every mix layer, the selected relay:

1. authenticates and unwraps one Sphinx layer;
2. rejects replay;
3. holds the packet for its encrypted, policy-bounded delay;
4. emits it among real, loop, and drop cover cells;
5. forwards it through the next Waku route bucket.

Properties:

- five privacy hops: ingress, three mixes, egress;
- one relay selected from each of three independent mix strata;
- fixed cell geometry;
- Poisson-style sending and per-hop delays;
- cover and loop traffic;
- responses through SURBs;
- higher latency, bandwidth, CPU, and battery cost;
- stronger timing-correlation resistance only while the anonymity-set,
  diversity, and cover gates are satisfied.

## 11. Privacy Profile Contract

| Profile | Route | Intentional delay | Cover traffic | Primary guarantee | Expected cost |
| --- | ---: | --- | --- | --- | --- |
| `direct` | 0 privacy hops | none | none | semantic confidentiality | lowest |
| `onion` | 3 hops | no, only bounded jitter | connection/circuit padding only | source-final-selector separation from one relay | low/medium |
| `mix_balanced` | ingress + 3 mixes + egress | seconds-scale cohort parameters | mandatory bounded cover | practical metadata protection with interactive tolerance | high |
| `mix_strong` | ingress + 3 mixes + egress | larger cohort-wide delay | mandatory stronger cover | strongest supported traffic-analysis resistance | highest |

Exact mix delay and cover parameters must be common epoch parameters, not
per-user configuration. Per-user values partition the anonymity set and create
fingerprints.

`mix_balanced` and `mix_strong` must not report `ready` if:

- fewer than the required eligible relays exist in a layer;
- directory quorum is absent or stale beyond its grace window;
- path diversity constraints cannot be satisfied;
- cover generation is disabled or below the accepted budget;
- queue or replay state cannot be maintained;
- the selected Waku realm cannot carry the required cell rate;
- the dependency/audit gate for the packet engine has not passed.

## 12. Route Directory And Trust

Anonymous path selection requires a common, authenticated view. Letting every
bootstrap peer give each client a different route list enables Sybil,
fingerprinting, and local-view attacks.

### 12.1 Ownership

The route directory is Discovery-owned trust-aware knowledge consumed by
Network Foundation. It is not a new product domain and not a second network
foundation.

Identity owns relay identity and signing material. Policy owns admission and
eligibility decisions. Diagnostics projects aggregate directory health.

### 12.2 Epoch Document

All route-capable clients use one threshold-signed `RouteEpochDocument`:

```text
RouteEpochDocument {
  version
  network_id
  epoch
  valid_from
  valid_until
  geometry_id
  route_bucket_parameters
  privacy_profile_parameters
  relay_descriptors[]
  authority_signatures[]
}
```

Each relay descriptor contains:

- stable Ardents relay identity;
- operator and relay-family identifiers;
- allowed roles: ingress, mix layer, egress;
- epoch Sphinx/KEM public key;
- supported Waku roles and transport profiles;
- signed Waku reachability information;
- capacity class and admission state;
- validity interval;
- descriptor signature.

It must not contain client membership, channel selectors, current circuits, or
traffic statistics fine-grained enough to identify a user.

### 12.3 Authority Model

The initial target is at least three independently operated directory
authorities with a two-of-three threshold acceptance rule. A deployment may
raise the threshold but must not silently reduce it.

If fresh quorum is unavailable:

- an already accepted document may be used for at most one explicitly defined
  grace epoch;
- new routes are denied after the grace limit;
- existing packets may drain within their bounded lifetime;
- diagnostics report directory degradation without exposing selected routes.

### 12.4 Relay Admission

A route relay must have:

- a valid relay capability issued by an accepted realm authority;
- a valid signed descriptor;
- a current epoch key;
- observed Waku participation;
- required role capability;
- acceptable dependency/security version;
- capacity and health above the profile threshold.

Configuration alone is not admission truth.

## 13. Path Selection

### 13.1 Universal Constraints

A route must not:

- select the same relay twice;
- select two relays with the same operator or declared family;
- select two relays from the same prohibited network prefix;
- select an egress that cannot perform the required Waku role;
- use an untrusted, expired, quarantined, or unreachable descriptor;
- use a route that violates local Policy.

Where data is available, selection should also diversify:

- ASN;
- hosting provider;
- jurisdiction;
- physical region;
- software version;
- failure domain.

Unknown diversity data is not treated as proof of independence.

### 13.2 Entry Guards

Continuously choosing a new first hop increases the probability of eventually
selecting a malicious entry. The client therefore maintains a small guard set:

- three eligible guards by default;
- one active guard and warm alternatives;
- multi-day stickiness;
- replacement only on expiry, revocation, persistent observed failure, or
  Policy action;
- no rotation merely to make the route look dynamic.

Guard identity and changes are local secret operational state. Diagnostics
show guard health and replacement reason, not the guard identifier.

### 13.3 Circuit Bounds

An onion circuit is bounded by:

- maximum wall-clock lifetime;
- maximum cell count;
- maximum idle time;
- directory epoch validity;
- relay revocation and route-health signals.

The initial values must be cohort-wide and derived from measured behavior.
They must not be freely randomized per client.

Mix routes are normally packet routes rather than long-lived streams. A client
may cache route templates and SURBs but constructs a fresh Sphinx packet for
each cell.

## 14. Waku Topic And Route-Bucket Strategy

### 14.1 Direct Private Selectors

Direct product traffic continues to use the capability-derived content topic
from `ardents-private/1`. No readable owner, operation, service, blob, request,
conversation, or principal value is added.

### 14.2 Route Topics

Anonymous forwarding must not allocate a visible topic per user, circuit,
channel, final selector, or relay.

Instead, relays are assigned to rotating route buckets. The logical derivation
is:

```text
route_epoch_key = HKDF(
  realm_route_secret,
  "ardents-route/1" || network_id || epoch)

bucket_token = Trunc160(HMAC(
  route_epoch_key,
  profile_class || layer || bucket_number))

content_topic =
  "/ardents-route/1/" || base32(bucket_token) || "/proto"
```

The exact byte contract must be frozen in the future protocol document.

Properties:

- network-visible values are opaque to parties without the route realm
  capability;
- all participants change buckets on a common epoch boundary;
- route role and layer are not readable from the token;
- multiple relays consume one bucket;
- the intended relay is learned only by successful Sphinx processing;
- old/new bucket overlap is bounded and cohort-wide.

The epoch document chooses bucket count so each active bucket has a minimum
eligible relay cohort. If the cohort is too small, the strong profile is not
ready. More buckets improve efficiency but reduce the anonymity set.

### 14.3 DoS Consequence

Bucket consumers may need to attempt bounded Sphinx admission on packets not
addressed to them. Therefore:

- Waku/RLN or another accepted unlinkable admission proof is checked before
  expensive processing;
- packet length and geometry are checked first;
- per-bucket, per-peer, and global budgets apply;
- invalid packets never create durable replay entries;
- overload narrows participation explicitly rather than exhausting the node.

A reusable principal or capability identifier must not be placed in the route
message to solve rate limiting. That would create a new tracking handle.

## 15. `ardents-route/1` Packet Contract

The final byte-level protocol requires a separate normative specification.
This architecture fixes its required shape.

### 15.1 Outer Waku Message

- content topic: current route-bucket selector;
- payload: exactly one supported Sphinx geometry;
- ephemeral: true for intermediate route traffic;
- timestamp: cohort policy with Waku-compatible bounded fuzz;
- anti-spam proof: accepted Waku RLN proof or an independently reviewed
  unlinkable one-show admission token.

Intermediate route cells must not be retained by Waku Store.

### 15.2 Sphinx Payload

The packet engine owns:

- per-hop route encryption and authentication;
- packet transformation;
- hidden remaining route;
- replay tag;
- per-hop delay command;
- SURB construction and use;
- fixed geometry.

Ardents owns only the final service payload and integration:

```text
RouteServiceRequest {
  protocol_version
  action
  final_pubsub_topic
  final_content_topic
  private_envelope
  reply_surb_or_mailbox
  expiry
}
```

Allowed final actions initially:

- `WAKU_PUBLISH`;
- `WAKU_LIGHTPUSH_SUBMIT`;
- `WAKU_STORE_QUERY`;
- `WAKU_FILTER_ATTACH`;
- `WAKU_FILTER_DETACH`.

The action and final topics become visible only to the egress service. Product
semantics remain inside `private_envelope`.

### 15.3 Geometry

The initial target is one network-wide fixed route packet geometry near the
Waku recommended average message size, with 4 KiB as the design starting
point. The final value depends on the selected Sphinx implementation's exact
overhead and Waku serialization measurements.

Rules:

- only a small, versioned set of geometry IDs may exist;
- a client cannot choose arbitrary cell size;
- geometry is part of the signed epoch document;
- all cells in one profile/epoch are indistinguishable by length;
- the complete Waku message remains below the Waku absolute size limit;
- large objects use Data Substrate chunking into cells;
- a high-privacy operation never falls back to an unshaped large transfer.

If a payload cannot be carried under the selected privacy profile and bounds,
the operation fails explicitly.

### 15.4 Replay

Every privacy relay maintains a bounded replay filter/ledger for Sphinx replay
tags covering the maximum packet lifetime and directory overlap.

Requirements:

- authentication precedes durable admission;
- replay state survives restart where a restart could otherwise reopen the
  accepted packet window;
- saturation rejects new packets fail closed;
- raw packet, route, selector, or client identity is not retained;
- route replay and `ardents-private/1` message replay remain separate security
  boundaries.

## 16. Mix Scheduling And Cover Traffic

### 16.1 Poisson-Style Emission

In mix profiles:

- clients emit real and cover cells according to common profile distributions;
- each mix delay is sampled from the profile's exponential distribution and
  carried inside its protected hop command;
- relays queue by monotonic deadline and emit without exposing whether a cell
  is real or cover;
- mix relays generate loop traffic that returns through the mix and provides
  both cover and active health evidence;
- drop cover terminates only at its intended protected command.

The implementation must use a monotonic clock for queue deadlines and an
injectable deterministic source for tests. Cryptographic randomness uses the
operating-system CSPRNG.

### 16.2 Parameter Governance

The following values are signed epoch/profile parameters:

- send-rate distribution;
- per-layer delay distribution;
- maximum delay;
- cover and loop rates;
- queue bounds;
- geometry;
- route lifetime;
- minimum relay cohort;
- congestion/backpressure policy.

They are calibrated through simulation, multi-host measurement, and anonymity
evaluation. Arbitrary constants chosen from intuition are not a security
argument.

### 16.3 Mandatory Cover Gate

A mix profile without cover traffic is not a degraded mix profile; it is an
invalid claim.

If cover cannot be maintained because of:

- Waku/RLN rate limits;
- bandwidth quota;
- battery policy;
- provider capacity;
- queue pressure;
- insufficient active participants;

the runtime must either:

- keep the requested operation queued within its explicit deadline;
- transition to an operator-visible unavailable/degraded privacy state; or
- use another profile only when Policy explicitly permits that downgrade.

It must not silently send direct traffic.

### 16.4 Public Waku Capacity Constraint

The public Waku network has message-size and RLN rate policies that may be
incompatible with continuous cover traffic at a useful rate. Therefore:

- no mix capacity claim may assume free public-network bandwidth;
- cover cells require the same admission and spam controls as data cells;
- the target mix profile normally requires an Ardents-operated Waku realm with
  explicit capacity and abuse policy;
- use of public Waku shards requires a separate measured capacity, RLN, cost,
  and reliability decision;
- creating that realm does not replace Waku: Relay, Store, Filter, and
  Lightpush remain the carrier roles.

## 17. Reply And Provider Privacy

### 17.1 Replies And ACKs

The origin includes one or more fresh SURBs in the protected final request.
The responder or egress uses one SURB once.

SURB rules:

- single use;
- bounded expiry;
- no logging or diagnostics exposure;
- no reuse after timeout uncertainty;
- replenished before the local pool falls below profile minimum;
- separate pools per privacy profile and epoch.

### 17.2 Anonymous Publish

```text
Origin -> route -> Egress -> Waku Publish/Lightpush -> Recipient
```

The entry sees the origin but not the final selector. The egress sees the final
opaque selector but not the origin.

### 17.3 Anonymous Store Query

```text
Client -> route -> Store Egress -> Waku Store provider
                                  -> encrypted results
Client <- SURB route <------------+
```

The Waku Store provider links the query to the egress, not the client. The
egress sees the opaque selector and query range but not the client endpoint.
This does not hide selector equality from the egress and is not private
information retrieval.

### 17.4 Anonymous Filter

A strong-profile client must not directly give its final private selector to a
Filter provider.

Instead:

1. the client establishes an authenticated route mailbox or supplies a bounded
   SURB pool;
2. a Filter egress subscribes to the final opaque selector;
3. matching encrypted `ardents-private/1` envelopes are returned through route
   cells;
4. the client refreshes the attachment through the anonymous route;
5. expiry or SURB exhaustion terminates the attachment explicitly.

The Filter provider sees the egress PeerID and selector. The ingress sees the
client but not the final selector.

### 17.5 Light Clients

Light clients may use WSS and Waku Lightpush/Filter to communicate with the
ingress route bucket. This preserves Waku's light-client role while preventing
the client's direct provider from learning its final product selector.

## 18. Delivery Reliability

Mix networks and Waku are both best-effort carriers. Product reliability must
therefore be explicit and end-to-end.

### 18.1 Delivery Identity

Each logical delivery has:

- random `delivery_id`, stable across retries and encrypted end to end;
- random `attempt_id` for each route attempt;
- fresh `ardents-private/1` message ID and nonce per attempt;
- fresh Sphinx packet and SURB per cell/attempt;
- domain-owned idempotency key where the operation has side effects.

Security replay and product idempotency are different:

- replay protection rejects re-use of an authenticated network envelope;
- idempotency prevents the same logical operation from being applied twice
  after a legitimate retry.

### 18.2 State Machine

```text
queued
  -> route_building
  -> dispatched
  -> egress_accepted
  -> recipient_acked
  -> completed

terminal alternatives:
  denied | expired | unavailable | cancelled | permanently_failed
```

`egress_accepted` means only that the final route service accepted the Waku
operation. It is not recipient delivery.

`recipient_acked` requires an authenticated end-to-end receipt produced after
the recipient decrypted, authorized, replay-checked, and durably admitted the
delivery.

`completed` may require a domain-specific terminal result in addition to
network receipt.

### 18.3 Retry

- retry uses a fresh route packet and normally a different middle/egress path;
- the entry guard remains sticky unless it is the observed failure;
- timeout includes profile delay distribution and current health;
- backoff is bounded and randomized at cohort-compatible granularity;
- maximum attempts and delivery deadline are explicit;
- no retry continues after terminal denial, revocation, expiry, or
  cancellation;
- retry state and terminal fate survive restart.

The runtime maintains at least two prebuilt eligible circuits where the
profile and resources allow it, but sends one ordinary attempt at a time.
Racing identical real traffic across many routes creates correlation and load;
it is not the default reliability strategy.

### 18.4 Fragmentation

Large logical payloads are split before Sphinx construction.

The encrypted delivery frame carries:

- delivery ID;
- fragment index and total;
- content identity or manifest reference;
- integrity commitment;
- attempt information;
- bounded reassembly expiry.

Receivers:

- bound incomplete reassembly memory/disk;
- verify every fragment and final commitment;
- deduplicate across retries;
- delete terminal incomplete state;
- never expose fragment identifiers outside encryption.

Erasure coding is not part of the first target slice. It may be added only
after a mature dependency and a demonstrated loss/overhead benefit.

### 18.5 Offline Delivery

Intermediate privacy relays do not become durable message stores.

Offline support remains:

- final encrypted envelope retained through Waku Store where policy permits;
- Data Substrate for durable replicated objects;
- anonymous Store query through an egress;
- sender retry within an explicit delivery lifetime.

## 19. Transport Architecture

### 19.1 `tcp_only`

Role:

- compatibility and controlled-network fallback;
- real Waku participation;
- no HTTPS camouflage claim;
- useful when TCP is allowed and operational simplicity is more important than
  censorship resistance.

### 19.2 `tcp_wss`

Role:

- primary restrictive-network profile;
- Waku/libp2p carried through secure WebSocket;
- TCP port 443 where deployment policy permits;
- compatible with ordinary HTTPS infrastructure and browser-constrained
  clients.

Production requirements:

- current TLS 1.3 implementation;
- publicly trusted certificate for public endpoints;
- automated issuance and rotation through ACME or an equivalent accepted
  certificate-management system;
- TLS private key managed outside ordinary node state;
- 0-RTT disabled for authenticated Ardents/Waku control and publish paths
  unless a future replay-safe design explicitly proves otherwise;
- certificate expiry and renewal failure surfaced before outage;
- no plaintext `ws://` remote fallback;
- self-signed certificates limited to explicit local/test scenarios.

TLS certificate identity and Ardents identity remain separate:

- TLS authenticates the HTTPS endpoint;
- libp2p/Waku authenticates the peer connection;
- Ardents Identity and capabilities authorize product behavior.

### 19.3 Shared HTTPS Front

The target WSS endpoint may coexist with an ordinary HTTPS site on the same
domain, address, and port:

```text
Internet :443
  -> TLS/HTTP frontend
       -> ordinary site/routes
       -> authenticated WSS upgrade
            -> Waku/libp2p listener
```

Requirements:

- unauthenticated probing receives an ordinary site response or a response
  indistinguishable from the site's normal not-found behavior;
- upgrade authorization is capability-bound, short-lived, replay-resistant,
  and redacted from access logs;
- TLS terminates only on infrastructure inside the accepted trust boundary;
- reverse proxies must not log authorization headers, paths carrying secret
  material, or raw Waku metadata;
- the ordinary site is real and operational, not a static error-only decoy;
- SNI, IP, certificate, ALPN, timing, and TLS fingerprint leakage remain
  documented residuals.

This profile claims improved allow-list compatibility and active-probe
resistance, not perfect browser indistinguishability.

### 19.4 HTTP Versions

- RFC-compatible WebSocket over HTTP/1.1 is the baseline;
- WebSocket over HTTP/2 may be added through Extended CONNECT when the complete
  server/proxy/client chain supports it;
- protocol negotiation remains standard and does not use handwritten HTTP;
- permessage compression is disabled for protected binary traffic unless a
  security review proves no size or resource side channel;
- request, header, frame, idle, and connection limits are mandatory.

### 19.5 Future `tcp_quic`

QUIC can improve handshake latency, multiplexing, and path migration, but:

- UDP is frequently blocked in hostile environments;
- current Ardents runtime does not implement this profile;
- the active DTLS-related dependency exception prohibits accidental expansion
  into WebRTC/DTLS-bearing paths;
- adding QUIC requires dependency safety, transport truth, endpoint tests,
  vulnerability reclassification, and no change to privacy-profile semantics.

QUIC is an expansion path, not the only reliability path.

### 19.6 Pluggable Censorship Resistance

WebTunnel-like HTTPS camouflage, obfs4-like random transports, Snowflake-like
proxy discovery, and AmneziaWG-style WireGuard obfuscation solve different
blocking problems. None solves Waku topic, provider, or timing metadata.

Ardents may adopt one specific mature censorship adapter below Waku only if:

- Waku remains the carried foundation;
- the adapter is explicitly selected and dependency-reviewed;
- it does not become a generic plugin bus;
- its active fingerprint and failure behavior are measured;
- it has active-probe and bridge-distribution semantics;
- operator truth names the actual adapter;
- no product data bypasses `ardents-private/1`.

Ardents must not reimplement Tor WebTunnel, obfs4, browser TLS fingerprints, or
AmneziaWG packet mutation from scratch.

## 20. Connection And Transport Switching

### 20.1 Health Inputs

The transport controller observes:

- DNS/bootstrap outcome;
- dial success and latency;
- handshake and certificate outcome;
- peer continuity;
- role-capable peer availability;
- route construction success;
- packet loss and ACK latency;
- transport-specific resets or blocking patterns;
- policy/security signals;
- resource pressure.

### 20.2 Switching Rules

- use make-before-break when a replacement path can be established safely;
- maintain hysteresis and minimum dwell time to avoid flapping;
- distinguish peer failure, path failure, transport-family failure, directory
  failure, and privacy-profile failure;
- prefer a different peer on the same transport before changing transport when
  evidence points to one peer;
- prefer WSS when UDP appears blocked;
- return to a broader/faster profile only after sustained recovery;
- never widen endpoint exposure without Policy approval;
- never change privacy profile as an implicit consequence.

### 20.3 Multi-Provider Reliability

The node keeps independent candidates for:

- Relay mesh;
- Store query;
- Filter;
- Lightpush;
- route ingress;
- route egress.

Candidates should be diversified by operator and failure domain. Several
services on one machine or provider are not independent copies.

More nodes improve resilience and can enlarge an anonymity set only when
traffic, directory view, role distribution, and operator independence are also
diverse.

## 21. Admission, Spam, And DoS

Anonymity increases abuse risk because the receiver should not learn a stable
sender identifier.

### 21.1 Required Controls

- Waku GossipSub peer scoring and message validation;
- RLN where the selected Waku realm supports it;
- capability or privacy-preserving one-show admission at ingress;
- size and geometry checks before crypto;
- bounded concurrent unwraps;
- per-bucket and global token buckets;
- bounded delay queues;
- bounded SURB and reassembly state;
- expiry before enqueue;
- circuit and connection caps;
- backpressure instead of unbounded goroutines;
- load shedding that preserves control/health capacity;
- operator-visible attack and saturation reasons.

### 21.2 Anonymous Admission Direction

The strong target should avoid presenting a reusable principal to every
ingress. The preferred direction is an accepted Privacy Pass-style unlinkable
token or Waku-compatible RLN proof whose issuance and redemption contexts are
separated.

This is a dependency and protocol decision, not permission to invent an
anonymous credential. Until a reviewed mechanism is selected:

- a private realm may use capability-based ingress admission and document that
  the ingress can link the principal;
- the runtime must not claim principal-unlinkable public admission;
- `mix_strong` public release remains blocked if its threat model requires that
  property.

## 22. Key And Secret Lifecycle

### 22.1 Key Classes

| Material | Owner | Lifetime |
| --- | --- | --- |
| Ardents node signing identity | Identity | long-lived, explicit migration |
| Waku node key | Network Foundation | long-lived, bound to Waku continuity |
| Channel capability secret | Identity | capability generation |
| Route realm secret/capability | Identity | realm policy, explicit rotation |
| Privacy relay descriptor key | Identity | long-lived signing identity |
| Relay epoch Sphinx/KEM key | Privacy relay | one directory epoch plus bounded overlap |
| Per-packet Sphinx ephemeral material | Packet engine | one packet |
| SURB | Recipient | single use and short expiry |
| Route replay key/state | Network Foundation | deployment secret plus replay horizon |

### 22.2 Rules

- route and channel key schedules use distinct domain separation;
- no route key is stored with retained Waku payloads;
- old epoch private keys are destroyed after the accepted drain window;
- a revoked relay is excluded by signed emergency status and the next epoch
  document;
- active circuits using a revoked relay stop;
- missing/corrupt route keys or replay state fail closed;
- backups preserve only material required by the documented continuity model;
- logs, API, diagnostics, crash reports, and tests never contain route,
  selector, SURB, packet, or key material.

### 22.3 Cryptographic Agility

The route packet carries a versioned geometry/suite identifier selected by the
epoch document. Agility means an explicit migration between reviewed suites;
it does not mean negotiating arbitrary algorithms or falling back.

Post-quantum Sphinx/KEM variants may be evaluated later. They are not mixed
into the first implementation without a dependency, size, performance, audit,
and migration decision.

## 23. Diagnostics And Local Control Truth

The Network snapshot must expose:

- active transport profile;
- active privacy profile;
- operational mode;
- requested versus effective profile;
- downgrade policy and whether downgrade was denied;
- directory epoch, quorum state, and staleness class;
- eligible relay count per role/layer as bounded aggregates;
- diversity gate result;
- circuit pool health;
- route build and E2E ACK latency classes;
- Store/Filter/Lightpush proxy availability;
- cover-budget health as a coarse state;
- queue, retry, and terminal delivery counts;
- certificate validity/renewal state;
- reduced capabilities and recovery actions.

It must not expose:

- selected route or guard identity;
- raw directory descriptor keys;
- route bucket or product selectors;
- channel, delivery, grant, or SURB identifiers;
- per-user real-versus-cover classification;
- exact traffic timing capable of reconstructing a route;
- packet bytes, nonces, replay tags, or keys.

Initial stable reason vocabulary:

- `anonymity.directory.unavailable`
- `anonymity.directory.stale`
- `anonymity.directory.quorum_lost`
- `anonymity.route.insufficient_diversity`
- `anonymity.route.build_failed`
- `anonymity.route.replay_detected`
- `anonymity.route.queue_exhausted`
- `anonymity.surb.exhausted`
- `anonymity.cover.budget_unavailable`
- `anonymity.profile.unavailable`
- `anonymity.downgrade.denied`
- `transport.wss.certificate_invalid`
- `transport.wss.renewal_failed`
- `transport.family.blocked`
- `delivery.recipient_ack_timeout`

Reason codes must not distinguish real and cover cells on a per-packet
operator surface.

## 24. Failure Semantics

| Failure | Required behavior |
| --- | --- |
| One relay unavailable | Build another valid route without changing requested privacy profile |
| Entry guard unavailable | Use warm guard after bounded confirmation; record guard replacement reason |
| Directory quorum lost | Use bounded grace, then deny new anonymous routes |
| One mix layer below minimum | Mark mix profile unavailable; no direct fallback |
| Cover budget unavailable | Queue, deny, or explicitly authorized downgrade |
| WSS certificate expired | Do not expose invalid secure profile; use another allowed transport |
| UDP blocked | Prefer WSS/TCP; privacy profile unchanged |
| Store provider fails | Query another independent provider through a fresh route |
| Filter egress fails | Reattach through another egress with fresh SURBs |
| Egress accepts but ACK is lost | Retry with same delivery ID, fresh envelope/route/SURB |
| Duplicate logical delivery | Domain idempotency returns prior terminal result |
| Replay ledger saturated | Reject new packet and report degraded/failed security state |
| Relay compromise/revocation | Exclude from directory, stop affected routes, rotate epoch as required |
| Insufficient anonymity set | Do not claim mix readiness |

## 25. Dependency Decision

### 25.1 Prohibited

- handwritten Sphinx or onion cryptography;
- a custom mix-network wire transport between relays;
- a second generic network foundation;
- copying Tor or Nym protocols selectively without their required directory,
  replay, cover, and threat-model machinery;
- using a deprecated package merely because it has a convenient API;
- accepting license or vulnerability risk without the project dependency
  workflow.

### 25.2 Candidate Direction

The leading implementation direction is a maintained Sphinx-compatible
component from a mature mixnet project, with the current Katzenpost monorepo as
a candidate for evaluation because it provides:

- a parameterized Sphinx implementation;
- SURBs;
- replay specifications;
- path and PKI models;
- active implementation and protocol documentation.

This is not dependency acceptance. Before selection, the dependency review
must verify:

- exact maintained module/package path;
- release and compatibility policy;
- license compatibility for library and linking/distribution shape;
- security audits and unresolved issues;
- primitive and parameter choices;
- Windows/Linux support;
- transitive dependency and vulnerability graph;
- ability to use only the packet engine without adopting Katzenpost as a
  second foundation;
- deterministic test vectors and cross-version migration.

Nym, Tor, Katzenpost full-network sidecars, WebTunnel, Privacy Pass, and other
candidates remain research inputs until their role and dependency posture are
accepted explicitly.

### 25.3 Preliminary Katzenpost Posture

As of the research date:

- the Katzenpost monorepo and specifications show active maintenance and a
  current release process;
- the required Sphinx/SURB/replay mechanisms exist in the maintained monorepo;
- the project is licensed under AGPLv3;
- importing its packet engine would make it a direct critical dependency;
- the full Katzenpost runtime is intentionally outside the intended scope.

Recommendation: **do not accept the dependency yet**.

AGPLv3 compatibility is a release blocker until the Ardents license and
distribution model are explicit. Search results are not a vulnerability audit.
Acceptance additionally requires a pinned candidate version, exact imported
package graph, `govulncheck`, advisory review, audit review, platform tests,
license counsel/owner decision, and proof that only the packet engine is
adopted without its competing wire/network foundation.

## 26. Considered Alternatives

### 26.1 Keep Only Opaque Waku Topics

Rejected as the target because it hides meaning but preserves selector
equality, provider linkability, timing, volume, PeerID, and IP relationships.

### 26.2 Replace Waku With Tor, Nym, Or Katzenpost

Rejected for `v1` because it violates the canonical foundation decision and
would move discovery, messaging, publication, and operational truth onto a
different substrate.

Their mechanisms and mature components may be reused only under Ardents
boundaries and dependency rules.

### 26.3 Run A Complete Second Mixnet Beside Waku

Rejected as the default architecture because:

- it creates two operational foundations;
- role and readiness truth become ambiguous;
- mobile Store/Filter/Lightpush semantics split;
- failures and delivery guarantees become difficult to explain;
- operators must secure and maintain two networks.

### 26.4 Implement Simple Nested HPKE Per Hop

Rejected as the target packet format. Although it looks simpler, safely hiding
route length, preventing tagging, transforming packet appearance, supporting
replies, and handling replay recreates a substantial part of Sphinx.

### 26.5 Use HTTPS Alone

Rejected as an anonymity solution. HTTPS protects link contents and can improve
censorship resistance, but endpoints, timing, size, TLS metadata, Waku
providers, and topics remain observable at their respective boundaries.

### 26.6 Rotate Topic For Every Message

Rejected because it destroys practical Filter/Store subscription behavior,
creates synchronization and recovery failure, increases routing overhead, and
does not prevent timing correlation.

## 27. Implementation Plan

Work starts only after the active stabilization loop reaches its applicable
gate. At that point a dedicated process directory must be created from
`docs/process/process-template/`. The phases below are target phases, not
current task statuses.

### Phase A — Formalize Claims And Select Dependencies

Deliver:

- accepted version of this architecture;
- updated privacy, transport, network/discovery, persistence, and domain docs;
- formal threat/guarantee matrix per profile;
- Sphinx/Katzenpost/Nym/Tor mechanism comparison;
- dependency-safety report for packet engine candidates;
- license decision;
- Waku realm/RLN capacity model for route and cover traffic;
- non-product discrete-event anonymity/capacity simulator;
- protocol review by an external anonymity specialist.

Gate:

- no handwritten crypto or unreviewed dependency remains in the selected path;
- profile claims and non-claims are testable;
- public versus operated Waku realm decision is explicit;
- exact owner for every state and secret is fixed.

### Phase B — Harden Transport And Connection Control

Deliver:

- production WSS configuration and certificate lifecycle;
- shared HTTPS front with safe unauthenticated behavior;
- multi-source bootstrap and restricted bridge bundles;
- transport health classification and hysteresis;
- make-before-break peer/path replacement;
- certificate and switching diagnostics;
- active-probe, block, expiry, renewal, and restart scenarios.

Gate:

- WSS works as a real Waku profile across supported platforms;
- self-signed material cannot become production truth;
- transport switching never changes privacy profile;
- failure remains explainable.

### Phase C — Build The Common Route Directory

Deliver:

- threshold-signed epoch document;
- relay descriptor and capability lifecycle;
- common-view consistency checks;
- family/operator/network diversity;
- guard-set persistence;
- route-bucket derivation;
- revocation and stale-directory behavior.

Gate:

- clients reject unique or under-signed local views;
- Sybil/family constraints are enforced;
- route selection is deterministic under test inputs and unpredictable under
  production randomness;
- directory and guard secrets are redacted.

### Phase D — Implement `ardents-route/1` Onion Profile

Deliver:

- accepted Sphinx packet engine integration;
- fixed geometry and route-bucket carriage through real Waku;
- three-hop onion route;
- per-hop replay defense and bounded queues;
- egress Waku publish/Lightpush action;
- SURB reply;
- onion diagnostics and no-downgrade Policy.

Gate:

- one compromised relay capture cannot recover both source and final selector;
- tamper, replay, wrong epoch, wrong geometry, stale route, and queue exhaustion
  fail explicitly;
- all hop traffic is Waku-carried;
- `ardents-private/1` remains intact end to end.

### Phase E — Proxy Store And Filter Privately

Deliver:

- anonymous Store query through egress;
- Filter attach/detach through egress;
- bounded SURB/mailbox lifecycle;
- egress failover;
- mobile/light-client WSS flows;
- provider-linkability capture tests.

Gate:

- final Store/Filter provider sees egress rather than origin;
- ingress never receives the final selector;
- offline retrieval and live filtering survive one provider failure;
- no direct fallback occurs in onion/mix profiles.

### Phase F — Add End-To-End Reliable Delivery

Deliver:

- delivery frame and state machine;
- recipient-authenticated ACK;
- retry, deduplication, idempotency binding, fragmentation, cancellation, and
  restart recovery;
- independent provider/path failover;
- bounded queues and backpressure.

Gate:

- carrier acceptance is never reported as recipient delivery;
- duplicate attempts cause one domain effect;
- terminal fate survives restart;
- loss, delay, duplication, reorder, partition, and ACK-loss scenarios pass.

### Phase G — Activate Mix Profiles

Deliver:

- ingress + three mix layers + egress topology;
- Poisson-style delays and emission;
- loop and drop cover;
- common epoch parameters;
- anonymity-set and cover-budget readiness gates;
- multi-host timing/correlation evaluation;
- resource and battery profiles;
- mix-specific DoS controls.

Gate:

- no mix profile can run without fixed cells, delay, cover, replay, and common
  directory;
- measured latency and overhead are within declared product bounds;
- a defined traffic-correlation classifier fails to distinguish accepted
  real/cover and route relationships at the required threshold;
- low-population and cover-loss states remove the mix readiness claim.

### Phase H — Adversarial Qualification And Release

Deliver:

- external cryptographic/protocol review;
- release vulnerability and dependency reviews;
- active probing and censorship lab;
- Sybil, eclipse, malicious relay, tagging, replay, flooding, and route-kill
  campaigns;
- multi-AS/multi-provider topology evidence;
- multi-day soak;
- operator deployment, backup, rotation, incident, and recovery guides;
- final guarantee matrix exposed by product documentation.

Gate:

- no unresolved blocker exists for a claimed profile;
- all accepted residuals have containment, detection, and review trigger;
- runtime, docs, diagnostics, packet captures, and tests agree;
- no mandatory property is deferred behind a production profile name.

## 28. QA And Evidence Model

Every integration and E2E path must follow `docs/qa/test-model.md` with a
scenario document and formal metadata.

### 28.1 Unit

- epoch and bucket derivation vectors;
- directory signature/quorum;
- path diversity and guard rules;
- delivery state transitions;
- retry bounds;
- fragmentation and reassembly;
- profile availability gates;
- redaction.

### 28.2 Integration

- Sphinx interoperability vectors;
- real Waku route bucket across three relays;
- egress publish/Store/Filter/Lightpush;
- replay across restart;
- directory rollover and relay revocation;
- WSS certificate/upgrade;
- profile switching without privacy downgrade;
- queue and cover budget exhaustion.

### 28.3 E2E

- direct, onion, balanced mix, and strong mix flows across separate processes;
- mobile Lightpush/Filter path;
- origin offline and Store recovery;
- recipient ACK loss and idempotent retry;
- peer, provider, guard, middle, egress, and directory-authority failure;
- DNS blocking, UDP blocking, active probing, and certificate expiry;
- multi-provider partition and recovery;
- clean operator-visible terminal fate.

### 28.4 Adversarial

- single-hop and entry/egress collusion captures;
- timing-correlation baseline and defended comparison;
- packet tagging/tamper;
- route replay;
- Sybil/local-view directory;
- bucket CPU amplification;
- cover starvation;
- circuit-kill/forced-route-selection;
- selector and topic capture;
- diagnostics/log/report secret scan.

### 28.5 Performance And Soak

Measure at minimum:

- startup and route-build latency;
- direct/onion/mix delivery distributions;
- cover overhead;
- Waku message and RLN budgets;
- CPU per attempted/valid Sphinx packet;
- queue memory and disk;
- circuit and SURB pool size;
- Filter/Store provider load;
- mobile wakeups and battery impact;
- recovery time after each failure class;
- multi-day resource growth and delivery loss.

## 29. Release Claim Matrix

A release must publish a table of actual guarantees similar to:

| Claim | Direct | Onion | Mix |
| --- | --- | --- | --- |
| Payload confidentiality | required | required | required |
| Product semantic hiding | required | required | required |
| Source hidden from final Waku provider | no | yes, absent collusion | yes, absent collusion |
| Final selector hidden from ingress | no | yes | yes |
| Resistance to one malicious route relay | n/a | required | required |
| Global timing-correlation resistance | no | no | conditional measured target |
| Cover traffic | no | limited padding | mandatory |
| Offline retrieval | direct Store | Store egress | Store egress |
| No silent direct fallback | n/a | required | required |

The word `anonymous` must not appear in a product status without the effective
profile and its current readiness gate.

## 30. Residual Risk Register

The implementation must continuously track:

- small or partitioned anonymity sets;
- operator/ASN/provider concentration;
- entry/egress collusion;
- global active traffic confirmation;
- WSS/TLS fingerprinting and IP blocking;
- bridge enumeration;
- cover traffic cost and public Waku RLN limits;
- mix latency and mobile battery cost;
- Sphinx dependency and license posture;
- route-directory authority compromise;
- relay queue DoS and bucket amplification;
- malicious but valid capability holders;
- compromised endpoints;
- future cryptographic migration.

No item is closed merely because payload encryption is strong.

## 31. External Technical Basis

This target is informed by the following primary specifications and research:

- Waku content-topic privacy and k-anonymity guidance:
  https://docs.waku.org/learn/concepts/content-topics
- Waku protocol delivery and Store/Filter/Lightpush limits:
  https://docs.waku.org/learn/concepts/protocols/
- Waku pseudonymity and provider-linkability limits:
  https://docs.waku.org/learn/security-features/
- Waku public-network shards, RLN policy, and message-size limits:
  https://rfc.vac.dev/waku/standards/core/64/network/
- Tor onion routing and relay cells:
  https://spec.torproject.org/tor-spec/routing-relay-cells.html
- Tor guard rationale and path diversity:
  https://spec.torproject.org/guard-spec/
  and
  https://spec.torproject.org/path-spec/path-selection-constraints.html
- Tor connection/circuit padding:
  https://spec.torproject.org/padding-spec/connection-level-padding.html
  and
  https://spec.torproject.org/padding-spec/circuit-level-padding.html
- Sphinx packet format:
  https://eprint.iacr.org/2008/475
- Loopix Poisson mixing and cover traffic:
  https://www.usenix.org/conference/usenixsecurity17/technical-sessions/presentation/piotrowska
- Katzenpost specifications and maintained implementation direction:
  https://katzenpost.network/docs/specs/
- TLS 1.3:
  https://www.rfc-editor.org/info/rfc9846/
- WebSocket:
  https://www.rfc-editor.org/info/rfc6455/
- WebSocket over HTTP/2:
  https://www.rfc-editor.org/info/rfc8441/
- ACME:
  https://www.rfc-editor.org/info/rfc8555/
- QUIC:
  https://www.rfc-editor.org/info/rfc9000/
- Privacy Pass architecture:
  https://www.rfc-editor.org/rfc/rfc9576.html
- Tor WebTunnel as an example of HTTPS camouflage:
  https://blog.torproject.org/introducing-webtunnel-evading-censorship-by-hiding-in-plain-sight/

These references provide mechanisms and threat-model guidance. They do not
automatically make their implementations or networks Ardents dependencies.

## 32. Acceptance Of This Architecture

Accepting this target means agreeing that:

- Waku remains the carrier;
- the current privacy protocol is necessary but not a complete metadata
  privacy solution;
- onion and mix behavior are product-grade profiles, not optional labels;
- Sphinx, directory consistency, replay, cover, SURBs, provider proxying,
  reliability, diagnostics, and adversarial evidence are one inseparable
  security boundary;
- a requested privacy guarantee fails closed unless Policy explicitly
  authorizes a visible downgrade;
- dependency and independent security review are release gates, not cleanup
  work.

The implementation is not complete when packets can traverse several nodes.
It is complete only when the advertised guarantee survives the defined
adversaries, failures, resource limits, and operator workflows.
