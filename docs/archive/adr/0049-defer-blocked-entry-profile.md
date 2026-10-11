---
status: accepted
date: 2026-08-27
---

# ADR-0049 — Do not select a blocked-entry profile for the functional alpha

## Context

H4-2C considered obfs4-like Bridges, WebTunnel-style entry, MASQUE proxies, and
WebRTC/Snowflake-like brokered entry. Each candidate needs a concrete blocked
access condition plus an acquisition, distribution, probing, exposure, abuse,
operation, update, and withdrawal model. The current project has no such
bounded condition or operator infrastructure. QUIC is a second authenticated
Carrier, not camouflage.

## Decision

The functional alpha supports no Bridge, proxy, broker, fronting, or camouflage
Entry profile. An unavailable supported Carrier fails explicitly. An Adapter
never discovers or tries an alternate profile, public proxy, or direct path.

A future blocked-entry profile requires a new research question and superseding
ADR that names one censorship condition, every contacted source/operator, the
finite exposure and retry budget, active-probe behavior, lifecycle ownership,
and reproducible qualification evidence.

## Consequences

- H4-2C is closed by an explicit negative selection, not by placeholder code.
- No hidden infrastructure or H3 compatibility mechanism enters the alpha.
- H4-2 makes no censorship-resistance claim.

## Compliance

- [R-094](../research/records/r-094-hostile-network-carrier-profiles.md) records
  the evaluated families and stop conditions.
- [ADR-0048](0048-maintain-tcp-and-quic-carriers.md) remains the maintained
  adjacent-Node Carrier decision.
