# Product scope and delivery boundaries

Status: **accepted scope control**

Accepted: 2026-08-08

This document is authoritative for **what may be built now**. The vision,
functional map, threat model, operating model, research records, and ADRs may
describe later public-network obligations; they are not an undifferentiated
implementation backlog.

## Audit verdict

The product goal did not become incoherent, but the delivery scope did expand
too far. Product invariants, the first tracer, public-beta launch controls, and
stable-network qualification were all described as one `V1`. That accidentally
made permissionless naming, hostile bootstrap, Bridges, multiparty governance,
reproducible releases, two desktop platforms, general Application isolation,
and a large performance matrix appear to be prerequisites for the next line of
research code.

That interpretation is rejected. Ardents now has one Product Core and four
explicit delivery horizons. A
decision can be fixed for a later horizon without entering the current backlog.
The horizons are conditional research gates, not a committed roadmap: passing
one does not oblige the one-to-one team to start the next.
The unqualified term `V1` is retired from current planning; older records using
it mean the first public product contract, not the next prototype.

## Product Core — not a delivery horizon

The Product Core is the smallest durable promise that defines Ardents:

- a local client Application opens a connection to a location-independent
  Service Target backed by one active local Service Instance;
- the exact Target and current Instance are authenticated, and Application Data
  is confidential and integrity-protected end to end;
- a low-latency multi-hop Route aims to hide each endpoint's ordinary location
  from the opposite endpoint and to keep the direct origin-to-destination/full-
  Route binding out of one ordinary Node's permitted role-local view;
- Applications exchange a live reliable ordered byte stream through a local
  Application Interface and own every higher-level data meaning;
- failures, unavailable destinations, resource exhaustion, and unsupported
  security profiles are explicit; there is no direct-network or weaker-profile
  fallback;
- the network does not index an exact human-readable Service Name, but anyone
  already knowing it may attempt to connect; a machine Target Link remains a
  complete naming-independent path;
- independently operated infrastructure is required before Ardents may claim a
  decentralized public network.

The Product Core fixes information-flow and responsibility boundaries. ADR-0009
selects Go as the maintained project foundation, but dependencies, wire format,
cryptographic suite, discovery mechanism, route implementation, and operating
model remain separate decisions.

The five-position split-circuit Route with a separate Introduction is the first
Carrier Lab candidate. Four Role Domains, Candidate View, and source-exclusion
are conditional designs for enforcing the same promise in a later public
network; they enter experiments only if the data-path candidate survives and
only as separately promoted slices. Neither set is an irrevocable product
feature. A maintained alternative may replace them only if it satisfies the
same claim and evidence contract without weakening the Application Interface.

## Horizon 1 — Completed Research Slice: Carrier Lab

The **Carrier Lab** was the first authorized implementation slice and completed
Gate B with `advance`. It remains maintained in the root project for regression
evidence, but is not a release or production network.

It contains only:

1. Ubuntu LTS `x86-64` controlled client and publisher fixtures;
2. one deterministic Application byte-stream workload and one active Service
   Instance;
3. ephemeral project-owned test keys and a preconfigured authenticated Target/
   reachability fixture;
4. a fixed synthetic topology with the current five-position split-circuit
   candidate and separate Introduction, plus only the comparison controls needed
   to falsify that candidate;
5. exact Target/Instance authentication, end-to-end confidentiality and
   integrity, fresh connection keys, and explicit failure;
6. per-role traffic/state inspection proving what each position can and cannot
   learn;
7. coarse setup-latency, one-stream goodput, memory, CPU, and one injected path-
   failure measurements sufficient to decide whether further work is rational.

Carrier Lab deliberately does **not** implement:

- Service Names, Namespace allocation, Name recovery, or private naming;
- public discovery, permissionless Node admission, operator incentives, or Sybil
  defense;
- Network Epoch governance, threshold ceremonies, transparency auditors, or
  independent builders;
- hostile first-install bootstrap, Bridges, pluggable camouflage, or censorship
  UX;
- production installers, automatic updates, Authority Recovery Bundles,
  uninstall/purge workflows, or long protocol migrations;
- Windows support, the full cross-platform matrix, `64/256` concurrency, or the
  complete R-023 qualification suite;
- a public SDK, generic proxy, browser, Application sandbox, or malicious-
  sibling Application isolation;
- a public anonymity, decentralization, availability, or production-security
  claim.

The harness Applications have no ordinary network access by construction, but
this is a controlled-test condition, not a reusable Application isolation
product.

### Carrier Lab lifecycle boundary

