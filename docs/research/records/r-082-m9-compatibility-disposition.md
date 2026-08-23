---
id: R-082
title: Which M9 Service Connection and local publication bytes require compatibility support after the DA-10 inventory?
status: accepted
owner: Product Owner and Codex
started: 2026-08-23
reviewed: 2026-08-23
---

# R-082 — M9 compatibility disposition

## Decision this unlocks

Let M9 replace the H3 `serviceconn` and local publication tracer bytes without
retaining an unbounded reader or moving H3 framing into a native package under
a new name.

## Current contract

R-076/ADR-0024 select native `ardents-interactive-route-v1`, State/publication
authority, and Service Connection-owned recovery. DA-10 requires a named
external observer before a local compatibility surface may survive. The
S8.3 compatibility-observer inventory is factual input only: it found no
repository-visible external consumer for A01/A02 local stream and terminal
bytes, A03 publication administration text, or the associated H3 `AS*` /
domain-tag framing. Product semantics still require separate publication and
connection authority, exact Service Target authentication, bounded recovery,
and one classified terminal outcome.

## Hypotheses

- **H1:** M9 makes a coordinated C0 break for its unobserved H3 local and
  connection bytes, replaces the owning modules and their tests together, and
  preserves only the semantic contract.
- **H2:** retain H3 decoders or an adapter until a hypothetical observer is
  discovered.
- **H0:** an actual external consumer needs a bounded migration first.

## Evaluation criteria

The result must preserve no direct Service fallback, no silent profile
downgrade, authority separation, finite recovery, and one honest terminal
classification. It must name an actual observer, source identity, support
horizon, and removal test for any compatibility reader; an absent observer
cannot justify an indefinite legacy path. M9 must not decide M13 command
syntax or remove historical C4 evidence readers by implication.

## Evidence plan

### Primary sources

- S8.3 [compatibility-observer inventory](../../development/stage-8-compatibility-observer-inventory.md),
  [DA-10 register](../../development/stage-8-decision-authority-register.md),
  R-076/ADR-0024, and M9 plan row, inspected 2026-08-23.
- Repository-visible `internal/serviceconn`, `internal/serviceendpoint`,
  `cmd/ardents-service`, `cmd/ardents-publish-app`, and
  `cmd/ardents-stream-app` readers, searched 2026-08-23.

### Experiment

M9 replaces the old owner and runs behavior tests through the new publication
and connection interfaces. A residue search must show that no maintained
package reads H3 `AS*` bytes or H3 Service Connection domain tags. Any newly
identified consumer falsifies this disposition and receives its own bounded
observer rule before mutation.

### Failure scenarios

- an unrecorded user or automation is silently broken;
- an H3 decoder survives as an unbounded fallback;
- a C0 byte break weakens target authentication, recovery, or terminal
  classification; or
- command and historical-evidence removal is smuggled into M9.

## Findings

- **Inspection:** the inventory finds only source, e2e, and tracer-command
  consumers of A01--A03; no current product/operations/reference schema,
  deployment, packaging, or external versioned contract names them.
- **Inspection:** every H3 Service Connection reader is owned by the current
  `serviceconn`/`serviceendpoint` tracer implementation or its tests.
- **Assumption:** the Product Owner's standing Stage-8 delegation authorizes a
  coordinated break when the inventory names no external observer. A newly
  named consumer overrides this assumption.
- **Inference:** H1 is safer than H2 because a legacy reader would make a
  retired profile reachable after a native failure without any supported
  observer or finite removal condition.

## Options

| Option | Disposition |
|---|---|
| C0 break of M9 H3 local/connection bytes | Choose. No observed external consumer requires them; semantic behavior is rebuilt under the native profile. |
| Indefinite H3 compatibility adapter | Reject. It creates a reader and downgrade surface without an observer, expiry, or removal test. |
| Defer M9 until an observer appears | Reject. Absence of an observer is not a reason to preserve a tracer runtime. |

## Recommendation

Choose H1 with high confidence. The strongest objection is an unrecorded
private consumer; the decision therefore has a falsification rule and does not
cover commands or C4 evidence. No ADR is needed: R-082 selects a compatibility
disposition, no new protocol, platform, or technology.

## Disposition

**Accepted 2026-08-23 under the Product Owner's standing Stage 8 delegation.**
M9 may delete `serviceconn` and current Service endpoint H3 readers/writers
only as it replaces their semantic behavior. M13 separately decides commands;
M14 separately audits historical evidence.
