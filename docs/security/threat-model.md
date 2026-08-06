# Threat model

Status: **proposed; research must turn claims into measurable contracts**

Last reviewed: 2026-08-06

## Scope

This threat model covers the first tracer product: resolving and opening a named
private site and publishing updates. It also sets constraints for later
Application Data transport, Spaces, and stateful Private Services.

Security is evaluated across payload, metadata, endpoint, availability,
software supply chain, and governance. Success in one dimension must not be
presented as success in another.

## Protected assets

- site and Application Data payload confidentiality and integrity;
- publisher and visitor network location;
- relationship, Persona, membership, and service-use graphs;
- recovery authority and Device revocation state;
- Service Name continuity and release authenticity;
- application access patterns and retained-data availability when retention is
  part of a later accepted contract;
- Client execution boundary and local secrets;
- control-plane integrity, operator diversity, and update provenance.

## Adversaries

We assume the presence of:

- a local or ISP observer seeing one endpoint's traffic;
- a censor performing blocking, probing, and protocol fingerprinting;
- malicious, unreliable, or colluding relays and Replicas;
- a global passive observer capable of broad timing and volume correlation;
- Sybil actors able to create many identities and infrastructure Nodes;
- malicious Services attempting fingerprinting, exfiltration, or permission
  escalation;
- infrastructure seizure, legal coercion, and operator disappearance;
- a stolen or fully compromised Device;
- supply-chain compromise of dependencies, builds, or update channels;
- capture of bootstrap, naming, release, moderation, or emergency governance.

We do not assume that Nodes advertised under different identifiers have
independent ownership, networks, legal jurisdictions, or software supply chains.

## Route Profiles

### Interactive Route

Used for site retrieval and interactive application traffic. Its target promise
is to hide endpoint locations from the opposite endpoint and from any individual
ordinary intermediary while maintaining useful latency.

It does **not** claim strong unlinkability against a global timing-and-volume
observer. Applications using it must isolate Personas, Services, sessions, and
state so the fast path does not create unnecessary cross-context links.

### Shielded Route

Used for Application Data operations that require stronger metadata protection,
sensitive updates, and privacy-preserving proofs. Its target promise is stronger
sender-receiver unlinkability under a declared observer and relay-collusion
model.

The required delay distribution, traffic shaping, cover budget, anonymity-set
size, and measurable advantage over the Interactive Route remain research
questions. Higher latency and bandwidth consumption are expected product costs.

### Bridge entry

A Bridge helps a Client enter when ordinary participation is blocked. It is a
replaceable circumvention layer, not an anonymity guarantee by itself.

## Threat and response matrix

| Adversary | Representative attack | Required product response | Honest limitation |
|---|---|---|---|
| Censor / DPI | Block known peers, bootstrap, or fingerprints | Replaceable Bridges, multiple bootstrap channels, pluggable entry transports, rotation | No fixed disguise remains unblockable forever |
| Local observer | Observe entry address, timing, and volume | Bounded entry policy, multi-hop route, Shielded profile where needed | Interactive use remains correlation-sensitive |
| Global passive observer | Correlate both ends statistically | Only Shielded operations target this class; measure delay, mixing, cover, and observer advantage | No blanket protection claim before evidence |
| Malicious relays | Tag, delay, drop, or bias paths | Endpoint-selected diversity, integrity, isolation, bounded retry, ownership analysis | Guarantee depends on honest-path probability and real diversity |
| Sybil / flooding actor | Capture discovery or exhaust rendezvous, retained-data, and Replica capacity | Quotas, bounded work, Invites, local policy, anonymous consumable admission proofs | No universal proof of personhood |
| Malicious Service | Fingerprint Client or request broad access | Application isolation, explicit Capabilities, privacy lint, per-Service Persona and storage | A controlled endpoint can still disclose data |
| Seizure / operator loss | Inspect or remove infrastructure | Protected content, independent replication, replaceable instances, no single host truth | Availability needs real operator and jurisdiction diversity |
| Compromised Device | Steal active keys and local history | Recovery/Device separation, revocation, compartmented Personas, minimal retention | Network protocols cannot secure a controlled endpoint completely |
| Supply-chain attacker | Ship malicious official software | Reproducible builds, signed releases, staged updates, independent review, rollback protection | A single binary distribution root remains power |
| Governance capture | Control naming, bootstrap, releases, or emergencies | Explicit control map, bounded quorum, transparency, appeal, expiry, multiple implementations | A decentralized data plane does not remove governance |

## Claim format

No document or interface may say only “anonymous,” “private,” “secure,” or
“decentralized.” A durable claim must state:

1. **Information:** what is protected or kept available?
2. **Adversary:** from whom?
3. **Conditions:** required honest Nodes, traffic, diversity, time, and endpoint
   behavior.
4. **Measurement:** what experiment or analysis can falsify the claim?
5. **Limitation:** what remains visible or attackable?

## Security invariants

- Payload encryption is not metadata protection.
- Service Name ownership is not human identity.
- Device authorization is not transport identity.
- A Credential proves a predicate; a Capability grants bounded authority.
- Resolution records never contain an ordinary origin IP.
- A Replica cannot choose or rewrite an authenticated release.
- Route downgrade is visible and cannot happen silently for sensitive actions.
- Protected data has explicit retention and deletion semantics.
- Recovery, multi-device operation, revocation, and forward secrecy are designed
  together.
- Bootstrap, naming, software releases, and emergency policy are each treated as
  separate control roots.

## Open security research

The prioritized questions live in [the research queue](../research/questions.md).
No production architecture should be selected before the observer models,
naming control and privacy, application boundary, recovery model, and first
tracer availability contract are testable.
