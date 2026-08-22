---
id: R-062
title: Which supported-platform resource adapters can enforce retained H3 profiles?
status: accepted
owner: Codex
started: 2026-08-22
reviewed: 2026-08-22
---

# R-062 — Resource platform scope

## Decision this unlocks

M4 may retain and deepen `internal/resource` only after selecting the platforms
on which its profile is actually enforced and measured. This prevents a
non-Linux process from reporting a normal resource state while its OS adapter
does nothing.

## Current contract

R-028 selects explicit cgroup-v2, rlimit, Go-runtime, and external-accounting
controls for the H3 experiment profiles; it does not establish equivalent
Windows, macOS, or generic Unix enforcement. R-029 assigns admission, drain,
and shutdown reactions to consumers, while `resource` owns measurement and
pressure decisions. The accepted Stage 8 M4 wave requires an explicit platform
scope and a fail-closed unsupported result.

## Hypotheses

- **H1:** Linux cgroup-v2 plus rlimit is the only currently supported runtime
  adapter; every other platform refuses production readiness. In-repository
  injected measurements are behavior-test seams, not runtime platform adapters.
- **H2:** A portable Go-only observation can safely support the same profiles.
- **H3:** A Windows-native job-object/metric adapter can support the profiles
  now with equivalent enforcement and evidence.
- **H0:** no current adapter meets the retained contract; resource-governed
  behavior must stop pending a new measured profile.

## Evaluation criteria

The selected adapter must bind every enforced profile value to a concrete OS or
runtime observation, reject unavailable counters and placement rather than
returning zeros, preserve `PROTECT`/`DRAIN` semantics across counter reset and
restart, and keep Node, Route, State, and Entry consumers from becoming policy
owners. It must not create a public capacity or platform claim from a local
test result.

## Evidence plan

### Primary sources

- R-028 and R-029, reviewed 2026-08-22.
- Current `internal/resource/{sample,placement}_{linux,other}.go`, inspected
  2026-08-22.
- Linux cgroup v2 and `getrlimit(2)` sources already recorded by R-028; any
  new Windows adapter requires its official platform documentation and an
  independent measurement plan.

### Experiment

Run profile creation, placement, observation, counter-reset, oversubscription,
and restart tests on every candidate platform. Retain exact OS/build/cgroup or
job configuration and external resource observations. An unsupported platform
must produce a stable explicit refusal before any Node/Route/State readiness.

### Failure scenarios

- absent or forged pressure counters;
- cgroup/job reassignment after start;
- counter reset, cgroup deletion, or partial metric read;
- parent/child process escape from the measured boundary;
- platform-specific success that cannot enforce the declared profile;
- restart after a terminal resource refusal.

## Findings

- **Sourced fact:** R-028's selected H3 profile is explicitly cgroup-v2 and
  rlimit based, and does not choose a portable substitute.
- **Measurement:** the current Linux adapter reads cgroup-v2, `/proc`, Go
  runtime metrics, and validates rlimit/placement.
- **Sourced fact:** the current `!linux` files return a zero Sample and nil
  placement error, so they cannot prove the retained profile on this platform.

## Options

| Option | Fit | Risk | Disposition |
|---|---|---|---|
| H1: Linux-only now | Matches R-028 evidence and fails honestly elsewhere. | Defers Windows resource-governed behavior. | Candidate. |
| H2: portable observation | Low effort. | Cannot enforce cgroup/rlimit profile or establish full process-tree accounting. | Reject unless new evidence contradicts R-028. |
| H3: Windows adapter | May widen supported scope. | Requires a separately measurable native boundary and maintenance proof. | Future candidate. |
| H0: stop | Avoids a false readiness signal. | Blocks current consumers until a supported scope is selected. | Fallback. |

## Recommendation

Accept H1. Linux cgroup-v2 plus rlimit is the only currently supported runtime
adapter. The default adapter on every other platform must deterministically
refuse both readiness and observation; a maintained runtime caller must
therefore remain unready or drain, rather than infer a normal resource state
from absent OS evidence. `Measure`, `CheckPlacement`, and `ResourceCheck` are
in-repository behavior-test seams; the caller audit finds no non-test override.
A future native adapter needs its own measured platform record before widening
this scope.

## Disposition

**Accepted H1 by the Product Owner on 2026-08-22.** The `!linux` adapters now
return the stable unsupported-platform error. `Check` tests that error before
runtime placement checks, and default `Observe` produces a protected drain
observation with the same error. The Windows execution test covers the refusal;
the Linux build is compiled during integration. The only custom measurement or
placement/check callbacks are behavior tests, so they do not create a supported
runtime path. No ADR is required unless a new native resource foundation or
profile lock-in is selected.
