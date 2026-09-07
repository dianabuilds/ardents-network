---
id: R-151
title: Map development of the successor common privacy and anonymity scheme
status: decided
owner: Product Owner and Codex
started: 2026-09-07
reviewed: 2026-09-07
---

# R-151 — What must change to develop the successor privacy and anonymity scheme?

## Decision this unlocks

The Product Owner requests a complete integration map, explicitly planned for
developing and implementing a new privacy/anonymity scheme in ordinary Ardents
use. The decided result is the scope, dependencies and evidence route for that
work. It does not decide the actual scheme or qualify any implementation.

## Current contract

[ADR-0077](../../adr/0077-evolve-one-common-protection-baseline.md), the current
[scope](../../product/scope.md),
[threat model](../../security/threat-model.md),
[common protection baseline](../../product/operating-model.md#common-protection-baseline),
[dependency policy](../../development/dependencies.md) and
[development workflow](../../development/documentation.md#system-protection-workstream)
govern this work. NET-29/30 select common protection and dependency acceptance.
Current [technical owners](../../development/package-map.md) provide the
source responsibility map. Existing C0 selections, public/independence limits,
wire/state compatibility and numeric budgets remain until explicit replacement.

## Hypotheses

- **H1:** A dependency-ordered set of product/architecture decisions and complete
  operation slices can evolve one ordinary baseline through maintained owners.
- **H2:** Replacing the Route or adding a privacy library alone is sufficient.
  A bypass or linkable observation elsewhere in an admitted operation falsifies it.
- **H0:** No available composition meets the desired privacy, useful operation,
  deployment and maintenance conditions; requirements must be revisited.

## Evaluation criteria

The map must cover all maintained product owners and ordinary lifecycle paths,
distinguish present code from claims, trace candidate choices to acceptance
evidence, enumerate blocking decisions and library/removal work, and provide
a bounded implementation and system-test sequence for one Product Owner and
Codex. No success threshold, rewrite percentage, anonymity population or
implementation-ready protocol is inferred from preparing the map.

## Evidence plan

### Primary sources

Accessed **2026-09-07**:

- The Product Owner's request in this task for the complete map on the premise
  that a successor privacy/anonymity scheme will be developed and implemented.
- Current Network/Route/Node, Endpoint/Service, Naming, private reachability,
  Transit acquisition, enrollment and Release/Custody technical owners; the
  current product/threat/development owners linked above.
- Root `go.mod`, `cmd`/`internal` Go source paths,
  `.github/workflows/quality.yml`, dependency register and checked test profiles.
- [Loopix, USENIX Security 2017](https://www.usenix.org/conference/usenixsecurity17/technical-sessions/presentation/piotrowska):
  a research reference for mixing and cover, with its own workload/assumptions.
- [Comprehensive Anonymity Trilemma, PoPETs 2020](https://petsymposium.org/popets/2020/popets-2020-0056.php):
  motivation to compare privacy, delay and bandwidth within stated models.
- [Tor padding specification](https://spec.torproject.org/padding-spec/index.html)
  and [Arti](https://arti.torproject.org/): design/reference inputs, not adopted runtimes.
- [Katzenpost source](https://github.com/katzenpost/katzenpost) and
  [specification index](https://katzenpost.network/docs/specs/):
  possible implementation investigation, including license and architecture gates.
- [RFC 9458](https://www.rfc-editor.org/rfc/rfc9458.html):
  the OHTTP role/encapsulation contract underlying current resolution.
- The earlier [dated dependency source check](r-150-common-protection-baseline.md#dependency-baseline-observation)
  is named bounded provenance, not a fresh complete maintenance or artifact audit.

### Experiment

No anonymity mechanism experiment, target-platform trial or benchmark was run.
The map defines D1–D10 decisions and E1–E12 evidence groups so actual experiments
can declare falsification and cost limits before execution.

A read-only source walk used `rg --files cmd internal tests -g '*.go'`,
selected command/internal directories, and compared those package paths and
the root module requirements with the final map. The source revision was
`0dd9fc09d9cf4938132d1ac43284c144cda10e1d`; unrelated documentation/research
changes were present. The inventory receipt is outside the repository at
`C:\Users\vitek\AppData\Local\Temp\ardents-privacy-map-03364db94b0d489e8a6dc9761b76e4cd\inventory.json`.
This measures coverage, not implementation security.

Documentation verification matched all 41 package paths and 25 module
requirements to the map and validated its local link destinations. The existing
`go test ./internal/architecture` check and `git diff --check` passed. No Go
source or module file was changed by this mapping work. These checks do not
exercise the proposed scheme or add anonymity evidence.

### Failure scenarios

Combined edge/control/infrastructure observations, active correlation, sparse
traffic, Sybil capture, discovery/bootstrap links, Application escape, secret
and diagnostic leakage, overload, restart, upgrade/downgrade and unsupported
dependency use have explicit owners and future test obligations in the map.
Current correlation exclusions cannot become a passing successor result.

## Findings

1. **Sourced fact:** current Route/Connection accept one exact interactive
   profile; the threat contract excludes broad-observer and sufficient-collusion
   correlation from that current claim. A successor needs a new claim and evidence.
2. **Sourced fact:** current generic Application attachment is unqualified for
   isolation; complete Name runtime composition is also missing. A Route-only
   change cannot establish whole-journey protection.
3. **Sourced fact:** current Source, admission, publication, recovery and update
   lifecycles have distinct identities and observations. Their composition must
   be included in anonymity analysis without merging their authority.
4. **Measurement:** the maintained command/internal source walk finds 41 package
   directories; the current root module declares 25 third-party requirements.
   The promoted map accounts for all of them and separately covers test/build
   inputs. These are inventory counts, not a rewrite estimate.
5. **Inference:** entry/admission, reachability, packet/scheduling, Route and
   Connection are the most tightly coupled redesign region. Existing Custody
   and Release separation can remain useful if revalidated.
6. **Sourced fact:** research systems and reusable implementations bring their
   own workload, runtime, directory and distribution conditions. A familiar
   mechanism name or a current repository cannot close Ardents's selection gate.
7. **Assumption:** some complete successor may meet a useful explicitly accepted
   privacy/cost contract. This mapping effort does not demonstrate that feasibility.

## Options

| Option | Assessment |
|---|---|
| Dependency update campaign alone | Useful maintenance but insufficient for the requested successor anonymity scheme. |
| Replace Route first and attach other behavior later without common decisions | Leaves bootstrap, control, local execution and reliability gaps; not a complete selected composition. |
| Establish coherent decisions, then integrate through complete operation slices | Selected development approach. Allows significant redesign, bounded evidence and honest rejection without permanent parallel modes. |
| Select a ready-made full anonymity stack immediately | Rejected at this stage: runtime, authority, license, storage and Service semantics need explicit assessment. |

## Recommendation

Use the [current integration map](../../development/privacy-anonymity-map.md):
18 areas, 10 coupled design gates, 11 conditional work packages, 12 evidence
groups and complete current package/module coverage. Start the next selected
design brief with D1's claims, workload and cost contract, then close joint
feasibility before maintained implementation.

Confidence is high in coverage and the need for composed evidence; confidence
in any unselected mechanism or rewrite estimate is not established. The main
counterargument is the cost of changing coupled system behavior with one human
and Codex. Keep the product contract small and reject compositions whose
maintenance or traffic costs the actual team cannot sustain.

## Disposition

**Decided: integration mapping only.** The current development map owns the
coverage and dependency route. Product operating model and NET-29 state the
successor direction; security, dependency and testing owners link the map with
honest claim/evidence limits. No new ADR, library, package, runtime, wire,
numeric budget, C0 implementation issue or additional active C0 research
execution is selected. No mechanism experiment code exists to retain.
The scheme's D gates remain unresolved; completing this record does not
complete design, implementation or anonymity qualification.
