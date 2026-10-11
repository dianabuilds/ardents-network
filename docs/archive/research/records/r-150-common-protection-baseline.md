---
id: R-150
title: Integrate protection into ordinary Ardents development
status: decided
owner: Product Owner and Codex
started: 2026-09-07
reviewed: 2026-09-07
---

# R-150 — How should one common protection baseline guide development?

## Decision this unlocks

Resolve whether the former Horizon 5 security/privacy intent should become a
separate stronger mode or guide the ordinary system's incremental development.
The Product Owner explicitly rejects parallel security modes and alternative
routes offered as different levels of protection. The objective is to make
ordinary Ardents use as protected as demonstrably feasible, through a full
product/technical review, integrated implementation and system-testing workstream.

This is a product and engineering contract clarification. It selects no new
protocol, experiment, C0 implementation slice or second active C0 research
execution. R-149 retains its separately selected design subject.

## Current contract

The [scope](../../product/scope.md), [functional map](../../product/functional-map.md),
[operating model](../../product/operating-model.md), and
[threat model](../../security/threat-model.md) own current requirements.
[Route/Node](../../technical/network-route-node.md) and
[Endpoint/Service](../../technical/endpoint-service-runtime.md) own the maintained
Interfaces. [ADR-0024](../../adr/0024-native-interactive-route-foundation.md),
as superseded for the affected selections, establishes authenticated Route
evolution and preserves Service Connection ownership. The C0 limitations,
owner boundaries, resource gates and existing compatibility obligations remain.

## Hypotheses

- **H1:** Common protection can evolve through the existing responsibility
  owners while Applications retain their connection contract. A design that
  makes an Application select weaker protection or manage Route internals
  falsifies this proposed integration boundary.
- **H2:** A separately selected stronger profile can satisfy the product goal.
  Requiring ordinary users to opt into that protection falsifies product fit.
- **H0:** A proposed mechanism cannot meet the common protection, useful
  operation and finite resource requirements together; none should be selected
  until the affected trade-off is resolved.

## Evaluation criteria

Require one ordinary product protection contract, coverage of the whole user
journey, preserved ownership, explicit failure and version migration, and a
maintainable bounded delivery process. Require maintained dependencies and a
fix or scoped non-applicability evidence for every known vulnerability in
current and proposed dependency closures, including build and test tools.
Keep the existing performance budgets;
any replacement needs its own researched decision. Protection improvement needs
predeclared adversary observations, an attack metric, uncertainty and a failure
threshold before an experiment. No numerical improvement is inferred here.

## Evidence plan

### Primary sources

Repository evidence inspected on **2026-09-07**:

- The Product Owner's instruction in this task: protection belongs to ordinary
  use of the whole system; parallel modes and alternative protection routes
  are not the intended product. The further instruction selects a complete
  product and technical stream through implementation and test-environment trials.
  The additional instruction requires current and future dependencies to remain
  maintained and free of known exploitable vulnerabilities in our use.
- The current owners linked above, including NET-07A/F, NET-13A, NET-14M and
  the research-to-implementation handoff in
  [documentation policy](../../development/documentation.md).
- [Route profile validation](../../../internal/route/wire_encoding.go) accepts
  only its exact `ardents-interactive-route-v2` value; the
  [Connection contract](../../../internal/service/connection/contract.go) binds
  that same profile. This is source inspection, not runtime qualification.
- Named historical provenance: commit `2376a9c975c413f4b64a9aa4f7875dae8bab8494`
  introduced Horizon 5; `git show 5b526367^:docs/product/horizon-5/README.md`
  recovers its final pre-retirement text. It deferred review until Public Beta
  evidence and contemplated a distinct stronger Route Profile. It is not a
  current requirement, implementation plan or support claim.

### Experiment

None selected or run. This decision compares product fit and existing owner
contracts, not anonymity mechanisms. A later mechanism comparison must define
its own falsification criteria before collecting traffic or running a spike.

### Failure scenarios

A successor must address unsafe bootstrap/refresh, combined source and Route
observations, name/reachability lookup, Application escape, key capture, active
traffic confirmation, overload, recovery, diagnostics, updates and incompatible
peers as applicable to its complete journey. Missing resources or evidence must
not select a weaker accepting path. Minimized observations must not themselves
create a new retained relationship or disclosure source.

## Findings

1. **Sourced fact:** the former separate-profile product option conflicts with
   the Product Owner's current direction. Its timing gate also cannot postpone
   security design or review for current work.
2. **Sourced fact:** the maintained Route reader already accepts one exact
   profile. The Application/Connection seam hides routing details and permits
   researched evolution; this task does not require a second routing system.
