---
id: R-124
title: What mechanics demonstrate project-controlled shared control?
status: decided
owner: Product Owner and Codex
started: 2026-08-29
reviewed: 2026-08-29
---

# R-124 — What mechanics demonstrate project-controlled shared control?

## Decision this unlocks

Close H4-6C as a reproducible project-controlled simulation rather than turn it
into a search for people outside the actual Product Owner-and-Codex team.

## Current contract

The [H4-6 journey](../../product/horizon-4/06-transparent-control-transition.md),
[threat model](../../security/threat-model.md), and
[glossary](../../../CONTEXT.md) terms Control Plane, Endpoint, Candidate View,
and Public Beta apply. ADR-0004 keeps Control Plane roots separate and ADR-0054
keeps alpha transition domains separate. ADR-0055 changes only H4-6C's
completion contract: the Product Owner selected a simulation in which role
boundaries are test inputs, not claims about people or organisations. Public
operation remains outside this decision.

## Hypotheses

- **H1:** Fresh simulated role keys, thresholds, retained in-memory artifacts,
  deterministic reconstruction, and explicit failure outcomes can demonstrate
  the intended shared-control mechanics to this project team.
- **H2:** One project-controlled role or an unbounded reader/transition can
  demonstrate the same mechanics without distinct simulated role boundaries or
  retained fault outcomes.
- **H0:** The mechanics cannot be reproduced or a failure cell is accepted.

## Evaluation criteria

The Product Owner and Codex can retain one bounded JSON receipt with a
caller-declared source revision and receipt digest, that reports
`simulation: true`, `simulation_result: passed`, `qualified: false`, and all
of the following:

1. a `3-of-5` routine action and an expiring disable-only `4-of-5` emergency;
2. predecessor-and-successor quorum rotation for loss, compromise, removal,
   replacement, and emergency recovery, with wrong predecessor rejection;
3. two matching full Candidate View reconstructions from one canonical complete
   input log, cutoff, rule revision, root, summary, and indexed proof;
4. two retained builder and auditor signatures over source/dependency/recipe/
   SBOM/qualification/artifact inputs, with forged or mismatched rejection; and
5. the bounded reader's forged, stale, replayed, revoked, conflicting,
   unavailable, boundary-collision, and malformed outcomes.

No criterion asserts factual external independence, public availability, or a
Public Beta claim.

The protected information is every ephemeral simulated private key; the
adversary is a malicious local signer, builder, auditor, or input distributor
trying to forge, omit, replay, downgrade, or escalate a simulation artifact.
The simulator has no network, participant, availability, latency, bandwidth,
or persistent-storage budget: its bounded in-memory inputs and output receipt
are the entire profile. The exact execution-time budget is the repository test
profile; a timeout or resource failure is a failed run, not a passing skip.

Governance is deliberately only the Product Owner decision recorded in
ADR-0055; no external operator, staffing, payment, or ongoing maintenance
dependency is introduced. Standard-library Ed25519 is the maintained reviewed
implementation surface; no new dependency, license, distribution channel, or
accessibility surface is selected. Developer experience is one documented Go
command and JSON receipt, usable without secrets or privileged setup.

## Evidence plan

### Primary sources

- [The Update Framework specification](https://theupdateframework.github.io/specification/v1.0.28/), accessed 2026-08-29.
- [SLSA Build Provenance v1.0](https://slsa.dev/spec/v1.0/provenance), accessed 2026-08-29.
- ADR-0004, ADR-0038, ADR-0054, and ADR-0055, accessed 2026-08-29.

### Experiment

From a clean checkout, run:

```powershell
$revision = git rev-parse HEAD
go run ./cmd/ardents-control simulate-public-control --source-revision $revision
```

Retain its JSON receipt
outside Git. It must identify the exact revision, report the versioned matrix,
and remain `simulation: true` and `qualified: false`; any other result
falsifies the run. The command creates fresh ephemeral identities and no
persistent authority.

### Failure scenarios

Under-threshold or expired emergency, escalated emergency, wrong predecessor,
Candidate View disagreement, forged audit signature, builder mismatch, and
each reader-matrix outcome must fail closed. A malicious local actor produces
the same forged/replayed/conflicting inputs as the reader matrix; degraded
input is `unavailable`; recovery uses the five explicit rotation reasons; and
governance failure is a receipt that lacks the Product Owner-selected contract
or exact source revision.

## Findings

- **Sourced fact:** threshold verification must count an authority key once and
  rotation needs continuity between predecessor and successor sets.
- **Sourced fact:** provenance binds build inputs and output subjects, but does
  not itself establish who operated a build boundary.
- **Measurement:** the accepted receipt records six passing mechanics cells and
  sixteen rejected failure cells for one caller-declared source revision.
- **Inference:** this is sufficient evidence for the selected internal
  simulation and insufficient for any future public-operation claim.

## Options

1. **Require unavailable external actors for H4-6C** — rejected. It has poor
   product fit for the actual two-person team, adds an unowned operational and
   governance dependency, and makes completion depend on recruitment rather
   than a reproducible contract. It provides a different public-claim security
   property but no additional simulation mechanics.
2. **Select simulated roles as a Public Beta candidate** — rejected. It has no
   security or product fit because simulation identities have no external
   operating boundary, and it would create an unsupported distribution and
   governance claim.
3. **Accept the bounded project-controlled simulation** — selected. It fits
   the Product Owner's team and security scope, has no external operational
   dependency, keeps authority non-persistent, and has low implementation risk
   because the standard library and existing reader own the whole surface.

## Recommendation

Choose option 3. Confidence is high because all stipulated mechanics and
failure paths are executable by the only relevant project team. The limitation
is explicit: this decision does not authorize a public claim.

## Disposition

**Decided for H4-6C.** ADR-0055 is the accepted decision; this record changes
the H4-6 product brief, scope, technical contract, command reference, package
map, research question/program, simulator and behavior tests. The maintained
implementation stays in `internal/publiccontrolsimulation`; its duplicated
experiment runbook was retired by the pre-H4-8 baseline inventory. There is no
accepted follow-up. A future public claim needs a new Product Owner decision;
it is not an open H4-6C task.
