# Network Privacy Architecture

## 1. Purpose

This document defines the minimum product-grade architecture for hiding
product meaning while keeping `Waku` as the canonical `v1` network substrate.

It complements:

- `docs/canonical-network-foundation.md`
- `docs/network-and-discovery-requirements.md`
- `docs/network-privacy-requirements.md`
- `docs/communication-contracts.md`
- `docs/reference-invariants.md`
- `docs/network-privacy-protocol.md`

The protocol document is authoritative for v1 wire fields, cryptographic
suite, capability lifecycle, replay, limits, and technical-alpha migration.

## 2. Architectural Rule

Ardents must not expose raw `Waku` topic, filter, or lightpush mechanics as the
primary client contract for private traffic.

Instead, the product must provide one canonical transport-facing contract that:

- publishes private traffic through encrypted network envelopes;
- derives opaque selectors from capability material;
- keeps `Waku` as the actual relay/store/filter/lightpush carrier;
- hides transport/profile complexity from the client;
- exposes active privacy posture and switching reasons through diagnostics.

## 3. Product-Facing Contract

The stable product-facing contract should stay at the level of product
operations, not raw transport selectors.

The required shape is:

- publish private envelope;
- subscribe using owner or shared capability material;
- fetch retained private envelopes;
- inspect current transport/privacy state;
- inspect degraded or defense posture;
- inspect whether automatic mitigation changed participation profile.

Clients must not be required to:

- choose a raw `Waku` transport family;
- construct plaintext content topics;
- construct plaintext filter selectors;
- reason about relay/store/filter/lightpush orchestration details.

## 4. Internal Components

The minimum internal architecture is:

### 4.1 Transport Facade

One canonical owner for network participation.

Responsibilities:

- accept product-level publish/subscribe/fetch requests;
- coordinate selector derivation, envelope protection, and runtime profile use;
- keep one runtime truth source for diagnostics and local control surfaces.

### 4.2 Selector Deriver

Derives opaque network-visible selectors from capability material.

Responsibilities:

- derive content-topic tokens;
- derive filter selectors;
- derive lightpush addressing inputs;
- support rotation and version tagging;
- prevent readable product meaning from appearing in network-visible selectors.

Readable owner, service, conversation, or blob semantics must never be emitted
directly by this component.

### 4.3 Privacy Envelope

Wraps product payloads before they enter `Waku`.

Responsibilities:

- encrypt private payloads before relay/store/filter/lightpush entry;
- carry only the metadata required for authorized delivery;
- support versioned framing for future rotation and compatibility;
- optionally support padding or size-shaping policy if required by threat
  posture.

### 4.4 Capability Resolver

Maps owner or shared capability material to the inputs required for selector
derivation and decryption.

Responsibilities:

- resolve whether the caller is authorized to derive a private selector;
- keep derivation material out of diagnostics and general API output;
- support revocation or rotation hooks.

### 4.5 Privacy Profile Controller

Owns adaptive exposure control above the same `Waku` foundation.

Responsibilities:

- choose the active privacy/participation profile;
- react to attack, degradation, and policy signals;
- keep switching explicit in runtime truth;
- avoid silent capability claims after profile reduction.

### 4.6 Transport Snapshot

Provides operator-visible truth.

It must expose:

- active privacy profile;
- active transport participation mode;
- switch reason;
- reduced capabilities;
- recovery state;
- whether the current mode is automatic or policy-forced.

It must not expose:

- raw private selectors;
- derivation secrets;
- decryptable capability material.

## 5. Data Flow

The canonical private-traffic flow is:

1. The client submits product payload plus capability reference.
2. The capability resolver validates derivation authority.
3. The selector deriver computes opaque network-visible selectors.
4. The privacy envelope encrypts and frames the payload.
5. The transport facade sends the protected envelope through the active
   `Waku` participation profile.
6. Subscription and fetch paths repeat the same derivation logic on the
   authorized side and only then decrypt the payload.

At no step may readable product meaning be required in network-visible routing
labels.

## 6. Adaptive Profiles

Profiles are variants of participation and exposure on top of the same
`Waku` foundation.

The initial expected shape is:

- `standard`: normal participation with the default privacy posture;
- `low_exposure`: reduced network surface during instability or suspicious
  pressure;
- `defense`: strongest exposure reduction allowed by product invariants;
- `recovery`: temporary controlled return toward the standard profile.

Profiles may change:

- on repeated dial or peer-collapse signals;
- on transport-family failure or instability;
- on suspicious metadata pressure or rate anomalies;
- on operator-forced policy;
- on local security posture changes.

Switching is valid only if:

- the product-facing contract stays stable;
- runtime truth reflects the actual active profile;
- reduced capabilities are explicit;
- recovery conditions are explicit.

## 7. Diagnostics Contract

Diagnostics and the local control surface must explain:

- which profile is active;
- why the profile changed;
- what capabilities are reduced;
- whether publication, filter, or fetch behavior is constrained;
- what the recovery path is.

Diagnostics must not reveal enough information to reconstruct private topic or
filter material.

## 8. Non-Negotiable Constraints

The architecture must not:

- introduce a second canonical network foundation;
- treat `Waku` as an implementation detail that disappears from operator truth;
- allow plaintext product semantics to leak into selectors;
- depend on client-side manual transport selection for safety;
- silently downgrade capabilities without diagnostics truth.

## 9. Minimum Code Shape

When implemented in the root `v1` codebase, the expected package-level
responsibilities are:

- `internal/network/api`: the single Network Foundation / Messaging facade;
- `internal/network/transport` and `internal/network/participation`: Waku-backed
  participation and carrier truth, without product message semantics;
- `internal/network/messaging`: network-owned message framing and Waku delivery;
- a cohesive privacy boundary inside `internal/network`, owning selector/key
  derivation, envelope framing, replay admission, and redaction-safe outcomes;
- Identity-owned capability grant/secret resolution with Policy-owned admission
  decisions, consumed through their domain APIs rather than copied into
  Network Foundation.

The exact package split may vary, but ownership must remain explicit and must
not devolve into generic transport plugins.
