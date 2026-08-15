---
id: R-038
title: What bounded role capacity contract closes Horizon 3 Stage 4?
status: accepted
owner: product research
started: 2026-08-15
reviewed: 2026-08-15
---

# R-038 — Stage 4 role capacity

## Decision this unlocks

Freeze P3-D3b4 for the infrastructure roles implemented through Stage 4 so
S4.4 can implement and test useful work, pressure, lifecycle, and scale-up
without copying Endpoint floors or waiting for the Stage 5 Bridge Adapter.

## Current contract

R-023 fixes the Ubuntu `2 vCPU`/`2 GiB`/symmetric `100 Mbit/s` reference host,
post-exclusion accounting, bounded overload, resource evidence, and a fourfold
stronger-host requirement. R-028 supplies the measured pressure-state
thresholds but explicitly did not turn the Stage 1 probe into role capacity.
R-013 measured the selected native shape at `92.38 Mbit/s`, `20/20` successful
setups, at most `5.23 MiB` ordinary-Node RSS, and at most `0.1282` mean logical
core in its controlled Ubuntu campaign. Those measurements justify a
conservative tracer floor, not a production-network claim. R-034 assigns
Bridge-specific work and capacity to Stage 5.

The Stage 4 implementation has four maintained infrastructure roles:
Initiator, Introduction, Rendezvous, and Responder. Destination Resolution,
discovery, and Bridge are not implemented roles in this stage and receive no
invented floor.

## Hypotheses

- **H1:** every maintained role can provide four concurrent authenticated Route
  Attachments on the reference class while preserving sustained forwarding,
  recovery, bounded pressure, and cleanup; the stronger profile provides 16.
- **H2:** four concurrent Attachments pass functionally but pressure or churn
  destroys established useful work.
- **H0:** the current per-Attachment process shape cannot provide useful bounded
  role capacity and must be redesigned.

## Evaluation criteria

One **role Attachment unit** is useful only when one selected role process:

1. authenticates both adjacent leg bindings under the exact Network/Epoch/Node
   duty;
2. forwards opaque bytes without receiving Target, Application, or complete
   Route knowledge;
3. returns an honest terminal observation with byte counts and cleanup; and
4. releases every connection, goroutine, timer, queue credit, and listener.

The reference workload is conjunctive:

- four concurrent Attachment units complete in one role-process lifetime;
- the same implementation sustains one active Attachment for ten minutes in
  each separate data direction at the S4.3 impaired goodput, queue, resource,
  and traffic gates;
- one eligible recovery and one abandoned/hostile attempt are charged rather
  than hidden; and
- anonymous incomplete establishment, authenticated over-capacity work, slow
  read/write, and emergency pressure produce bounded refusal or drain while
  already accepted work retains its deadline.

The effective post-exclusion floor is four Attachments on each candidate that
remains selectable after the maximum applicable identity, family, domain,
Direct Source, resolver, and drain/quarantine union. Capacity is not summed
from excluded candidates and grants no selection weight, trust, or authority.

The stronger-host profile is accepted only when the identical process and
security behavior completes 16 concurrent units, at least four times the
reference useful work. It does not add a role or change selection priority.

## Evidence plan

### Primary sources

Accessed 2026-08-15:

- R-013 for the official Ubuntu native-role measurements;
- R-023 P3-D3b3/P3-D3b4 and its pressure, direct-baseline, and scale rules;
- R-028 for resource ownership and `NORMAL/PROTECT/DRAIN/EXIT`; and
- R-032/R-034 for Stage 4 recovery and Bridge sequencing.

### Experiment

The maintained unit and process E2E suites exercise the public Route and Node
interfaces. The final live campaign runs the reference four-Attachment cell in
separate containers, both ten-minute impaired directions with bracketing
60-second direct baselines, and strict cleanup. The command E2E also runs the
16-Attachment functional scale-up cell. Only the controlled Ubuntu run may close
the reference-host evidence gate; Windows Docker is development evidence.

### Failure scenarios

- incomplete TLS/leg binding and authenticated work above the finite cap;
- admitted slow reader/writer, queue saturation, and incomplete handshake;
- PROTECT recovery, emergency DRAIN/EXIT, listener loss, cancellation, and
  terminal deadline;
- exclusion leaving insufficient capacity or selecting an excluded family;
- an established connection being evicted to manufacture capacity;
- stronger hardware changing trust, role, priority, or security behavior; and
- missing samples, zero useful work, contradictory observations, or residual
  containers, networks, volumes, sockets, and state.

## Findings

**Measurement:** the retained R-013 Ubuntu campaign provides ample headroom
above the four-Attachment tracer floor, but does not by itself qualify current
Stage 4 code.

**Measurement:** the maintained public Route process E2E completes 16
simultaneous authenticated Attachments through every Stage 4 role process while
malformed TLS setup is abandoned and the seventeenth authenticated client is
refused without reducing the declared useful-work count.

**Measurement:** Route runtime behavior tests preserve an established
Attachment in PROTECT, refuse new setup, enter emergency DRAIN/EXIT, and leave
no listener. The live suite reruns that runtime behavior inside the checked
`h3-np1-v1` cgroup profile. The independent Node lifecycle suite retains the
same bounded pressure contract for assignment admission and its private probe.

**Measurement:** the maximum supported identity/family/domain exclusion union
retains one eligible candidate of declared capacity four for every maintained
Route position.

**Measurement:** on 2026-08-15 the complete local Docker development suite
passed in `27m02.062s`. It covered four useful concurrent Attachments plus one
authenticated refusal, real measured `NORMAL/PROTECT/DRAIN/EXIT`, two separate
ten-minute impaired Service directions with paired deterministic 60-second
direct baselines, bounded resources/traffic, and empty final Compose ownership.
This Windows-Docker result closes local development evidence only; it does not
replace the controlled Ubuntu reference-host verdict.

**Inference:** four is a conservative finite development floor for the current
one-owner H3 topology. It is not a production Node capacity, public supply, or
decentralization claim.

## Options

1. Copy Endpoint `64/16` or publisher `256/64`. Rejected: those are different
   work and resource boundaries.
2. Use four reference Attachments and 16 for the stronger functional profile,
   conjoined with sustained forwarding and pressure gates. Accepted.
3. Leave capacity qualitative. Rejected: it cannot falsify admission, scale,
   or resource behavior.

## Recommendation

Accept option 2 for Horizon 3 Stage 4 development. A later production role or
protocol must replace these tracer units with evidence from its own maintained
implementation; it inherits no numerical credit.

The strongest counterargument is that four concurrent Attachments is small.
That is intentional: the stage proves bounded useful operation and fourfold
scale behavior without turning a research tracer into a public capacity claim.

## Disposition

- State: accepted under the Product Owner's 2026-08-15 instruction to complete
  all Stage 4 development.
- S4.4 implementation and final controlled evidence are authorized.
- Bridge-specific capacity remains Stage 5 work under R-034.
- No new runtime dependency or public performance/privacy claim is authorized.
