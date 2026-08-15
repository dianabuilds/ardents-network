---
id: R-033
title: What is the exact Horizon 3 Stage 5 scope and decision order?
status: decided
owner: product research
started: 2026-08-14
reviewed: 2026-08-15
---

# R-033 — Horizon 3 Stage 5 scope

## Decision this unlocks

Freeze the smallest Horizon 3 Stage 5 research scope that can later authorize
one maintained Bridge and blocked-entry vertical slice without importing
unfinished Stage 4 assumptions or expanding into public Bridge distribution,
general routing, or a stronger traffic-analysis profile.

This record authorizes documentation research only. It authorizes no Stage 5
implementation brief, package, dependency, binary, protocol, experiment, or
claim.

## Current contract

The following inputs are authoritative:

- [Horizon 3 scope](../../product/scope.md#horizon-3--closed-test-network);
- [J-06 degraded/blocked-path journey](../../product/journeys.md#j-06--continue-through-degradation-or-recover-from-a-failed-path);
- [Horizon 3 technical design](../../development/horizon-3-technical-design.md);
- [R-009 Bridge architecture](r-009-hostile-bootstrap-and-bridge-entry.md);
- [ADR-0005](../../adr/0005-route-domains-and-bounded-entry-exposure.md);
- [operating model](../../product/operating-model.md);
- [threat model](../../security/threat-model.md#bridge-entry);
- [R-023 resource and qualification contract](r-023-interactive-route-performance-budget.md);
- [R-032 Stage 4 recovery decision](r-032-h3-same-connection-recovery.md); and
- [Stage 4 implementation brief](../../development/horizon-3-stage-4-brief.md).

The canonical glossary definitions are
[Entry Set](../../../CONTEXT.md#entry-set),
[Bridge](../../../CONTEXT.md#bridge),
[Transport Camouflage](../../../CONTEXT.md#transport-camouflage),
[Role Domain](../../../CONTEXT.md#role-domain), and
[Work Safety Lease](../../../CONTEXT.md#work-safety-lease).

As of 2026-08-15, Stage 4 development and local container evidence are complete;
official controlled-Ubuntu qualification remains evidence-gated. R-034 is
accepted and assigns Bridge-specific capacity to Stage 5. Stage 4 measurements
are not Stage 5 evidence, and no Stage 5 implementation is authorized.

R-009 and ADR-0005 already fix these invariants:

- ordinary and Bridge entry are separate regimes for the same
  endpoint-adjacent Route role;
- one Bridge identity is eligible for exactly one Initiator, Responder, or
  Introduction Role Domain during its assignment lifetime;
- an Invite changes only the installation's finite Bridge Entry Set for that
  domain and cannot create another set;
- the Bridge sees the adjacent Endpoint IP and traffic pattern but receives no
  Service Name, Service Target, opposite endpoint, full Route, or Application
  Data;
- Bridge identity and known family cannot simultaneously enter conflicting
  Route or Destination Resolution work;
- regime changes, contacts, retries, exposure, and retained history are finite;
- blocked Bridge entry never becomes a direct, DNS-selected, shorter, or weaker
  fallback; and
- an Endpoint with no reachable known address and no out-of-band information
  reports `blocked`.

The exact H3 state machine, Invite representation, Adapter interface, candidate
selection, evidence matrix, and blocked-profile budgets remain open.

## Exact Stage 5 scope

### Input

The Stage 5 fixture starts with:

- the authenticated project-controlled network state and exact Target already
  exercised by the preceding H3 stages;
- the same Route, Service Connection, Application Interface, Work Safety, and
  resource contracts;
- one synthetic, authenticated, expiring Bridge Invite imported from a local
  file or equivalent offline fixture; and
- a controlled network that can block ordinary entry, classify or allow-list a
  carrier, and issue active probes.

Stage 5 does not acquire an Invite from a public service. File import is the H3
input seam, not evidence that a real censored user can obtain an Invite.

### Behavior

The maintained slice may add only:

1. validation, expiry, replacement, restart, and rejection of the H3 Bridge
   Invite;
2. one finite Bridge Entry Set for one adjacent Role Domain selected by R-035,
   plus wrong-domain and conflicting-role negatives;
3. one explicit owner-approved or precommitted bounded policy transition from
   ordinary to Bridge entry, with finite contacts, retries, and deadlines;
4. one replaceable Camouflage Adapter seam below the Route; R-036 exercises the
   same seam with exactly two feasibility candidates before selecting exactly
   one for the maintained slice; and
5. deterministic evidence for blocking, probing, withholding, replay, restart,
   resources, forbidden fallback, and cleanup.

The Route Module remains responsible for authenticated Route selection and
receives only an endpoint-adjacent Carrier Channel or an explicit classified
failure. Bridge state owns Invite validity, Entry Set membership, regime,
exposure, contact order, and exhaustion. A Camouflage Adapter owns only the
candidate-specific process/configuration lifecycle and opaque bidirectional
byte carriage to one selected Bridge endpoint. It does not select a Bridge,
choose a Role Domain, rotate candidates, reset deadlines, inspect the Target,
or own Route recovery.

This is the intended deep seam: Route and Service Connection callers do not
learn candidate-specific configuration, while transport replacement does not
change Bridge product state.

### Observable outcome

Under each declared controlled censor profile applicable to the selected
Adapter, the Endpoint either:

- reaches the same authenticated network and exact Target through the unchanged
  Route and Application-visible byte contract; or
- returns the explicit classified terminal result frozen by R-035 within the
  accepted finite bound.

A pass requires unchanged Target/Instance/context/profile authentication,
unchanged Route length and role separation, bounded exposure and resources, no
forbidden fallback, and complete cleanup.

### Explicitly outside Stage 5

Stage 5 does not include:

- public, anonymous, automatic, brokered, email, chat, CAPTCHA, token, or account
  based Bridge distribution;
- DHT, peer exchange, general relay discovery, topology optimization, mesh or
  source routing, or changes to Candidate View construction;
- camouflage for every relay-to-relay Carrier leg;
- automatic cycling across transport families inside one connection attempt;
- padding, mixing, multipath, cover traffic, or a Shielded Route Profile;
- NAT traversal, volunteer brokers, cooperating ISP/refraction infrastructure,
  or real censored-region deployment;
- a new cryptographic primitive or first-party camouflage protocol;
- Windows, installer, public UX, production operations, external audit, or
  public censorship-resistance, anonymity, availability, independence,
  invisibility, or indistinguishability claims; or
- any change to Service Names, Namespace, application semantics, credentials,
  Service migration, update/release, or governance.

## Stage 4 dependency audit

This record identified one real sequencing cycle:

1. R-023 P3-D3b4 and the Stage 4 brief require measurable Bridge work before
   role-specific capacity and S4.4 can close;
2. the H3 sequence places the maintained Bridge and camouflage slice in Stage 5;
3. synthetic Bridge capacity cannot qualify a later selected Adapter, while
   requiring the maintained Adapter would start Stage 5 before Stage 4 closes.

R-034 evaluated only this ownership question. The clean candidates were:

- Stage 4 uses a disposable protocol-neutral Bridge workload solely as an input
  to its general role-capacity decision, with no claim about a later Adapter; or
- Bridge-specific useful-work and capacity acceptance move to Stage 5, while
  S4.4 closes only the roles implemented through Stage 4.

R-034 accepted the second option: S4.4 closes only the roles implemented through
Stage 4, and Stage 5 owns Bridge-specific useful-work and capacity acceptance.
That removes the ordering cycle but grants no Stage 5 implementation authority.

## Hypotheses

- **H1:** the scope above is sufficient when capacity sequencing, Bridge state,
  Adapter selection, and the integrated evidence contract are decided
  separately and then joined by one implementation gate.
- **H2:** one combined decision can freeze all four without circular evidence or
  candidate-specific product state.
- **H0:** no maintained Adapter can carry the bounded Stage 5 workload within the
  accepted security, resource, supply, and one-to-one maintenance constraints.

## Evaluation criteria

The Stage 5 decomposition passes only if it:

- gives the Endpoint exactly two observable outcomes: the same authenticated
  network/Target and unchanged Application byte contract, or one existing
  classified terminal result;
- states protected information, censor/Bridge/probe adversaries, required
  honest endpoint/Route conditions, falsification measurements, and visible
  address/timing/volume limitations in the subordinate decisions;
- preserves every accepted Route, Target, Instance, Isolation Context, Work
  Safety, Application Interface, and exposure invariant;
- resolves the Stage 4 capacity cycle explicitly;
- keeps Bridge state independent of candidate-specific Adapter configuration;
- exercises exactly two materially different initial candidates against the
  same Adapter interface and workload before selecting maintained behavior;
- precommits censor profiles, budgets, observers, evidence, and verdict rules;
- precommits finite latency, bandwidth, CPU, memory, process, socket, disk,
  state-retention, useful-work, and availability thresholds in R-036/R-037
  before an experiment rather than inheriting R-023 startup numbers;
- requires no public DNS lookup or public Invite-distribution service at the
  Endpoint;
- fits one Product Owner and Codex without assumed operators, auditors, or
  censored-region testers;
- compares maturity, audits/advisories, misuse resistance, privilege,
  maintenance, license, offline supply, removal cost, and developer operation
  for exactly the two R-036 candidates; and
- requires only strict offline/file fixture import in H3, while recording public
  acquisition, installation, accessibility, and novice UX as later gates; and
- stops rather than expanding into an excluded subsystem when the slice cannot
  pass.

## Evidence plan

### Primary sources

Accessed 2026-08-14:

- [Tor Pluggable Transport specification](https://spec.torproject.org/pt-spec/)
  for a replaceable transport subprocess seam;
- [PT configuration environment](https://spec.torproject.org/pt-spec/configuration-environment.html)
  for bounded state, startup, forwarding, status, and shutdown behavior;
- [obfs4 protocol specification](https://github.com/Yawning/obfs4/blob/master/doc/obfs4-spec.txt)
  for the random-looking authenticated candidate and its probing limitations;
- [Tor WebTunnel description](https://blog.torproject.org/introducing-webtunnel-evading-censorship-by-hiding-in-plain-sight/)
  and [deployment guide](https://community.torproject.org/relay/setup/webtunnel/)
  for the HTTPS/WebSocket-shaped candidate and its domain, TLS, web-server, and
  operational requirements; and
- [Cure53 2024 Tor circumvention audit](https://blog.torproject.org/code-audit-censorship-circumvention-tools/TTP-03-report.pdf)
  as implementation-maturity evidence, not an Ardents security proof.

### Experiments

R-033 by itself authorizes no experiment. On 2026-08-15 the Product Owner
accepted this record and separately authorized the narrow R-036 ordering
exception: one disposable Adapter conformance/feasibility harness may exercise
the two pinned artifacts after R-036's falsification rules and immutable supply
inputs are fixed, before the final R-036 candidate selection. R-037 still owns
the later integrated Stage 5 campaign.

Generated keys, Invites, captures, candidate state, evidence, and build outputs
remain outside Git.

### Failure scenarios

The later evidence contract must cover at least:

- malformed, tampered, expired, future, wrong-network, wrong-domain, replayed,
  conflicting, and Entry-Set-expanding Invites;
- ordinary address/signature blocking and the declared protocol allow-list;
- unauthenticated and informed active probing, replay, partial handshake, and
  slow handshake;
- withholding, accept-then-stall, malformed carriage, crash, restart, resource
  exhaustion, and shutdown refusal;
- regime oscillation, retry/deadline reset, exposure reset, and cross-domain or
  conflicting-role reuse;
- every attempted direct, DNS, proxy, ordinary-entry, shorter-Route, or weaker
  fallback; and
- secret-bearing evidence or residual listeners, processes, files, queues,
  timers, and sockets after cleanup.

## Findings

### Finding 1 — the accepted product contract already bounds the stage

**Sourced fact:** R-009 and ADR-0005 define Bridge entry as a replaceable way to
reach the same endpoint-adjacent role, not a new routing system or trusted proxy.

**Inference:** Stage 5 needs no broader network-mechanism survey. Such systems
may remain reference material elsewhere, but cannot add work to this stage.

### Finding 2 — Bridge state and camouflage are different Modules

**Sourced fact:** the Tor PT specification defines transport startup,
forwarding, status, and shutdown, but not Ardents Role Domains, Entry Sets,
exposure, Target authentication, or Route recovery.

**Inference:** the Adapter seam belongs below Bridge selection and Route logic.
Making candidate configuration part of the Route interface would create a
shallow pass-through and spread transport knowledge across callers.

### Finding 3 — H3 imports an Invite but does not distribute one

**Sourced fact:** R-009 permits offline/file transfer and states that an Endpoint
cannot discover an unknown secret Bridge when every known path is blocked.

**Inference:** public acquisition is a later product problem. Stage 5 needs only
authenticated fixture import, bounded use, and honest `blocked` behavior.

### Finding 4 — two initial candidates test the intended seam

**Sourced fact:** obfs4 and WebTunnel target materially different censor
profiles: random-looking authenticated transport versus HTTPS/WebSocket-shaped
transport with additional web infrastructure.

**Inference:** compare these two only. Run them as separate declared campaign
configurations, not an automatic fallback chain. Selecting, adding, or surveying
more families is outside R-033.

### Finding 5 — the Stage 4 capacity cycle is documentary, not implementation permission

**Sourced fact:** P3-D3b4 names Bridge work as a prerequisite, while the H3
technical design places the maintained Bridge in Stage 5.

**Inference:** R-034 had to assign that work to one stage before either S4.4 or a
Stage 5 implementation brief could be accepted. Its accepted assignment removes
the cycle without qualifying a Bridge or Adapter.

## Required independent questions

| ID | Decision | Earliest effect |
|---|---|---|
| R-034 | Accepted: Bridge-specific useful-work and capacity move to Stage 5; S4.4 covers Stage 4 roles. | Satisfied for sequencing only; supplies no Stage 5 implementation permission. |
| R-035 | Freeze the transport-neutral H3 Invite, Entry Set, exposure, regime, contact, restart, expiry, replacement, and result state machine. | Before a Stage 5 brief. |
| R-036 | Freeze the Adapter interface and compare pinned obfs4/WebTunnel artifacts, supply, licenses, advisories, resource behavior, and conformance. | Before a dependency, binary, or Adapter implementation is added. |
| R-037 | Freeze the controlled blocked-entry/probing campaign, exact budgets, observers, evidence schema, and `pass|fail|invalid` verifier. | Before integrated Stage 5 code or evidence. |

## Options

| Option | Product/security and evidence fit | Resources, operations, supply, and accessibility |
|---|---|---|
| A — combined decision | Risks candidate-specific product state and post-hoc evidence; protected outcome is harder to audit. | Smaller document count but mixes budgets, maintenance, privilege, license, supply, and UX decisions into one review. |
| B — four bounded decisions | Preserves the same authenticated outcome and lets state, Adapter, and hostile evidence falsify independently. | R-034 fixes ownership; R-035 fixes finite state; R-036 owns resource/supply/maturity/license; R-037 owns clocks/verdicts. Fits the one-to-one team and offline H3 fixture. |
| C — broad anti-censorship research | Cannot support a bounded H3 claim because routing, distribution, traffic analysis, and real deployment have different adversaries. | Requires undeclared operators, infrastructure, maintenance, public UX, and evidence unavailable to the current team. |

### Option A — one combined Stage 5 decision

Rejected as the recommendation. It lets the selected transport shape its own
Invite, retry policy, resource unit, and qualification profile.

### Option B — four bounded decisions and one integration gate

Recommended. R-034 resolves the predecessor cycle; R-035 owns product state;
R-036 owns the replaceable Adapter seam and supply; R-037 owns the evidence
contract. A later Stage 5 decision may authorize one maintained vertical slice
only after all applicable gates are accepted.

### Option C — broaden Stage 5 into general anti-censorship research

Rejected. Public distribution, additional transport families, mesh/routing,
traffic-analysis resistance, and real-world deployment each introduce separate
trust, infrastructure, performance, and evidence questions.

## Recommendation

Choose Option B with high confidence and freeze the exact scope in this record.
R-034 through R-036 are accepted. Continue with R-037. Do not write an
implementation brief until every applicable predecessor decision is accepted.
R-033 and R-035 were accepted on 2026-08-15, and the completed authorized R-036
comparison selected standalone WebTunnel on 2026-08-16.

The strongest counterargument is the process cost of four records. The split is
still narrower than a combined record because it prevents candidate technology,
public distribution, routing, and stronger privacy work from entering Stage 5.

## Disposition

- State: `decided`; the Product Owner accepted the recommendation on 2026-08-15.
- R-034 through R-036 are accepted; the remaining research follow-up is R-037.
- Stage 4 development/local evidence is complete; its official controlled-
  Ubuntu evidence gate and all public claims remain unchanged.
- No accepted ADR or R-023 decision is modified by this decided record.
- R-036 selected standalone WebTunnel behind the replaceable Adapter seam; no
  implementation brief, dependency, maintained package/binary, integrated code,
  or public claim is authorized before R-037 is decided.
