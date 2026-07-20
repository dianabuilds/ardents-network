# Network Transport Variants Architecture

## 1. Purpose

This document defines the minimum product-grade architecture for transport
participation variants above the canonical `Waku` foundation.

It is the transport-side counterpart to:

- `docs/network-transport-variants-requirements.md`
- `docs/network-privacy-requirements.md`
- `docs/network-privacy-architecture.md`

## 2. Architectural Rule

Transport-variant logic must be owned by the canonical transport domain.

It must not be implemented as:

- a competing runtime facade;
- a client-driven matrix of raw transport toggles;
- a generic backend/plugin system detached from `Waku`.

## 3. Canonical Internal Components

### 3.1 Transport Facade

The canonical owner of transport participation.

Responsibilities:

- expose one stable product-facing transport contract;
- delegate transport-profile selection internally;
- keep runtime truth synchronized with diagnostics and local control surfaces.

### 3.2 Transport Profile

Represents an allowed participation variant.

A profile defines:

- allowed underlying transport families;
- endpoint exposure shape;
- participation restrictions;
- role-impact expectations;
- downgrade and recovery semantics.

### 3.3 Transport Operational Mode

Represents the active defensive or recovery posture above the active transport
profile.

An operational mode defines:

- whether participation is in steady state or narrowed defense posture;
- what capability reductions must be surfaced;
- what recovery conditions apply before returning to steady participation.

### 3.4 Transport Profile Controller

Chooses the active transport profile and transport operational mode.

Responsibilities:

- evaluate transport-health and policy signals;
- keep the active transport profile stable unless a profile-level change is
  required;
- activate the next allowed operational mode when the runtime must narrow
  participation;
- avoid unsafe widening of exposed transport surface;
- emit switch reasons and reduced capability truth.

### 3.5 Transport Health Evaluator

Classifies the health of the active transport path.

Expected inputs:

- dial results;
- peer continuity;
- endpoint readiness;
- bootstrap outcomes;
- transport-family-specific failures;
- policy/security signals.

### 3.6 Transport Snapshot

Provides operator-visible truth.

It must expose:

- active profile;
- active operational mode;
- active endpoint families;
- suppressed endpoint families;
- switch reason;
- health summary;
- reduced capabilities;
- recovery state.

## 4. Current Code Baseline

The current root code has one canonical transport owner:

- `internal/network/transport` behind `internal/network/api.Service`.

The current startup and bootstrap path is constrained:

- `internal/network/transport/startup.go` uses `libp2p.NoTransports` plus explicit TCP
  transport;
- `tcp_only` and `tcp_wss` runtime shapes are explicit; `tcp_quic` is rejected;
- node roles are separately modeled by
  `docs/network-participation-profiles.md` and invalid combinations fail before
  lifecycle startup;
- `service_node` can combine explicit static peers with Waku's signed DNS ENR
  tree retrieval; results remain bounded, transport-compatible, non-persistent,
  and are periodically replaced rather than accumulated as stale trust;
- the runtime replenishes toward three live Relay peers and reports source
  discovery, dial, and Relay-readiness failures separately.

This means the recommended implementation path is:

1. retain the completed fail-fast secure WSS operator boundary;
2. retain signed DNS/static peer replenishment and add reachability observation;
3. implement constrained Filter/Lightpush product paths before enabling the
   constrained-light-client node profile.

The WSS boundary is owned by `ardd` configuration and the canonical transport
validator. It accepts only an explicit port, certificate/key pair, and
certificate-covered advertised host. The listener binds locally, while only
the WSS endpoint host is rewritten for publication. Certificate rotation is a
deployment-managed replacement followed by controlled restart, not live reload.

Peer Exchange and Discv5 are intentionally outside the current enabled shape.
The dependency and security decision is recorded in
`docs/process/v1-stabilization-hardening/stb-303-dependency-review.md`.

Reachability is a separate runtime axis from Relay participation. The transport
owner exposes local/LAN, outbound-only, and verified direct-public modes. Direct
public addresses are explicit operator inputs, filtered to the active carrier,
and withheld until AutoNAT reports public dialability. A later private/unknown
result withdraws them. Publication consumes this gated endpoint set and cannot
derive public ingress from a bound socket or an outbound peer connection.

The mechanism decision, including rejection of implicit port mapping and the
current non-support of Circuit Relay/hole-punch/browser paths, is recorded in
`docs/process/v1-stabilization-hardening/stb-304-dependency-review.md`.

## 5. Switching Model

The expected switching flow is:

1. Transport health deteriorates or policy demands a different posture.
2. The controller decides whether this is a profile change or an operational
   mode change.
3. The runtime applies a profile change through controlled restart/rebind or
   applies a mode change as a participation-state transition above the current
   profile.
4. Diagnostics surface the active profile, active mode, switch reason, and
   reduced capabilities.
5. Recovery logic decides whether and when the runtime can return to a broader
   profile or clear the narrowed mode.

If two profiles currently map to the same truthful endpoint-family set, the
transition flow may initially apply as a participation-state transition rather
than a transport-family rebind. In that case the operator-visible profile,
degraded state, reduced capabilities, and recovery conditions are still
mandatory.

## 6. Required Separation From Privacy

Transport profile and privacy profile must remain separate state axes.

Examples:

- `QUIC` failure may force `tcp_only` without changing opaque selector
  derivation.
- privacy defense mode may tighten selector/padding behavior without changing
  the active transport family.
- policy may intentionally switch both, but each change must remain visible as
  a separate runtime fact.

## 7. Minimum Code Shape

When implemented in code, the expected ownership is:

- `internal/transport`: transport profiles, controller, health evaluator, and
  transport snapshot;
- `domain/diagnostics` and `application/diagnostics`: transport adaptation explainability;
- `internal/node`: orchestration only where restart/rebind coordination is
  required.

The client must continue to use one canonical transport-facing owner.
