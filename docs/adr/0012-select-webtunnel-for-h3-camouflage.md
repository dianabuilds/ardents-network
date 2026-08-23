---
status: superseded
date: 2026-08-16
superseded_by: ADR-0024 (maintained Route applicability)
---

# Select standalone WebTunnel for the H3 Camouflage Adapter

> Superseded for maintained runtime use by
> [ADR-0024](0024-native-interactive-route-foundation.md). This record retains
> only the bounded H3 experiment decision and its provenance.

Horizon 3 Stage 5 uses standalone WebTunnel `v0.0.6` at commit
`d729fde1f38357dcefa2a751eb4752e9ca78f910` behind the replaceable,
candidate-neutral Camouflage Adapter seam defined by R-036. The exact client
binary identity is pinned by R-036. Lyrebird obfs4 is not packaged, chained, or
retained as a runtime fallback.

The precommitted two-candidate campaign exercised the same workload, DNS
boundary, resource series, three shutdown rungs, and residual checks. Both
candidates passed those structural gates. WebTunnel was selected because it
covers the required protocol-allow-list profile while using a materially
smaller candidate binary, runtime memory, state, dependency closure, and
advisory surface. Its TLS/front helper cost and higher measured fixture traffic
remain charged; R-037 must qualify them under candidate-independent thresholds.

This is an H3 controlled-network choice, not a production transport, public
distribution mechanism, availability promise, or censorship-resistance claim.
An informed probe with the Invite or an address blocker may still succeed. A
changed source or binary identity, additional fallback, public deployment, or
failure of the R-037 gates requires new research and a superseding ADR.
