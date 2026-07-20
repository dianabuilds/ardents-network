# Network Privacy Requirements

## 1. Purpose

This document fixes the mandatory privacy requirements for Ardents network
traffic on top of the canonical `Waku` foundation.

It does not replace `docs/canonical-network-foundation.md` or
`docs/network-and-discovery-requirements.md`. It defines how product semantics
must remain hidden while still using `Waku` relay, store, filter, and
lightpush.

The minimum product-grade architectural shape for this requirement is defined
in `docs/network-privacy-architecture.md`.
The versioned selector, envelope, capability, replay, and migration contract is
defined in `docs/network-privacy-protocol.md`.
Cross-surface request/response/error contract discipline is fixed separately in
`docs/communication-contracts.md`.

## 2. Scope

This document applies to:

- message publication and subscription;
- content-topic use;
- filter subscription selectors;
- lightpush submission selectors;
- discovery-related network messages;
- data announce / request / response traffic;
- adaptive privacy modes that reduce exposure during attack or degradation.

## 3. Core Rule

`Waku` remains the canonical network substrate for `v1`.

Ardents may add a privacy and orchestration layer above `Waku`, but that layer
must:

- preserve `Waku` as the actual carrier and transport foundation;
- expose one product-facing transport contract instead of raw transport
  plumbing;
- hide product semantics from non-authorized observers;
- keep privacy mode, degraded mode, and automatic switching explainable through
  diagnostics and the canonical local control surface.

## 4. Mandatory Privacy Properties

### 4.1 Opaque Topics And Selectors

Product semantics must not appear in plaintext in:

- content topics;
- filter selectors;
- lightpush addressing inputs;
- any other message-routing label derived from business meaning.

Human-readable labels such as:

- service type;
- conversation identity;
- blob purpose;
- owner identity;
- operation type;
- product domain name;

must not be used directly as network-visible topic or selector values.

### 4.2 Capability-Derived Addressing

Opaque content topics and filter selectors must be derived from secret material
or capability material available only to authorized holders.

At minimum this means:

- non-authorized observers cannot interpret topic/filter meaning;
- non-authorized observers cannot derive the selector for a private channel;
- possession of a node endpoint alone must not grant the ability to compute a
  private subscription selector.

### 4.3 Encrypted Payloads Before Network Entry

Product payloads must be encrypted before entering relay, store, filter, or
lightpush paths whenever the product semantics are private.

The network carrier may observe ciphertext and delivery metadata, but it must
not receive readable product payloads by default.

### 4.4 Filter Privacy

If Ardents uses `Waku` filter for light/mobile participation, filter
subscriptions must use opaque selectors instead of readable business labels.

The filter request must not reveal:

- owner identity;
- service identity;
- conversation identity;
- blob identity meaning;
- operation meaning.

### 4.5 Store Privacy

Use of `Waku` store must not expose readable product semantics through stored
topics or plaintext payloads.

Offline retrieval may remain possible, but a non-authorized store operator must
not be able to infer product meaning from the stored selector values alone.

### 4.6 Diagnostics Redaction

Diagnostics and local control surfaces may explain privacy mode and switching
decisions, but must not expose recoverable topic/filter secrets or other
derived capability material.

Explainability is required at the level of:

- active privacy profile;
- reason for automatic switching;
- reduced capabilities;
- recovery path.

It is not allowed to expose:

- raw private selector values;
- derivation secrets;
- decryptable routing capability material.

## 5. Allowed Carrier Shape

The product may still use:

- one or more carrier pubsub topics;
- relay, filter, store, and lightpush as `Waku` capabilities;
- adaptive participation profiles.

But these carrier-level paths must not themselves be treated as readable
product semantics.

`Waku` pubsub topic may remain a transport carrier if product meaning is moved
into opaque derived selectors and encrypted envelopes.

## 6. Adaptive Privacy Profiles

Ardents may support multiple privacy/participation profiles above `Waku`, for
example:

- default product mode;
- reduced-exposure mode;
- defense mode during suspected attack;
- constrained store/filter mode.

These profiles are allowed only if:

- they are variants of the same `Waku` foundation;
- automatic switching is explicit and operator-visible;
- switching does not silently claim capabilities that are no longer active;
- privacy reduction or exposure increase is visible as runtime truth.

## 7. Threat Model Baseline

The product must assume that a non-authorized observer may see:

- network participation itself;
- traffic timing;
- message volume;
- carrier-level routing activity;
- endpoint presence.

The privacy layer must therefore focus on hiding:

- product meaning;
- owner meaning;
- message class meaning;
- service meaning;
- blob meaning;
- filter interest meaning.

This document does not claim to make participation invisible. It requires that
product semantics remain hidden from parties that do not hold the relevant
capability material.

## 8. Implementation Frame

The expected architectural shape is:

- one product-facing transport facade;
- opaque selector derivation from owner or channel capability material;
- encrypted network envelopes;
- policy-governed participation profiles;
- diagnostics truth for profile selection and degraded privacy posture.

This layer is a `Waku` privacy/orchestration layer, not a second canonical
network foundation.

## 9. Minimum Acceptance Criteria

The privacy layer is not complete until all of the following are true:

- readable product semantics are absent from network-visible topic/filter
  values;
- authorized holders can derive and use the required selectors;
- non-authorized observers cannot interpret topic/filter meaning;
- private payloads enter `Waku` only as encrypted envelopes;
- diagnostics explain privacy mode without leaking private selector material;
- attack/degraded switching remains visible and truthful.
