# Repository layout and growth rules

Status: **accepted architecture policy**

Accepted: 2026-08-09

Structure and file-ownership rules revised: 2026-09-27, following the Product
Owner's requirement for readable subsystem hierarchy and cohesion-based sizing.

This is the normative source for repository structure, Module growth, and Go
dependency direction. [ADR-0010](../adr/0010-modular-monorepository.md) records
the monorepository decision, while
[ADR-0009](../adr/0009-go-project-foundation.md) selects the single root Go
module. [ADR-0031](../adr/0031-retire-generic-live-test-tree.md) retires the
generic live-test tree while preserving an explicit future live-profile route.
[Product scope](../product/scope.md) remains authoritative for what may be
implemented now. The [package map](package-map.md) is a factual registry of
packages that exist; it is not a roadmap.

This policy defines how to organize and review code. Existing package names and
paths do not become permanent architecture by appearing in the package map.
Target component boundaries must be designed from supported behavior, state
ownership and lifecycle, then reflected in code and the map with each change.

Ardents may contain several executables and runtime trust zones without
becoming several repositories or Go modules. Process isolation, operating-
system privileges, deployment, and secret custody are runtime concerns. Source
co-location grants none of them access to another zone's authority material.

## Stable top-level zones

### Temporary successor source isolation

The Product Owner selected `internal/successor/` as a temporary grouping zone
for independently developed replacement components on 2026-10-02. It has no
root Go package and creates no runtime, wire or domain identity. The reserved
composition command path is `cmd/ardents-next`; it is added only with real
behavior. Source and test imports are restricted to the standard library and
the successor zone by the architecture gate. Existing product packages cannot
import the zone. Exact cross-package imports are registered in the package map;
directory nesting grants no implicit permission. The reviewed third-party
exceptions are the enumerated OpenTelemetry command/test imports and CIRCL
blindrsa in `admission/issuance`. The public `admission/issuerprofile` grammar
and Hosting use only the standard library. Node Identity depends only on that
public profile contract; it cannot import Admission's ledger or issuance owner.
Each real child package still requires the normal registration, behavior tests,
non-test caller and permitted imports. Existing contracts and qualification
obligations remain in force. See `internal/successor/README.md`.

| Zone | Purpose |
|---|---|
| `cmd/<name>/` | One real supported executable. It contains only CLI/configuration adaptation, Module startup, result presentation, and exit-code translation. |
| `internal/` | Maintained subsystems and their components. Paths express ownership and responsibility; Go packages expose the boundaries between components. Grouping directories may contain real child packages without Go files of their own. |
| `tests/` | Shared fixtures, checked execution-profile manifests, cross-process end-to-end tests, and explicit live-container tests. Unit and single-Module integration tests remain beside their implementation. This zone has no second Go module. |
| `docs/product/` | Accepted product promise, scope, functions, journeys, and operating model. |
| `docs/security/` | Threat model, claim conditions, adversaries, and honest limitations. |
| `docs/technical/` | Current subsystem behavior, protocols, ownership and lifecycle contracts. |
| `docs/operations/` | Operator procedures and supported environment requirements. |
| `docs/reference/` | Command and format references. |
| `docs/research/` | Open decision questions, the research template, and completed records retained as provenance. |
| `docs/adr/` | Accepted consequential decisions. Open questions and implementation progress do not belong here. |
| `docs/development/` | Normative engineering policy, factual registries, and developer runbooks. |
| `experiments/` | Optional disposable question-scoped research spikes and their instructions. The zone is absent when no active experiment exists and is never maintained product code. |
| `scripts/` | Thin, explicitly invoked bootstrap and developer wrappers. Product behavior remains in Go Modules. |
| `packaging/` | Source definitions for supported release bundles, images or operating-system packages. It contains no generated package output. |
| `deployments/` | Conditional environment/deployment definitions after production orchestration is selected. It is not created before a real environment owner exists. |
| `.github/workflows/` | Repository CI and authorized release automation. |
| `.githooks/` | Optional local developer checks; CI remains authoritative. |
| repository root | Project-wide policy and build entrypoints such as `AGENTS.md`, `README.md`, `CONTEXT.md`, `go.mod`, and `Makefile`. |

