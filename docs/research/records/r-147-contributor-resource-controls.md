---
id: R-147
title: Resource controls for voluntary contribution on personal devices
status: open
owner: Product Owner and Codex
started: 2026-09-06
reviewed: 2026-09-06
---

# R-147 — Which resource controls make personal-device contribution viable?

## Decision this unlocks

Select a future-product resource-control concept for a person who receives
personal value from an Ardents-connected Application and separately enables
relay contribution on the same device. The Product Owner explicitly permits
recommendations that require revising accepted documentation. This request
narrows the ongoing contributor-incentive discussion to resource controls.

This is a source-backed product comparison, not implementation authorization,
market validation, a selected co-resident profile, or a C0 research execution.

## Current contract

- [Product scope](../../product/scope.md) separates the Network from Applications,
  persistence, compute, payments, and incentive markets.
- [J-07](../../product/journeys.md#j-07--contribute-network-resources) and the
  [operating model](../../product/operating-model.md) require a dedicated public
  Contributor host and exclude owned Contributor identities/families from an
  Endpoint's Routes. Co-resident contribution is the proposed change, not an
  inherited supported profile. The queue names R-093 as deferred; its historical
  evidence does not select a co-resident experiment here.
- [Threat model](../../security/threat-model.md) requires finite hierarchical
  accounting including helpers, buffers, control, retransmission, and cleanup;
  diagnostics are local and bounded by default. Resource isolation is not an
  anonymity boundary, and public timing may reveal local activity changes.
- [Network, Route, and Node](../../technical/network-route-node.md) owns a Linux
  resource-pressure reader and one bounded dedicated Rendezvous profile. It does
  not establish a general owner-configurable resource manager.
- [Documentation ownership](../../development/documentation.md) keeps this open
  question outside current product contracts. No ADR is created or superseded.

## Hypotheses

- **H1:** Measured presets with editable ceilings and bounded adaptation reduce
  configuration burden while preserving owner limits and useful relay work.
- **H2:** Fixed ceilings suffice; adaptation adds churn and maintenance without
  materially improving personal-device responsiveness or voluntary retention.
- **H3:** Scheduled or reserved contribution provides more useful network work
  than highly intermittent spare-resource participation for some duties.
- **H0:** No co-resident option satisfies owner impact, useful relay work, and
  privacy/resource boundaries; dedicated contribution remains necessary.

## Evaluation criteria

Comparison rules were set before the external comparison; no experiment ran.

1. Comprehension: distinguish activity permission, ceiling, current consumption,
   period allowance, reserve, and the consequences of pause or withdrawal.
2. Integrity: every controlled process and shared overhead has one parent budget;
   new tasks, identities, or restarts cannot multiply an allowance.
3. Owner impact: compare the same personal workload with contribution off/on;
   preserve latency, responsiveness, errors, and independent resource traces.
4. Network value: useful eligible relay work, rejection, churn, completion,
   recovery, and post-exclusion reserve. Processes and configured capacity are
   not measured capacity or operator independence.
5. Privacy: local aggregates; no destinations, routes, payloads, process lists,
   application titles, or personal schedules uploaded by default.
6. Lifecycle: overload, a lower live limit, exhausted quota, restart, clock or
   network change, withdrawal, and unavailable enforcement have bounded outcomes.
7. Maintenance: one Product Owner and Codex; one first measured platform; no
   assumed operators, interview panel, paid control service, or support staff.
8. Dependencies: prefer native controls and maintained components. These are
   product-pattern references, not selected runtime dependencies; license and
   distribution review is required before adopting any implementation.

Qualitative judgments below are inferences, not observed conversion rates or
numeric scores. Release defaults, acceptable owner-impact thresholds, burst
windows, tolerances, and minimum useful duty budgets must be fixed before a
candidate experiment. This comparison does not supply those measured numbers.

## Evidence plan

### Primary sources

All sources accessed **2026-09-06**:

| ID | Primary source | Relevant evidence and boundary |
|---|---|---|
| S1 | [BOINC preferences](https://github.com/BOINC/boinc/wiki/Preferences) | CPU, memory, disk, traffic, battery/activity policies, simple/advanced views. Volunteer computation can pause differently from a live relay. |
| S2 | [Transmission configuration](https://github.com/transmission/transmission/blob/main/docs/Editing-Configuration-Files.md) | Alternative speed limits and day/time scheduling. Transfer scheduling is not live Route lifecycle qualification. |
| S3 | [Tor bandwidth accounting](https://support.torproject.org/relays/performance/bandwidth-limits/) | Period quotas, rate shaping, and randomized wake-up to reduce synchronized availability gaps. Accounting semantics must not be copied without explaining directions. |
| S4 | [Linux cgroup v2](https://www.kernel.org/doc/html/latest/admin-guide/cgroup-v2.html) | Hierarchy, CPU bandwidth, pressure/terminal memory controls, and block-device I/O controls. These are different enforcement semantics. |
| S5 | [Linux PSI](https://www.kernel.org/doc/html/latest/accounting/psi.html) | CPU, memory, and I/O stall-pressure information at system or cgroup scope, where available. Pressure is not an Application latency guarantee. |
| S6 | [Windows Job Objects](https://learn.microsoft.com/en-us/windows/win32/procthread/job-objects) and [CPU-rate controls](https://learn.microsoft.com/en-us/windows/win32/api/winnt/ns-winnt-jobobject_cpu_rate_control_information) | Group accounting/limits and CPU-rate controls. Process inheritance, breakaway, and resource coverage require Windows-specific proof. |

### Experiment

No product experiment or external recruitment is selected. A conversation
illustration compares six concepts and demonstrates only a synthetic CPU
admission ceiling. It reads no host metrics and changes no host settings.

The illustration assumes a 100-percent normalized CPU envelope, owner demand P,
relay ceiling C, and a 20-percentage-point adaptation reserve. The displayed
relay allowance is max(0, min(C, 100 - P - 20)); unused capacity is the remainder.
P and C are illustrative inputs, not measured Ardents defaults. Output is an
upper allowance, not actual consumption or useful throughput. If P alone uses
the reserve, the allowance becomes zero; the model cannot control unrelated
programs. Quota/battery toggles show the admission state after withdrawal, not
an instantaneous zero-cost transition. Memory, network, and I/O are not inferred
from this CPU illustration.

### Failure scenarios

- Connection/handshake flood, slow readers, or helpers try to escape accounting.
- Repeated load changes cause controller oscillation or excessive route churn.
- Memory cannot be reclaimed promptly; a lowered disk cap is below retained data.
- Traffic allowance ends during live work, cleanup, safety refresh, or update.
- Restart, crash, clock rollback, accounting corruption, or network change grants
  a fresh allowance accidentally.
- Many scheduled contributors stop together or an attacker triggers withdrawals.
- OS controls are absent, not delegated, or ineffective for part of the workload.
- Public relay behavior exposes a person's activity rhythm.

## Findings

- **Sourced fact, S1:** BOINC separates the number of CPUs used from CPU-time
  duty cycling, offers activity/battery suspension, distinct memory preferences,
  storage ceilings/free-space protection, traffic rates, and period allowances.
  Its simple and advanced views expose different amounts of configuration.
- **Sourced fact, S2:** Transmission exposes alternative rates controlled by a
  weekly time schedule. This establishes a maintained scheduling pattern, not
  its suitability for Ardents connection continuity.
- **Sourced fact, S3:** Tor combines rate and period controls and discusses daily
  allowances and randomized wake-up to spread contribution over time.
- **Sourced fact, S4:** cgroup children cannot override ancestor limits. CPU
  quota applies per period; memory.high applies reclaim pressure; memory.max can
  cause a cgroup OOM and can be exceeded temporarily; I/O limits allow bursts.
  A storage-capacity quota is distinct from I/O rate limiting.
- **Sourced fact, S5:** PSI reports stalled time for CPU, memory, and I/O and can
  support pressure-triggered actions. It does not measure spare Internet capacity.
- **Sourced fact, S6:** Windows provides group-level limits and CPU-rate control,
  but identical UI labels cannot establish identical Linux/Windows guarantees.
- **Assumption:** Configuring and maintaining a second host is a substantial
  adoption barrier for ordinary users. No Ardents conversion dataset was found
  in the current owner documents inspected; the magnitude is unmeasured.
- **Inference:** Presets can reduce setup effort; visible ceilings can increase
  predictability. Neither demonstrates voluntary relay retention.
- **Inference:** Adaptive CPU admission is a plausible first bounded mechanism.
  Memory is stateful, and available access-link bandwidth cannot safely be
  inferred from Ardents traffic alone. Resource-specific control is necessary.
- **Measurement:** No Ardents resource, performance, adoption, or security
  measurements were produced. Visual QA tests only the illustration.

## Options

These options are composable. Presets select settings; adaptation and schedules
change admission within ceilings; reservation describes an owner's intended
resource commitment. A monetary estimate is an optional presentation layer.

| Option | Owner-facing proposition | Best fit | Network effect | Complexity and rejection reason |
|---|---|---|---|---|
| A. Explicit ceilings | Set CPU, RAM, directional rates, disk and period allowance directly. | Technical owners and servers. | Predictable configured bounds; demand and actual availability still vary. | Simplest starting control policy, but enforcement remains substantial. Reject as the only mass-user entry if owners cannot select sensible values. |
| B. Measured presets | Choose personal computer or always-on server, inspect exact budgets, then edit. | Ordinary users. | More consistent starting policies if matched to measured devices. | Adds a small UI/configuration layer over A. Requires measured envelopes; a friendly label cannot make arbitrary defaults safe. |
| C. Spare-resource adaptation | Reduce contribution when local pressure rises, never exceed approved ceilings. | Personal computers with changing workloads. | Adds opportunistic capacity; rapid withdrawal may reduce usefulness. | Needs bounded feedback, hysteresis, minimum duty budgets and drain. Reject if churn or owner latency is worse than A. |
| D. Schedule and allowance pacing | Contribute at chosen times and spread the traffic allowance over the chosen period. | Metered connections and predictable working hours. | Predictable windows, but synchronized gaps and low paced rates can make a role unusable. | Time/restart accounting and orderly boundary withdrawal add work. Reject when remaining budget cannot support any useful duty plus terminal reserve. |
| E. Reserved contribution | Allocate a modest stable budget; optionally lend more within a separate ceiling. | Always-on home servers and community operators. | Plausibly more stable duty supply than pure idle participation. | Requires explicit owner commitment and admission validation. A reservation is not guaranteed capacity, uptime or network priority. |
| F. Estimated money budget | Show approximate incremental electricity and metered-traffic cost. | Cost-sensitive owners. | Makes tradeoffs visible but is not an enforceable network resource. | Tariffs, idle baseline and power measurement differ. Use an optional estimate later; reject a guaranteed ruble ceiling without trusted billing/energy coverage. |

### Shared product rules proposed for every option

- Explicitly enable a duty. Relay participation does not enable retained storage,
  arbitrary compute, GPU work, or consensus participation. Future duties need
  their own opt-in budgets and may not borrow by default.
- One installation ceiling contains role/Application budgets and a finite shared
  operating reserve. Identity, process, and connection multiplication grants
  no extra allowance. No controller decides who is a Person.
- Show ceilings, actual usage, period remaining, and the limiting reason as
  different values. A configured maximum is not a consumption target.
- CPU controls specify whole-machine/core-equivalent units, sampling windows,
  and permitted bursts. Memory accounting names the entire controlled boundary;
  reducing CPU use does not release memory automatically.
- Disk controls cover managed retained data, temporary files, logs, staging,
  write volume/rates, and minimum free space where supported. No automatic
  deletion of personal data to satisfy a lower cap; swap has a separate bound.
- Network controls separate ingress and egress rates and allowance directions.
  Account attributable encryption, control, retries, and maintenance. An
  application cannot guarantee an ISP bill or prevent packets already counted
  upstream. A rate cap does not automatically qualify latency protection.
- Prefer fixed directional network caps for the first candidate. An adaptive
  access-link controller is separate research; do not silently run unbounded
  speed tests or assume idle Ardents traffic means an idle household link.
- Admission stops before the terminal reserve is consumed. Exhaustion or owner
  pause ends duties within a bounded window; safety-state expiry still closes
  work. Updates and security work cannot spend an unlimited exception budget.
- Persist allowance consumption and period identity conservatively. A crash
  must not undercount; uncertain/corrupt accounting suspends affected spending.
  Ordinary restarts, clock changes and network changes do not silently reset it.
  An explicit owner reset is a visible new authorization, not recovered usage.
- Apply lower limits atomically as policy; show pending drain when physical
  consumption cannot fall immediately. An owner can stop immediately, accepting
  explicit connection loss. Storage reduction cannot promise immediate reclaim.
- Low limits may mean that no network duty is currently possible. Show that
  outcome; do not silently raise the limit, weaken authentication, or inflate
  contribution counters.
- Resource controls cover managed processes. An independently launched external
  Application is not covered merely because it connects through Ardents.
- No trusted global activity graph or new central controller is required. Shared
  deployment/funding does not prove independent operators. Co-residence still
  requires a separate threat/qualification decision even if budgets pass.

### Plausible enforcement direction, not a selected architecture

First evaluate Ubuntu LTS with an OS-enforced parent boundary and distinct
controlled relay/Application process groups, coupled to Ardents admission,
finite queues and lifecycle. Use local pressure for bounded CPU adaptation;
keep memory and directional network controls explicit. OS controls and product
admission solve complementary problems. Container packaging alone supplies
neither a configured budget nor an anonymity boundary. Do not introduce a
cluster orchestrator or a generic remote management service for this need.

A later Windows candidate must independently prove process coverage and each
resource's enforcement. Do not label unavailable controls as hard guarantees.
The current Go/runtime/dependency rules remain applicable to any later work.

## Recommendation

**Run a named follow-up experiment: fixed ceilings versus bounded adaptation.**
This is a recommendation for later selection, not an experiment activated here.

The preferred product concept combines B + A + a narrow C: start with a measured
preset, expose every numerical ceiling, and allow bounded CPU-pressure response.
Add D for explicit schedules and quota pacing. Offer E to always-on operators.
Keep F as a later transparent estimate, not the primary promise.

Confidence is moderate in implementability on one controlled Linux platform,
low in the size of any adoption effect, and unestablished for co-resident
privacy. The strongest counterargument is that extra intermittent nodes may
add maintenance and churn while contributing too little stable relay work;
fixed limits on dedicated or always-on hosts could be a better product.

Before a candidate run, freeze platform, host, personal workload, eligible
relay duty, OS accounting boundary, CPU/RAM/I/O/network limits, burst windows,
terminal reserve, and pass/fail thresholds. Compare contribution disabled,
fixed ceilings, and adaptive ceilings under identical normal and hostile load.
Fail for escaped accounting, silent allowance reset, unbounded queues/drain,
forbidden disclosure, or exceeding the predeclared resource tolerance. Reject
adaptation if it fails its predeclared owner-impact and useful-relay-work
comparison against fixed ceilings. No tolerances may be invented after results.

The Product Owner and Codex can first perform a structured walkthrough and
later a bounded technical candidate. External participants are a future
condition for studying comprehension and voluntary 30-day retention, not
currently scheduled staff. Record eligible-start conversion, retained useful
contribution, owner complaints and support time; do not substitute badge clicks
or raw byte counts for meaningful adoption.

## Disposition

The theoretical comparison is complete; **R-147 stays open** because no product
concept, co-resident profile or experiment has been accepted. This record and
its queue row are the only intended repository changes. No ADR, maintained
code, package, dependency, protocol, or current product claim changes.

The interactive conversation illustration is outside the repository. It is
synthetic explanatory material, not experiment code or qualification evidence.
