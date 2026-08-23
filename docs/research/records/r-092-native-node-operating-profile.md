---
id: R-092
title: Which measured Linux operating profile can admit one native Route Node role?
status: open
owner: Product Owner and Codex
started: 2026-08-23
reviewed: 2026-08-23
---

# R-092 — Native Node operating profile

## Decision this unlocks

Select the first concrete, fail-closed native Node admission and listener
profile for M8/M11, or retain preannouncement status if no measured candidate
meets the contract.

## Current contract

R-076/ADR-0024 select mutually authenticated TCP/TLS 1.3 legs and R-078/ADR-
0026 selects their closed LegBinding record. R-081 expressly leaves native Node
capacity, host profile, pressure, drain, and listener integration unselected:
the retained H3 probe capacity cannot be renamed or inherited. NET-01A fixes
the Linux infrastructure reference environment as Ubuntu LTS `x86-64`, 2 vCPU,
2 GiB RAM, and symmetric 100 Mbit/s; it is not itself a Node admission result.

## Hypotheses

- **H1:** a measured native role-carriage workload can select one finite Node
  profile with an explicit reservation, pressure, drain, and withdrawal rule.
- **H2:** a native carrier is functional but no candidate capacity is safe on
  the reference host; preannouncement remains the correct result.
- **H0:** the native role-carriage workload cannot preserve the selected TLS,
  LegBinding, resource, or cleanup contract.

## Evaluation criteria

Before a profile can be selected, the same declared host must show all of:

- State-pinned TLS 1.3 and reciprocal v1 LegBinding on every admitted leg;
- an explicit refusal before allocating a new listener/connection when the
  measured reservation budget is exhausted;
- pressure transition, finite drain, withdrawal, cancellation, and joined
  cleanup with no surviving listener, connection, or goroutine owned by the
  run; and
- raw per-second CPU/RSS/FD/socket observations, exact workload bytes and
  elapsed time, host identity, source/binary digests, and all failed attempts.

The experiment's capacity points are observations, not a promise. No result
from a different operating system, co-resident Endpoint, or H3 probe is a
native Node profile.

## Evidence plan

### Primary sources

- R-076, R-078, R-081, ADR-0024, ADR-0026, R-023, and NET-01A, inspected
  2026-08-23.
- Go 1.26 `crypto/tls` documentation, to be rechecked on the measured host.

### Experiment

[`experiments/r-092-native-node-profile/`](../../../experiments/r-092-native-node-profile/)
contains a disposable, synthetic mTLS plus reciprocal-LegBinding baseline. It
has no Node listener, State root, H3 reader, or capacity decision. A follow-up
run must add the complete isolated role-carriage/pressure harness and retain
its raw Linux observations outside Git.

### Failure scenarios

- an H3 profile or capacity appears in any native result;
- TLS/ALPN or a reciprocal binding is absent, substituted, or downgraded;
- cancellation leaves a listener, connection, or worker alive; or
- a partial baseline is presented as a selected operating profile.

## Findings

- **Inspection:** the current Node implementation still owns only the H3
  probe runtime; the native Route Module has no Node listener or profile.
- **Measurement:** the disposable harness completed two sequential 4,096-byte
  synthetic loopback legs with TLS 1.3, the exact ALPN, reciprocal v1 bindings,
  and byte-identical echo on the local development host. It is a harness sanity
  result only: that host is not the R-092 Linux reference environment and its
  timing/allocation output is not a capacity measurement.
- **Measurement:** no Linux reference-host result has yet been captured.

## Recommendation

Run the named follow-up experiment on the declared Linux reference host. Do
not select a capacity, expose a peer runtime, or alter the H3 profile before
its complete evidence exists.

## Disposition

Open. This record adds no ADR, package, dependency, Node profile, or product
claim. Keep the disposable baseline only while it answers this question; delete
or replace it when a source-bound measured suite supersedes it.