| Area | In Carrier Lab now | Explicitly later |
|---|---|---|
| Installation | Developer-run Ubuntu fixture; setup may be scripted only for repeatability. | Supported package, unprivileged installer, repair, recovery, uninstall, or end-user UX. |
| Warm-up and readiness | Preconfigured authenticated test state and a fixed synthetic participant set; report whether the exact lab path is ready. | Hostile first-install bootstrap, peer expansion, Bridge acquisition, public readiness, or stale/fork recovery. |
| Operation | One active Instance, one ordered byte stream, role-local inspection, explicit terminal results, and one injected path failure. | General route optimization, multipath, public diagnostics, arbitrary Applications, retained delivery, or operational support tooling. |
| Security | Exact Target/Instance authentication, end-to-end payload protection, fresh connection keys, fail-closed behavior, and inspection of the permitted view at each synthetic role. | Production key custody, public admission, governance, update security, platform sandboxing, or a claim against adversaries not exercised by the lab. |
| Performance | Coarse comparable setup latency, one-stream goodput, CPU, RSS, and failure-reaction observations with stop conditions. | Product SLOs, capacity floors, long soak/load cells, cross-platform qualification, or the complete R-023 evidence bundle. |
| Updates | Replace and rerun the disposable experiment from declared inputs. | In-place update, rollback, protocol transition, reproducible release, or long-lived state migration. |
| Privacy | Attempt to falsify the candidate's role-local knowledge separation and record observed traffic/state. | A public anonymity, unlinkability, decentralization, censorship-resistance, or Broad Traffic Observer claim. |

Carrier Lab stops when it can answer one question: **is the current route
candidate plausibly capable of the Product Core's knowledge separation and a
useful live stream on modest hardware?** Failure reopens route design before
naming, governance, packaging, or public-network work begins.

## Horizon 2 — Conditional Reference Application: Named Unlisted Site

The first Reference Application is **Named Unlisted Site**. Carrier Lab produced
a viable route candidate, and official Gate C run `gatec-31464163490-1`
completed this controlled Ubuntu slice with `advance`.

Its first controlled slice adds only:

- the stable local Application Interface and Service Target/Instance Credential
  lifecycle;
- private reachability resolution;
- one pre-provisioned exact Service Name and private resolution binding to the
  Target;
- a deterministic local HTTP Service and single-response client;
- ordinary one-Instance host migration and explicit offline/failure behavior;
- Ubuntu-to-Ubuntu first, followed by Windows cells only after the contract is
  stable.

It does not yet require permissionless name claiming, leases, delegation,
catastrophe recovery, anti-abuse allocation, general browser support, public
Contributor operation, or production release governance. Those mechanisms may
be prototyped separately, but do not join shared product code merely because the
full public contract eventually needs them.

Gate C completion means only that the closed tracer worked: 20/20 positives,
17/17 required failures, and 5/5 migrations passed with complete cleanup. It
does not promote the laboratory Route/OHTTP protocol, qualify a privacy claim,
or satisfy public naming, platform, operator, or release gates.

## Horizon 3 — Closed Test Network

The Named Unlisted Site slice now works in its bounded Gate C environment, so
Horizon 3 is the next permitted scope. Ardents may assemble one separately
scoped persistent multi-host test-network slice at a time. It remains
project-key-controlled, invite-only or
otherwise bounded, visibly centralized, and unable to make a public anonymity or
decentralization claim.

This horizon may add one vertical slice at a time:

- authenticated epoch/bootstrap distribution and finite source handling;
- public-role process separation, bounded Node admission, and withdrawal;
- Bridge and transport-camouflage experiments;
- permissionless Namespace, lease, recovery, and private-resolution candidates;
- install/update/rollback and authority-recovery prototypes;
- Windows compatibility plus brokered Application-isolation experiments;
- broader recovery, overload, and role-capacity measurements.

Each slice has its own stop condition. Passing the Named tracer does not
authorize building all of them in parallel, and a project-operated test network
is not evidence of independent operation.

No H3 implementation starts merely because this horizon is open. A research
record must first freeze the exact vertical slice, protected claim, finite
resources, evidence, falsification conditions, and exclusions, and the Product
Owner must explicitly mark that record `decided`. The
[Horizon 3 technical design](../development/horizon-3-technical-design.md) maps
the complete horizon and its sequential outcomes.
  [R-029](../research/records/r-029-h3-authenticated-node-lifecycle.md) is the
