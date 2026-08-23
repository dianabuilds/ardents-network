---
id: R-081
title: What resource admission profile may the unannounced native Route use before Node integration?
status: accepted
owner: Product Owner and Codex
started: 2026-08-23
reviewed: 2026-08-23
---

# R-081 — Native Route resource boundary

## Decision this unlocks

Let M8 replace H3 Route selection and framing without silently carrying an H3
resource profile into the native peer path or inventing unmeasured Node limits.

## Current contract

R-076 selects `ardents-interactive-route-v1`, but `internal/resource`'s
concrete profiles and the current Route actor/plan runtime are H3 tracer
artifacts. M4 permits only a selected, measured resource coordinator; M11 owns
the eventual Node listener/admission integration. No native peer announcement
or Qualification claim exists.

## Hypotheses

- **H1:** M8 provides native selection, binding, setup, cancellation, and
  cleanup behind an explicit resource-admission port; production Node admission
  waits for M11's measured selected profile.
- **H2:** reuse an H3 resource profile under a native name.
- **H0:** choose a new numeric profile without platform measurement.

## Evaluation criteria

The result must neither admit native work without a caller-owned resource
decision nor claim an unmeasured capacity. It must permit deterministic
behavior tests for refusal, cancellation, and cleanup, and keep Node-specific
placement out of endpoint Route selection.

## Evidence plan

### Primary sources

- R-076/ADR-0024, R-078/ADR-0026, M4/M8/M11 in the refactoring plan, and the
  current `internal/resource` profile inventory, inspected 2026-08-23.

### Experiment

M8 tests supply a narrow admission function and prove no listener, dial, or
attachment allocation occurs on refusal. M11 must supply host measurement,
pressure, restart, and drain evidence before wiring a real Node profile.

### Failure scenarios

- Native work inherits H3 capacity or process placement without evidence.
- A rejected resource admission allocates a peer socket.
- Route selection learns Node-local resource state or falls back to H3.

## Findings

- **Inspection:** Network State has the native Profile as a known accepted
  profile, while current Route selection and concrete resource profiles retain
  H3 literals.
- **Inspection:** M11, not M8, owns Node listener and pressure/drain lifecycle.
- **Inference:** a narrow caller-owned admission port is sufficient for M8's
  preannouncement behavior, but is not a substitute for an operating profile.

## Options

| Option | Disposition |
|---|---|
| Explicit admission port; defer Node profile to M11 | Choose. Preserves fail-closed admission without fictional measurements. |
| Rename/reuse H3 limits | Reject. A name change would preserve a retired runtime contract. |
| Invent a native numeric limit | Reject. It has no selected host evidence. |

## Recommendation

Choose H1 with high confidence. The strongest objection is that it delays a
real Node listener; that listener belongs to M11 and cannot be made sound by a
Route-only refactor.

## Disposition

**Accepted 2026-08-23 under the Product Owner's standing Stage 8 delegation.**
M8 may implement and test native Route behavior only through an explicit
resource-admission port. M11 must select and measure any concrete native Node
resource profile before a peer-facing runtime announcement. No ADR is needed:
this decision deliberately selects no platform, capacity, wire, or technology.