3. **Sourced fact:** current security owners cover enrollment, State, naming,
   Route, Application attachment, Custody and software lifecycle. A Route-only
   review cannot establish the whole-system result.
4. **Inference:** a bounded change to the responsible Module, qualified through
   its real consumers and the complete affected journey, offers a reviewable
   integration path without speculative abstractions or new shared authority.
5. **Assumption:** a useful stronger common baseline is feasible within some
   explicitly accepted budgets. No candidate or measurement yet establishes
   which additional protections satisfy that assumption.

## Options

| Option | Assessment |
|---|---|
| Preserve a separately selected stronger mode | Rejected by the Product Owner; ordinary use would retain a different protection level and require parallel lifecycle and qualification work. |
| Add a general security wrapper around existing behavior | Does not resolve owner-specific key, bootstrap, Application, resource or update obligations; a wrapper cannot inherit whole-system qualification. No such Module is justified here. |
| Evolve one common baseline through the responsible owners | Selected product direction. Each change keeps explicit privileges, versioned evidence and finite costs, and must pass complete-journey checks. A new cohesive Module remains possible only when a concrete responsibility justifies it. |

## Recommendation

Choose the common baseline and make security design, implementation and
verification part of each bounded product slice. Retain the small
Application/Service Connection Interface where its semantics remain adequate;
return a consequential Interface or authority change to design. Research may
compare alternatives, but supported product use does not become a privacy menu.

Confidence is high in product fit and the engineering ownership direction,
not in an unselected traffic-analysis defense. The strongest counterargument is
that one default must satisfy different workloads and constrained devices.
Evaluate complete latency, traffic, energy, memory, availability and maintenance
cost; revise a product requirement explicitly when necessary. Neither an
unusable mechanism nor an unmeasured protection claim meets the objective.

## Product and technical workstream