decided and authorized Stage 1: authenticated Network State must control a real
Node lifecycle. R-027 and R-028 are accepted only as its detailed bootstrap and
resource/evidence appendices, not standalone implementation slices. This
  original Stage 1 authorization. The Product Owner's later
  [R-030](../research/records/r-030-h3-real-multi-node-route.md) decision
  authorizes only the bounded Stage 2 tracer after local development readiness.
  It does not complete official Stage 1 qualification: Ubuntu `short`, current
  `churn-2h`, and independent `unattended-24h` remain required before the final
  integrated H3 verdict or any stronger external/release claim. The later
  [R-031](../research/records/r-031-h3-service-connection-application-interface.md)
  decision accepts the clean committed `95/95` local Stage 2 Docker development
  campaign only as readiness for the bounded Stage 3 Service Connection and
  Application Interface tracer. It does not change those official gates. All
  later H3 stages remain sequential research and Product Owner decision gates.

## Horizon 4 — Public Beta

The following are **promotion gates**, not current implementation tasks:

- signed unprivileged Windows 11 and Ubuntu LTS packages, repair, safe update,
  uninstall, authority backup, and rollback protection;
- permissionless public Contributor admission, hostile bootstrap, direct-source
  separation, Bridge entry, transport replacement, and bounded abuse controls;
- the canonical permissionless Namespace, lease/recovery/governance rules, and
  qualified Private Resolution;
- real threshold custody, transparent Candidate View construction, independent
  auditors/builders, reproducible packages, and protocol/build transition;
- OS-enforced or brokered per-Application principals and a qualified Network-
  Isolated Application Boundary for any Application-level location claim;
- the applicable R-023 cross-platform performance, recovery, overload, evidence,
  and requalification cells;
- measured independent operator/source capacity and external security review.

Public Beta uses beta thresholds. **Stable Network** is a later promotion of a
running beta, not a fifth design backlog: its larger effective family counts,
mature operational drills, and extended external evidence are evaluated only
after beta operation produces real data. Missing people or independent operators
block the claim; they do not create work for the current one-to-one team.

## Outside the network core

These are Applications, optional Overlay Services, or separate future products:

- messenger, Inbox, Contacts, social graph, presence, calls, and notifications;
- offline delivery, retained queues, history, content persistence, and site
  replication;
- universal User identity, Persona, proof of personhood, wallet, token, payment,
  staking, or incentive market;
- public Service search, directory, recommendations, or discovery feed;
- clearnet exit, VPN behavior, or general anonymous Internet access;
- multi-instance availability, replicated origin state, and automatic failover;
- bundled browser, arbitrary application runtime, decentralized compute, or
  content-safety engine;
- gateways to other networks and mobile/macOS clients.

An item on this list may return only with its own product job, threat model,
resource budget, and explicit scope decision. It cannot be added as “future
proofing” of the carrier.

## Scope-change rule

1. Only the current horizon enters the implementation backlog.
2. Later-horizon requirements constrain claims and architecture seams, but do
   not authorize placeholder subsystems or speculative abstractions now.
3. Security is never silently deferred: either the required condition is built
   and tested for a claim, or that claim is withheld.
4. A feature advances horizons only after evidence from the preceding horizon
   shows that it is still needed and the Product Owner explicitly promotes it.
5. Failure of the current route candidate triggers redesign or project stop,
   not compensating scope in naming, governance, SDKs, or UI.
6. Go is the maintained project foundation under ADR-0009. Runtime dependencies
   and protocol-bound foundations remain open until evidence justifies them.

## How to read the other documents

| Document | Meaning after this scope decision |
|---|---|
| [Vision](vision.md) | Long-term Product Core and public product intent. |
| [Functional map](functional-map.md) | Requirements registry across all horizons, not one backlog. `fixed` means decision maturity, not “build now.” |
| [Operating model](operating-model.md) | Target lifecycle for a public product; only explicitly promoted parts apply to Carrier Lab. |
| [Threat model](../security/threat-model.md) | Conditions required before making each claim; an unclaimed condition need not become a current feature. |
| [R-013](../research/records/r-013-carrier-lab-technology-candidates.md) | Frozen Carrier Lab experiment contract and current component candidates; not a production-stack decision. |
| [R-014](../research/records/r-014-language-runtime-candidates.md) | Evidence and rationale for the Go project foundation recorded by ADR-0009. |
| [R-023](../research/records/r-023-interactive-route-performance-budget.md) | Future qualification contract and hypothesis set; Carrier Lab uses only its coarse decision metrics. |
| [R-024](../research/records/r-024-operational-product-closure.md) | Completeness audit of the eventual public lifecycle, not authorization to implement every mechanism now. |
| [Development gates](../development/entry-gates.md) | The only promotion path between horizons. |
