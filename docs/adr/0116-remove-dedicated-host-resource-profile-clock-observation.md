---
status: accepted
date: 2026-09-28
completes: ADR-0114 (removal scope: the retained dedicated-host placement identity and its clock-observation trigger)
---

# ADR-0116 — The dedicated-host resource profile and the Node clock-observation trigger are removed as production-unreachable

## Context

ADR-0114 removed the dedicated-host Contributor retirement mechanism but
retained two symbols because live code still referenced them: the
`ardents-rendezvous-dedicated-host-v1` resource-placement identity in
`internal/resource` (aliased by `internal/node` as
`RendezvousDedicatedHostResourceProfile`) and the joined installed
clock observation (`StartContributorClockObservation`), which the Node
command started only for that profile. A package survey of
`internal/resource` on 2026-09-28 proved the whole chain is
production-unreachable on current `dev`:

- Closed-duty Node plans refuse any `node_resource_profile` with a typed
  error before further effects ("native Route Node resource profile is
  unselected"), pinned by the closed-plan behavior tests.
- Every old native duty (`rendezvous`, `initiator`, `introduction`,
  `responder`, `transit_issuer`) is a typed retirement refusal
  (`errOldNodeDutyRetired`) raised before resource-profile validation.
- The plan profile that does flow into Node admission is accepted only as
  `h3-np1-v1`, `h3-s-v1`, or `h3-s-v1-strong`; the dedicated-host identity
  receives "node resource profile is not supported", and a plan without any
  listener receives "node needs one local listener profile". No runnable
  `node.Run` configuration can therefore carry the profile.
- State accepts only `h3-s-v1` as its source-mode runtime profile, so the
  identity cannot enter through State either.
- Consequently the only remaining production reference — the Node-plan
  branch mapping the dedicated-host profile to a clock-observation trigger —
  is reachable solely by plans that then immediately fail admission. Its one
  surviving effect was a transient clock-file touch inside a doomed run.
- The mechanism's original owners, the dedicated-host Contributor
  installations, were confirmed absent by the Product Owner decision behind
  ADR-0114; no installation remains to observe the clock marker.

A read-only Node cleanup note had classified the `internal/node` alias as a
"live command-facing compatibility interface" because the command reference
exists. That classification is superseded by the reachability proof above:
the referencing branch itself is dead. The standing Product Owner decision
that legacy dies without backward compatibility applies, and the Product
Owner directed removing the proven-dead chain.

## Decision

Remove the production-dead chain in one slice:

- `internal/resource`: the `RendezvousDedicatedHostProfile` constant and its
  `profiles` map entry. `resource.New` now refuses the identity exactly like
  the historical `h4-5-rendezvous-alpha-v1` planning identity, and the
  legacy-refusal behavior test pins both retired identities.
- `internal/node`: the `RendezvousDedicatedHostResourceProfile` alias and
  the whole clock-observation mechanism
  (`StartContributorClockObservation`, `ContributorClockObservationInterval`,
  their unexported core, and their tests).
- `cmd/ardents-node`: the `nodeRuntime.clockObservation` field, the
  dedicated-host branch in the plan reader, and the clock-observation
  start/stop joins in the Node run loop.

Explicitly retained:

- The `node_resource_profile` plan field, the closed-plan refusal of any
  profile value, and the `h3-*` Node/State profiles — all live.
- The State-side `ClockObservationFile` input (`state.Config` validation and
  mtime observation) — an independent live mechanism consumed by the
  network-source and qualification flows, which own their file touching in
  fixtures and operator setups.
- The typed retirement refusals for old duties and the behavior tests that
  pin retired plan shapes (including the dedicated-host profile string as
  historical refusal data).

## Consequences

- `h3-np1-v1`, `h3-s-v1`, and `h3-s-v1-strong` are the only instantiable
  resource-guard identities; the dedicated-host identity joins the refused
  legacy identities at the single `resource.New` gate.
- Boundary note: after this removal no in-tree production code touches a
  clock-observation file; State's mtime observation remains contract-live
  and is exercised by the network-source E2E and qualification fixtures.
  Any future production toucher is a new decision, not a restoration of
  this mechanism.
- Boundary note: the dedicated-host profile was the only entry declaring
  storage ceilings (`maximumStorageBytes`/`Files`/`Directories`/`Depth`).
  Bounded storage measurement stays live (Sample fields, the
  `ardents-h3-resource-sample-v1` telemetry, and its measurement tests);
  the generic ceiling comparison in the monitor remains as an inert
  capability that no current profile arms. Arming or removing it is left to
  a future resource-policy decision.
- Historical records (R-092, R-155, ADR-0114, reconstruction findings, and
  the transition/scope/command-surface statements about ADR-0114) are
  untouched; living documents (`package-map.md`,
  `network-route-node.md`) are updated in this slice.