**Additional Product Owner direction, 2026-09-07:** undertake a full workstream
that revisits substantial product functionality and its technical design,
implements selected solutions and tests the resulting system on a test
environment. This is an end-to-end delivery objective, not just a recurring
review convention. The selected workflow and completion rule belong to
[documentation policy](../../development/documentation.md#system-protection-workstream).

The initial integration map is a reading and responsibility map, not an ordered
implementation backlog or evidence that any listed gap has been resolved:

| Area of ordinary use | Current owner and review focus |
|---|---|
| Installation, entry and current state | [Enrollment](../../technical/enrollment-verification.md) and [State/Source](../../technical/network-route-node.md): first trust, freshness, withheld or conflicting evidence, source observations and restarting without unsafe authority resurrection. |
| Names and finding a Service | [Naming](../../technical/naming.md) and [private reachability](../../technical/private-reachability.md): authorization, query/Target observations, colluding roles, freshness, denial and destination substitution across the complete lookup journey. |
| Publication and live connections | [Endpoint/Service](../../technical/endpoint-service-runtime.md) and [Route/Node](../../technical/network-route-node.md): exact owner and Target, key lifecycle, traffic correlation, combined observations, cancellation, recovery and finite terminal outcomes. |
| Local Application access | [Endpoint/Application](../../technical/endpoint-service-runtime.md) and R-099: scoped privileges and ordinary-network escape. The current generic attachment remains unqualified; a routing improvement cannot qualify local execution. |
| Shared decisions and private ownership | [Public operating model](../../product/operating-model.md#public-autonomy-target), [Custody](../../technical/release-update-custody.md) and R-149: valid effects, current evidence, capture, owner rights and the fixed no-disclosure boundary. |
| Contribution and hostile load | [Node/Resource](../../technical/network-route-node.md): aggregate work, admission, starvation, finite resource pressure and known cleanup through real process lifecycle. |
| Updates, diagnostics and leaving | [Release/Custody](../../technical/release-update-custody.md) and [operating model](../../product/operating-model.md): executable provenance, rollback, secret/metadata retention, recovery, withdrawal and owner-controlled adoption. |

The dependency lifecycle is part of this workstream. The
[dependency register](../../development/dependencies.md#maintenance-and-vulnerability-acceptance)
owns continued support, source integrity, advisory applicability and the
update/replacement boundary for existing and new components, including
transitive, toolchain, build and test dependencies. A runtime scan cannot
complete the maintenance or build-chain review. Its narrower baseline
observation is recorded below.

First establish this complete-operation coverage and dependency picture; then
select a bounded decision and implementation slice. An example slice boundary
is one protected connection establishment from current-State acquisition
through lookup and first useful Application bytes, including its refused and
interrupted paths. This is an illustration, not selection of a new C0 issue.
Define the required observable results before coding; evaluate the integrated
candidate afterward with real product commands and declared attack/failure
inputs. Controlled tests supply measured evidence for their actual environment,
while claims needing independent participation remain separately gated.

## Dependency baseline observation

**Measured 2026-09-07; bounded source evidence only.** The workspace's tracked
Go source and `go.mod`/`go.sum` matched commit
`0dd9fc09d9cf4938132d1ac43284c144cda10e1d`; documentation and research changes
were present, and no untracked Go file was included. SHA-256 identities:

- `go.mod`: `8e8751f110b25909ba41944827329974b1ab2fb2d3b577f3b8d95fac25e9e6e1`.
- `go.sum`: `5167264d35ddb70683f5b7a3fbc6cffac8fa0ec6cb15d72c1d4747a7976627a2`.

Go `1.26.6` and pinned `govulncheck v1.1.4` ran on Windows/amd64 against
`https://vuln.go.dev`; the database reported last update
`2026-09-02 19:12:04 UTC`. No extra build tags were supplied.

| Check | Configuration | Observed result |
|---|---|---|
| `make vuln`, followed by `govulncheck -show verbose ./...` | Windows/amd64, `CGO_ENABLED=1`, production source | Both exit 0; no symbol or imported-package finding; one module-only advisory. |
| `govulncheck -test -show verbose ./...` | Linux/amd64, `CGO_ENABLED=0`, production and test source | Exit 0; 26 scanned modules including the root, plus the standard library; no symbol or imported-package finding; the same module-only advisory. |
| `govulncheck -test -show verbose ./...` | Windows/amd64, `CGO_ENABLED=0`, production and test source | Exit 0; 24 scanned modules including the root, plus the standard library; no symbol or imported-package finding; the same module-only advisory. |
| `go list -buildvcs=false -deps -test ./...` | Each of Linux/amd64 and Windows/amd64, `CGO_ENABLED=0` | Both exit 0 without diagnostics; no `golang.org/x/crypto/openpgp` package or subpackage in either import closure. |

**Advisory applicability:** primary
[GO-2026-5932](https://pkg.go.dev/vuln/GO-2026-5932), accessed 2026-09-07,
identifies the unmaintained `golang.org/x/crypto/openpgp` package and its
subpackages, with no fixed version. The selected `x/crypto v0.56.0` module
contains that code, but the inspected production/test import closures exclude
it. This supports non-applicability of that advisory to those exact source
configurations. It neither labels all of `x/crypto` unmaintained nor accepts
OpenPGP use. The dependency owner must re-evaluate after any relevant import,
version, build/configuration, target or advisory change; a delivered artifact
needs its own inventory evidence.

**Method limits:** the
[pinned scanner documentation](https://pkg.go.dev/golang.org/x/vuln@v1.1.4/cmd/govulncheck)
and [Go vulnerability guidance](https://go.dev/doc/security/vuln/), accessed
2026-09-07, describe known-advisory analysis and its build/call-graph limits.
These source scans did not execute Linux or Windows product tests, inspect a
release artifact, audit every dependency's current maintenance, or check all
build tools and operating-system components. They are not a statement that
NET-30 is fully satisfied or that no unknown vulnerability exists.

The first import-list commands without `-buildvcs=false` returned exit 0 but
warned that writing VCS-derived main-module metadata to the external module
cache was denied. The import-only follow-ups disabled VCS stamping and produced
no diagnostic. This does not retry a failed test or qualify a changed build;
the original warning remains part of the evidence.

Raw scanner output, import lists and a scope receipt are outside the repository
in `C:\Users\vitek\AppData\Local\Temp\ardents-dependency-contract-025138c2cfb64c909101939ebde1a593`.
The `receipt.json` SHA-256 is
`115af61d5fb666eb6f7980abf6459c77855c383f7017c07599dcdbf9ea81a70b`.
The commands and bounded outcomes above are the durable observation; later
qualification must obtain fresh evidence for its own candidate.

## Disposition

**Decided: product direction and workstream structure**, accepted by the Product
Owner's clarification and recorded in
[ADR-0077](../../adr/0077-evolve-one-common-protection-baseline.md).
The operating model owns ordinary behavior and migration, NET-29 records the
common protection requirement, and NET-30 records dependency support and
vulnerability acceptance. The dependency register owns its review rules, the
threat model owns system coverage, and documentation policy owns slice handoff.
Scope, vision, glossary and repository instructions link or summarize those
owners. The source scan above supplies limited evidence for the existing
closure. No runtime, package, dependency version, wire identity, current
privacy claim or numeric performance gate changes. This closes the framing
question, not the security workstream or its future technical decisions and
implementation. No experiment artifacts exist.