`packaging/`, `deployments/`, `experiments/`, and `tests/` are permitted locations, not
instructions to create empty directories. A new top-level zone requires a real
artifact, a responsibility not owned by an existing zone, and an architecture
review in the same change. Generated output has no repository zone.

## Current maintained trunk

Exact implemented package responsibilities and allowed project imports are
registered in the [package map](package-map.md). Inspect the working tree for
current files and local changes. A registry describes the implementation at its
revision; it does not justify retaining a boundary that obscures responsibility.

The [command inventory](command-surface.md#process-boundaries) owns the current
executable set and process responsibilities; the [command reference](../reference/commands.md)
owns syntax and behavior. This layout does not maintain a second command list.

Cross-process tests live under `tests/e2e/<behavior>/`. Selected host and
artifact qualification runners live under purpose-named directories in
`tests/qualification/`; each is an explicit active profile or a preparation
tool named for the domain behavior it exercises. Test-only fixture builders
remain `_test.go` implementation owned by the scenario that uses them. Images,
keys, state, captures, and generated manifests remain outside Git.

The root `Makefile` owns the canonical Go command-build policy. Canonical
artifacts disable implicit VCS stamping so identical selected source has the
same bytes across repeated normal clones and ownership extractions and a linked
worktree. Source
revision remains explicit external release and attestation provenance and is
not reconstructed from an artifact's local repository representation.
The build checks and command inventory define the command set they cover.

`tests/profiles/` owns the checked profile registry and positive package
membership manifests. Every maintained and Go-bearing e2e package belongs to
the one active profile appropriate to its surface; Qualification selection is
explicit rather than inferred from a directory name or a negative Make filter.
Retired profiles are removed from the registry and remain Git provenance only.

Unselected behavior does not earn a placeholder directory. Horizon
numbers and stage names must not appear in product package paths or product
command names. Immutable historical evidence may retain its exact candidate
identity; the package map is the executable current-state import policy.

End-to-end and live tests drive product Interfaces and commands but cannot
implement missing product behavior on their behalf. A passing harness shortcut
is a test failure. Test runs are independent: no test consumes a receipt from a
previous run or requires a stage/profile selector.

A disposable Go spike under `experiments/` uses `//go:build ignore` so the root
module and its `./...` quality gates do not treat it as maintained project code.
It does not create a nested `go.mod`; its question record and README own the run
instructions and disposition.

## Subsystem hierarchy

Start from supported operations: identify the state and resources each operation
uses, who owns their lifetime, and which components collaborate. Place those
components under the subsystem that owns them. The tree should help a reader
find an operation's implementation, dependencies and cleanup without knowing
historical prefixes or task identifiers.

- A subsystem can contain a composition package and child component packages,
  or be a grouping directory containing its real components. The parent does
  not need several pre-existing packages before its first child can be extracted.
- Keep composition focused on construction, connecting component contracts and
  coordinated startup/shutdown. Component state machines and private storage
  belong with their owners.
- Choose nesting from actual ownership and reader navigation. A component with
  one caller can still warrant its own package. Multiple consumers do not by
  themselves justify moving a component to the root of `internal/`.
- Keep a shared component in a location that explains its responsibility and
  dependencies. Avoid generic shared-code buckets and forwarding-only layers.
- Directory depth, file count and line count have no target values. Check that
  the structure exposes responsibilities and keeps related behavior together.
- Folder structure does not dictate a chain of imports. Make dependencies
  explicit, avoid cycles, and place assembly where it can connect components
  without making them depend on the assembly owner.

Describe a proposed restructuring with its target tree, component contracts,
state/lifecycle ownership and dependency direction. Map existing code to those
responsibilities. Keep the proposal distinct from the implemented package map
until each extraction or relocation is realized.

## Commands and packages

A new `cmd/<name>` is justified only when a separately runnable supported
behavior has its own invocation, lifecycle, configuration, and exit contract.
The command may parse CLI input, load configuration, construct selected
Adapters, call one or more Modules, render a bounded result, and select an exit
code. Domain state machines, protocol behavior, evidence policy, retry logic,
and security decisions stay in `internal` Modules. The architecture gate
verifies that a command exposes no exported product behavior; semantic review
decides whether it remains a thin adapter over its actual Module boundary.

A new package, at any depth under `internal/`, is justified when all are true:

1. one cohesive responsibility can be stated without `and everything else`;
2. callers need a small Interface with explicit invariants, errors, operation
   order, resource limits, and operating conditions;
3. a real maintained Implementation and tests exist in the same change;
4. the boundary improves responsibility, encapsulation or navigation, with
   dependencies and exported surface proportionate to that benefit;
5. its name and permitted project imports are registered in `package-map.md`.

A file split, shared type, repeated two-line helper, organizational symmetry,
future roadmap item, or possible technology replacement is not enough. Do not
create generic directories or packages named `util`, `common`, `misc`, `types`,
`interfaces`, `api`, `services`, `models`, `adapters`, `src`, `pkg`, or `sdk`.
Do not encode a transport, storage engine, cryptographic suite, or other
unselected technology in a Module name.

Review an existing package for division when responsibility or navigation is
unclear. Useful evidence includes:

1. callers use disjoint parts of its Interface;
2. the parts own independent state, invariants, lifecycle, or failure policy;
3. one part introduces dependencies irrelevant to the other;
4. tests must reach past the Interface to isolate one part;
5. the package Interface has become a union of unrelated operations.
6. following one operation requires searching unrelated file clusters, while
   those clusters already have identifiable responsibilities.

Line count, file count, filename prefixes, or a future second Adapter do not by
themselves create a package Seam. A division moves complete behavior and its
tests; it does not introduce a global shared-types or helper package.

A directory containing maintained Go files is a Go package; parent and child
packages share no private implementation. A grouping directory can contain real
registered child packages without Go files of its own. Choose a child package
when it belongs to its enclosing subsystem and meets the package rules above.
Do not force a cohesive component into another file or a top-level sibling
merely because its parent has no other child yet.

Every new package, including a nested package, must arrive in one change with:

1. `doc.go` stating the single owned responsibility;
2. a maintained Implementation rather than placeholders or forwarding wrappers;
3. behavior tests at the package Interface;
4. at least one maintained non-test caller;
5. one `package-map.md` row naming its exact permitted project imports;
6. command ownership updated when a command crosses the new Seam.

Directory nesting grants no privileged dependency. The package map states the
direction explicitly, and the architecture gate rejects any undeclared import.

## Go code and review rules

These rules apply to every maintained Go change and are enforced where
possible by `internal/architecture`, Make, the Git hook, and CI.

- Keep exported interfaces small and implementation details unexported.
- Return errors with actionable context; never hide errors or use `panic` for
  first-party control flow. First-party `panic`, `unsafe`, cgo, and implicit
  `init` require a superseding accepted ADR and dedicated risk tests. The
  [scoped exception register](scoped-risk-exceptions.md) owns the exact
  admitted high-risk source bindings it covers.
- Use `gofmt`, mandatory package comments, and thin command adapters.
- Add behavior and failure-path tests with the owning change.
- Prefer the standard library. Review every third-party runtime dependency in
  [dependencies.md](dependencies.md) before changing `go.mod`.
- Keep caches, generated evidence, credentials, and artifacts outside Git under
  the [artifact rules](#generated-and-sensitive-artifacts).

Line counts, exported-declaration counts, broad records, direct clock use,
string outcomes, and duplication are review signals, not verdicts. Evaluate
responsibility, caller knowledge, state/lifecycle ownership, failure and
cleanup, format observers, and behavior evidence together. A review records
the local invariant, why an apparent split would add caller coordination, the
real caller/compatibility boundary, and normal and failure coverage. Do not
split a cohesive invariant, widen a result record, or add a generic helper just
to satisfy a superficial metric. The import rules below remain hard gates;
file size is assessed through the ownership and readability review below.

The [official Go layout guidance](https://go.dev/doc/modules/layout) and
[Go Code Review Comments](https://go.dev/wiki/CodeReviewComments) supply the
starting conventions. Automated analysis augments design review.

## Go file ownership and size

A Go file is an implementation navigation unit, not a Module. Its name states
one responsibility or one responsibility plus an aspect: for example
`compose_smoke.go`, `compose_evidence.go`, `tooling/role.go`, and
`tooling/role_runtime.go`. Tests use the corresponding responsibility name.
`doc.go` contains only the package comment.

- There is no fixed line limit for Go files, including tests. Choose file size
  according to responsibility and readability rather than a numerical budget.
- A file is divided at independently varying cohesive type/function clusters,
  not merely at a line threshold. Division does not justify another package or
  exported symbol.
- Keep related state transitions and invariants readable together. Split when
  independent responsibilities, lifetimes or reasons to change obscure that
  reading; retain a larger file when splitting would scatter one coherent
  operation and increase navigation or caller coordination. An 800-line file
  can be justified on this basis; that number is not a new limit or exemption.
- When cohesion is non-obvious, briefly record the responsibility, relevant
  invariants and the keep/split trade-off in the change description or existing
  owner document. Refer to existing behavior evidence where relevant. Do not
  create per-file size approvals, exception registries or threshold paperwork.
- Review package structure as well as files. Subsystems should be discoverable
  in the directory tree. Repeated long prefixes or scattered owner methods may
  indicate a missing component boundary; neither flatness nor nesting depth is
  a goal. Cohesive subpackages must meet the package requirements above.
- Catch-all filenames `model.go`, `support.go`, `types.go`, `helpers.go`,
  `common.go`, `misc.go`, and `util.go` are forbidden.

The architecture gate enforces the facts it can prove: command adaptation
without exported product behavior, package-map and
import direction, and forbidden filenames. Semantic responsibility remains a
review rule because a mechanical line count cannot determine cohesion or a
correct Seam.

## Module, Interface, Implementation, Seam, and Adapter

- A **Module** owns one coherent responsibility and hides substantial behavior
  behind one small Interface.
- An **Interface** is everything callers must know: Go surface, invariants,
  error modes, operation order, resource bounds, and operating conditions.
- An **Implementation** is the behavior hidden inside the Module.
- A **Seam** is the place where behavior can be changed without editing the
  caller through an explicit component contract.
- An **Adapter** is a concrete implementation role at a Seam. It is not a
  generic package category.

Place an Interface beside the behavior owner or the caller that consumes it,
not in a global `interfaces` package. Callers and tests use the same Interface.
An internal test seam may remain unexported. Introduce a Go interface for a
concrete caller's dependency or an actual component boundary; specify only the
operations the consumer needs. One production implementation is sufficient when
the interface makes that boundary explicit. Use a concrete type when it expresses
the contract adequately. Do not invent extra implementations to justify an
interface or generalize for hypothetical future technology. Test implementations
must exercise the same contract without exposing private component state.

## Dependency direction

The [package map](package-map.md) is the executable current-state dependency
policy. It names each permitted first-party import; a diagram cannot grant a
dependency that the map does not name. Commands adapt owned Modules, and
product Modules never import `tests/`, `experiments/`, or `scripts/`. Cyclic
project imports are forbidden.

## Experiment promotion lifecycle

An active experiment owns its fixtures, orchestration, fault injection,
bounded observations, evidence finalization, and cleanup. It may call a
maintained Module through that Module's Interface. The dependency never
reverses.

When experiment behavior is proven and the Product Owner promotes a maintained
slice:

1. record the evidence and promotion decision;
2. define the smallest product Interface from the accepted behavior rather
   than from experiment topology or tooling;
3. place the maintained Implementation in its owning product Module and use it
   from both the product caller and tests through the same Interface;
4. update the factual package map, dependency tests, and product records;
5. retain or remove experiment code according to its evidence disposition.

Promotion is not a wholesale copy of an experiment directory. A maintained
Module must not import experiment configuration, evidence schemas, fault
controls, Docker assumptions, or experiment state.

## Tests and test data

- Unit tests and integration tests contained within one Module live beside its
  implementation as `*_test.go` and exercise the Module Interface.
- A test crossing several Modules or processes lives under
  `tests/e2e/<behavior>/` only when the real cross-boundary behavior is
  implemented. It uses the root module and owns no product Implementation.
- A selected real-container network test owns a purpose-named test boundary,
  explicit profile selection, its complete lifecycle, and an independently
  runnable command. No generic `tests/live/` tree or build tag exists by
  default.
- `testdata/` lives directly below the Module, command, or e2e test that owns
  it. Test surfaces do not import fixtures or golden evidence from each other.
- Test Adapters satisfy the same Interface as real callers. Tests do not reach
  through a Seam to assert private implementation state.

Empty test trees, future profile fixtures, and speculative mocks are not
created. Generated test evidence follows the artifact rules below.

## Docker, infrastructure, and packaging

Packaging and test orchestration describe their actual supported environment
and invocation contracts in their owning documents. Their presence does not
establish production qualification or authorize a second runtime implementation.

Release bundle and operating-system package definitions belong under
`packaging/<target>/`; image definitions use
`packaging/images/<name>/`. Generated images, archives, installers, SBOMs, and
signatures remain outside the repository. A second image alone does not select
production packaging.

Environment-specific infrastructure or orchestration source belongs under
`deployments/<environment>/` only after an accepted delivery decision chooses
its ownership and lifecycle. Product logic never moves into Dockerfiles,
Compose, deployment templates, CI, or shell wrappers.

`scripts/` contains only thin bootstrap or developer wrappers that validate
inputs and call maintained Go behavior or an explicit tool. Scripts do not
become importable product dependencies, install tools implicitly, or hide a
second application runtime.

## Generated and sensitive artifacts

Generated evidence, packet captures, private keys, reusable credentials,
authority material, dependency caches, databases, compiled binaries, profiles,
coverage, images, installers, SBOM output, and temporary runtime state stay
outside the repository in an owned system-temporary or explicitly chosen
external evidence location. Cleanup validates the exact owned path before
removal.

Hand-authored, non-sensitive fixtures may be committed under the owning
`testdata/`; a public certificate or public-key fixture is permitted, but a
private, encrypted, or secret key is not. Generated source requires a
separately accepted need, a reproducible pinned generator, and a review of
whether retaining output is necessary; none is authorized currently.
`.gitignore`, `.dockerignore`, and the architecture gate are guardrails, not
permission to place sensitive material in an ignored path.

## Extraction into another repository

Ardents does not plan a first-party repository split before the Closed Test
Network. After that horizon, extraction requires a superseding ADR and all of
the following:

1. a real independently released or externally consumed Interface, materially
   different access/disclosure policy, or independently operated lifecycle;
2. a narrow versioned compatibility and threat contract that can be tested
   without importing the parent repository's source;
3. independent build, dependency, vulnerability, and release gates;
4. no circular source dependency and a bounded migration/rollback plan;
5. an owner and maintenance cost that the actual Product Owner plus Codex team
   can sustain;
6. measured benefit greater than atomic-change and coordination costs.

Separate binaries, runtime trust zones, languages used by external tools,
directory size, or aesthetic symmetry are not extraction criteria. Secret or
authority isolation is achieved by never storing that material here, not by
moving source code to another repository.

## Executable versus documentary rules

The architecture gate automatically checks the single root module, factual
package-map registration, permitted current imports, forbidden generic package
names, command adaptation, Go source placement, absence of product imports
from test/experiment/script code, formatting, selected unsafe constructs,
and common generated-artifact patterns. The Make targets add vet, tests, build,
module tidiness, race, Staticcheck, and vulnerability analysis.

Human review remains responsible for cohesive responsibility, Interface depth,
whether a component contract serves actual callers, product meaning of a dependency,
technology-neutral naming, delivery-horizon authorization, placement of
non-Go infrastructure, experiment promotion, and repository extraction. Those
decisions cannot be inferred safely from path names or line counts.
