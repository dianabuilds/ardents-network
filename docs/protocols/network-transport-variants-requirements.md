# Network Transport Variants Requirements

## 1. Purpose

This document fixes the product requirements for multiple transport
participation variants inside the canonical `Waku` foundation.

It defines:

- which transport-variant behavior is allowed;
- how automatic switching may work;
- what truth must remain visible to the operator;
- how transport adaptation stays separate from privacy semantics.

It does not replace:

- `docs/protocols/canonical-network-foundation.md`
- `docs/protocols/network-and-discovery-requirements.md`
- `docs/protocols/network-privacy-requirements.md`

## 2. Core Separation Rule

Transport participation and network privacy are different product concerns.

Transport participation answers:

- which underlying network transports are enabled;
- how the node joins and exposes transport endpoints;
- when the runtime should switch between transport participation modes;
- how transport degradation affects reachability and role availability.

Network privacy answers:

- what a non-authorized observer can infer from traffic;
- how selectors are hidden;
- how payloads are encrypted;
- how product meaning is obscured.

These concerns may interact, but they must not be merged into one undifferentiated
"adaptive transport" feature.

## 3. Canonical Foundation Constraint

`Waku` remains the canonical network substrate for `v1`.

Transport variants are allowed only as runtime/configuration variants of the
same `Waku`-backed participation model. They must not become a second network
foundation or a generic plugin abstraction.

## 4. Required Variant Model

The product may support multiple transport profiles, for example:

- `tcp_only`
- `tcp_quic`
- `tcp_wss`

The exact profile set may evolve, but every profile must:

- remain a `Waku`-backed runtime mode;
- preserve the required role mapping of `relay`, `store`, `filter`, and
  `lightpush` in the product model;
- make active transport capabilities explicit through diagnostics and local
  control surfaces.

For the current `v1` runtime, the recognized transport-profile set is:

- `tcp_only`
- `tcp_quic`
- `tcp_wss`

This set is intentionally narrow:

- `tcp_only` is the default implemented profile;
- `tcp_wss` is implemented only when the daemon receives a complete explicit
  WSS port, advertised address, and validated operator-managed certificate/key
  pair;
- `tcp_quic` is recognized only so explicit selection can fail closed while it
  remains unsupported;
- a separate operational mode axis may narrow participation without becoming a
  second transport profile.

The allowed operational mode set is:

- `steady`
- `restricted_defense`

`restricted_defense` is a narrowed operational mode for attack, instability, or
policy-driven surface reduction. It is valid only if:

- the active profile change is explicit and operator-visible;
- reduced participation and recovery conditions are visible through
  diagnostics/local control surfaces;
- the runtime does not falsely claim a different endpoint-family exposure than
  it actually has.

Each allowed profile must carry an explicit capability matrix that states:

- which transport families may be exposed;
- whether the profile is steady-state or expansion-oriented;
- how `relay`, `store`, `filter`, and `lightpush` remain represented in the
  product model;
- what degraded capability truth must be shown to the operator.

Each allowed operational mode must carry explicit semantics for:

- whether it is steady-state or temporary;
- what participation narrowing it enforces;
- what reduced capability truth must be shown to the operator;
- what recovery conditions or operator actions may clear it.

For `tcp_wss`, support is truthful only when the runtime has explicit secure
websocket certificate material configured for that profile.

- production/operator-facing `tcp_wss` support must not silently fall back to
  self-signed certificate generation;
- self-signed certificates are rejected by the product path; tests that need a
  successful listener use a test-CA-issued server leaf instead.

## 5. Automatic Switching

Automatic switching is allowed only if the product remains explainable.

Triggers may include:

- repeated dial failure;
- peer-collapse or route-collapse conditions;
- transport-family-specific instability;
- suspected transport-level attack or abuse pattern;
- operator-forced defense posture;
- local security or policy restriction.

Automatic switching must not:

- silently claim that the old mode is still active;
- silently widen the exposed surface without policy approval;
- hide reduced capability from the operator.

## 6. Required Runtime Truth

The operator must be able to inspect:

- active transport profile;
- allowed transport families;
- transport families currently suppressed or disabled;
- switch reason;
- whether the switch was automatic or policy-forced;
- reduced capabilities;
- recovery conditions.

Diagnostics must explain transport adaptation without exposing private selector
or key material from the privacy plane.

## 7. Product-Facing Contract

The client must not manually orchestrate raw transport-family selection for
ordinary product behavior.

The product-facing contract must stay stable while the transport controller
selects the active participation profile internally.

This means the client should continue to work through a canonical transport
owner, while transport switching happens behind that contract.

## 8. Minimum Acceptance Criteria

Transport-variant support is not complete until all of the following are true:

- at least two explicitly defined participation variants exist or the document
  explains why the second variant remains blocked;
- the runtime can report the active transport profile truthfully;
- transport-family switching is explainable and operator-visible;
- degraded transport states remain visible through diagnostics;
- tests cover both success and degraded/switch paths;
- transport adaptation remains separate from privacy adaptation.
