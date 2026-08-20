---
id: R-052
title: Which stable Ubuntu and Windows mechanisms isolate a complete Application process tree?
status: open
owner: Product Owner
started: 2026-08-20
reviewed: 2026-08-20
---

# R-052 — Stage 7 Application network isolation

## Decision this unlocks

Select one stable Ubuntu and one stable Windows Adapter for the Network-Isolated
Application Boundary in S7.6. Freeze supported Application constraints,
privilege, process-tree ownership, local IPC/storage allow-list, network deny
semantics, resource limits, observation, cleanup, and honest unsupported result.

## Current contract

ADR-0007, R-048, the
[Application-principal decision proposal](../../development/stage-7-launcher-bound-application-principals-proposal.md),
the threat model, and the lifecycle
specification require a launcher-bound complete Application/helper tree with
only scoped Ardents local IPC, deny-by-default ordinary ingress/egress, separated
Isolation Context storage, and no generic/clearnet fallback. This protects only
Application-induced direct network location disclosure under stated conditions;
it is not a universal sandbox, content filter, or endpoint-compromise defense.

## Hypotheses

- **H1:** stable native mechanisms can be composed behind one Interface:
  AppContainer plus non-breakaway Job/resource/storage rules on Windows, and an
  accepted launcher combination of `no_new_privs`, syscall/network/process/
  filesystem controls on Ubuntu.
- **H2:** a maintained packaged sandbox runtime is safer and smaller to maintain
  than first-party native composition on one or both platforms.
- **H0:** no stable candidate meets complete-tree/no-network/IPC/resource/cleanup
  requirements without a driver, cgo/unsafe, broad daemon, experimental-only
  Interface, or material Application incompatibility.

## Evaluation criteria

- confinement established before untrusted code and inherited by every child,
  helper, shell/interpreter, delayed process, and nested job where supported;
- no breakaway, unrelated inherited handle/descriptor, debugger/broker escape,
  alternate user/session path, or surviving process;
- scoped Ardents IPC and per-context filesystem roots work;
- no ordinary IPv4/IPv6, TCP/UDP/QUIC/raw/local-network socket, DNS, listener,
  fetch/proxy/redirect, callback/SSRF, WebSocket, or WebRTC/STUN escape;
- controlled host/interface observers can independently distinguish candidate
  traffic, harness traffic, absence, and observation failure;
- exact privilege/capability, source, version, license, maintenance, audit/
  advisory, supported-host/API, dependency/tool, and removal profile;
- finite CPU, memory, processes, handles/FDs, storage, IPC, time, logs, and
  cleanup; and
- explicit `isolation-unsupported` with no generic success conversion.

## Evidence plan

### Primary sources

Accessed 2026-08-20:

- Microsoft [Launch an AppContainer](https://learn.microsoft.com/en-us/windows/win32/secauthz/implementing-an-appcontainer);
- Microsoft [Job Objects](https://learn.microsoft.com/en-us/windows/win32/procthread/job-objects)
  and [Nested Jobs](https://learn.microsoft.com/en-us/windows/win32/procthread/nested-jobs);
- Microsoft June 2026
  [Create Process in Sandbox](https://learn.microsoft.com/en-us/windows/win32/secauthz/createprocessinsandbox),
  explicitly experimental and therefore evidence/candidate context only;
- Linux kernel [seccomp filters](https://docs.kernel.org/userspace-api/seccomp_filter.html)
  and [`no_new_privs`](https://docs.kernel.org/userspace-api/no_new_privs.html);
- Linux [network namespaces](https://man7.org/linux/man-pages/man7/network_namespaces.7.html);
  and
- official source/security/license documentation for each packaged candidate
  enumerated before the experiment.

### Experiment

Create `experiments/r-052-stage-7-application-isolation/`. Freeze clean patched
host images and candidate source/binary/supply. Launch deterministic controlled
client/publisher Applications and an adversarial helper corpus. Run all F0-F10
probes with host-side packet/DNS/listener/process/handle/filesystem observers,
resource pressure, grant revoke, broker/Application/Endpoint restart, crash, and
complete cleanup. Run controls proving observers detect identical operations
outside containment. Retain no real User activity or persistent Authority.

### Failure scenarios

GUI/helper incompatibility hidden as success; child/breakaway path; inherited or
broker-opened network handle; localhost treated as harmless; resolver/proxy
escape; rule race before launch; Job/namespace/filter not inherited; endpoint
firewall policy changed globally; privileged broker retains Authority access;
observer blind spot; cleanup leaves profile/rule/process/storage; experimental
API disappears or changes.

## Falsification criteria

Freeze the complete direct/child/helper/handle escape corpus and positive
observer controls before candidate launch. H1/H2 is falsified on a host by one
successful undeclared packet, DNS query, listener, direct callback, cross-context
read, process breakaway, surviving helper, or global policy change; by one
required Ardents IPC/storage operation made impossible without an explicit
unsupported result; or by a collector that misses its matching positive control.

Each F1–F8 probe runs through direct child, grandchild, delayed helper, and
shell/interpreter paths where applicable. The envelope is `32` processes,
`256 MiB` tree memory, `1,024` handles/FDs, and `5 s` complete termination and
cleanup after revoke/crash. Exceeding a bound must deny the operation without an
escape. Any failed required cell rejects the candidate; an observer failure is
`invalid`, never pass. If either required host has no stable candidate, select
O0 or change the product contract explicitly.

## Findings

- **Sourced fact:** AppContainer can launch with a constrained token/capability
  profile, while Job Objects group process trees and expose breakaway/nesting
  semantics that must be configured and observed.
- **Sourced fact:** the new Windows CreateProcessInSandbox Interfaces are marked
  experimental and subject to change; they cannot be the sole maintained H3
  foundation under the current contract.
- **Sourced fact:** Linux seccomp filters can persist across fork/clone/exec when
  configured with `no_new_privs`, but kernel documentation explicitly states
  that seccomp is not a complete sandbox.
- **Inference:** no single mechanism name is evidence. Qualification needs a
  composed profile plus black-box escape and observer controls.

## Options

- **O1:** stable native per-platform composition behind one Ardents Interface.
- **O2:** maintained packaged sandbox runtime with pinned supply and narrow
  launcher Adapter.
- **O0:** no claim-bearing isolated profile on a failing platform; stop Stage 7
  or change the supported product contract explicitly.

## Recommendation

Evaluate O1 and credible O2 candidates with identical hostile probes. Exclude the
experimental Windows API as the sole production candidate. Do not build a
first-party kernel driver or treat seccomp/firewall/AppContainer alone as proof.
Confidence: low-to-medium before measurement.

## Disposition

- State: `open`; no isolation mechanism, dependency, helper, or privilege is
  selected.
- Required before the proposal can be promoted to ADR-0016 and before S7.6.
- Failure cannot be waived by relabeling the same cell generic; the generic
  profile remains separately honest and unqualified.
