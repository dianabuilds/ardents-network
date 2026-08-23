---
id: R-085
title: What M10 Application operating profile can transfer local admission without asserting qualified Isolation?
status: accepted
owner: Product Owner and Codex
started: 2026-08-23
reviewed: 2026-08-23
---

# R-085 — M10 generic Broker scope

## Decision this unlocks

Select the narrow M10 scope that may replace `applicationipc` and the local
admission/result parts of `serviceconn` without selecting an operating-system
sandbox, Installed profile, Windows activation, or an Application Location
Privacy claim.

## Current contract

`CONTEXT.md` separates an Application Principal, Local Grant, Connection
Interface, Service Administration Interface, and Network-Isolated Application
Boundary. DA-08 explicitly forbids claiming qualified isolation or supported
host behavior before a platform threat/Adapter decision and evidence. M9 now
owns native Service Connection lifecycle but leaves local admission and result
projection in a temporary adapter. The project has one Product Owner and
Codex; it has no independent platform-validation team.

## Hypotheses

- **H1:** M10 implements a generic Broker that binds ephemeral sessions to a
  caller-supplied opaque local principal fact and scoped Local Grant, reports
  `generic/unqualified` isolation, and refuses any request that asks it to
  claim or enforce a Network-Isolated Application Boundary.
- **H2:** M10 selects a Linux or Windows sandbox now and treats its process or
  IPC facts as qualified isolation.
- **H0:** no local admission transfer is safe before qualified Isolation.

## Evaluation criteria

The selected scope must preserve separate Connection and Service Administration
authority, immediate revocation of new work, finite data drain only when a
preselected grant permits it, and one bounded terminal result. It must not make
an ordinary PID, socket, desktop user, token, loopback listener, or generic
adapter a distinct Application Principal or an isolation claim. It must add no
platform dependency, `unsafe`, privileged helper, or hidden operator role.

## Evidence plan

### Primary sources

Inspect `CONTEXT.md`, DA-08, the G0 A01--A03 rows in the Stage 8 workbook,
R-051's distinction between a launcher-bound principal and generic local trust
domain, and the current `applicationipc`/`serviceconn` callers (accessed
2026-08-23).

### Experiment

The M10 Broker interface must prove principal/grant mismatch refusal,
Connection-versus-Administration separation, descendant invalidation,
immediate new-work revocation, finite drain, and terminal result projection
using deterministic opaque local-principal adapters. A later qualified
platform profile needs independent real process-tree, socket-escape, and
substitution evidence; generic tests cannot satisfy that gate.

### Failure scenarios

An untrusted same-user sibling reuses a socket or token; a revoked application
opens new work during drain; an Administration grant is presented to a
Connection operation; a generic adapter is presented as sandbox evidence; or a
platform adapter is unavailable. Each must fail closed or report unqualified,
never silently broaden authority or privacy claims.

## Findings

- **Inspection:** the current local session maps in `serviceconn` combine
  admission with publication and connection execution, so callers cannot test
  Local Grant lifecycle through one dedicated Interface.
- **Sourced fact:** R-051 records that PID, named endpoint, and same-user facts
  alone do not establish a distinct Application Principal.
- **Inference:** a generic Broker can own volatile grant/session state without
  pretending to authenticate more than the supplied local trust domain.
- **Assumption:** the Product Owner's standing Stage 8 delegation permits a
  generic, explicitly unqualified scope when it makes no platform or privacy
  product claim.

## Options

| Option | Fit | Decision |
|---|---|---|
| H1 generic Broker, no Isolation claim | Preserves authority separation and makes the M9 adapter removable without a platform promise. | Choose. |
| H2 select a native sandbox now | Would require platform threat, Adapter, and independent escape evidence not present in this project. | Reject. |
| H0 defer all Broker work | Retains a mixed local-admission owner although its generic authority rules are independently testable. | Reject. |

## Recommendation

Choose H1 with high confidence. M10 may create `application/broker` and an
explicitly unqualified `application/isolation` observation contract, but may
not create a sandbox implementation or claim Network-Isolated Application
Boundary. The strongest objection is that callers may mistake usable generic
attachment for isolation; the Interface therefore returns the unqualified
state explicitly and has no success value representing qualified isolation.

## Disposition

**Accepted 2026-08-23 under the Product Owner's standing Stage 8 delegation.**
R-085 closes DA-08 only for generic M10 admission/composition. A qualified
Linux/Windows Isolation profile remains a new research question and ADR gate.
No ADR is needed: this decision declines platform lock-in and changes no
product privacy claim.
