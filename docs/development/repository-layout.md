# Repository layout and growth rules

Status: **accepted architecture policy**

Accepted: 2026-08-09

This is the normative source for repository structure, Module growth, and Go
dependency direction. [ADR-0010](../adr/0010-modular-monorepository.md) records
the monorepository decision, while
[ADR-0009](../adr/0009-go-project-foundation.md) selects the single root Go
module. [Product scope](../product/scope.md) remains authoritative for what may
be implemented now. The [package map](package-map.md) is a factual registry of
packages that exist; it is not a roadmap.

Ardents may contain several executables and runtime trust zones without
becoming several repositories or Go modules. Process isolation, operating-
system privileges, deployment, and secret custody are runtime concerns. Source
co-location grants none of them access to another zone's authority material.

## Stable top-level zones

| Zone | Purpose |
|---|---|
| `cmd/<name>/` | One real supported executable. It contains only CLI/configuration adaptation, Module startup, result presentation, and exit-code translation. |
| `internal/<domain>/` | Maintained cohesive deep Modules and their implementation. A directory exists only with real behavior and an owned Interface. |
| `tests/` | Shared fixtures, checked execution-profile manifests, cross-process end-to-end tests, and explicit live-container tests. Unit and single-Module integration tests remain beside their implementation. This zone has no second Go module. |
| `docs/product/` | Accepted product promise, delivery horizons, functions, journeys, and operating model. |
| `docs/security/` | Threat model, claim conditions, adversaries, and honest limitations. |
| `docs/research/` | Decision-relevant questions, completed evidence records, and the research template. |
| `docs/adr/` | Accepted consequential decisions. Open questions and implementation progress do not belong here. |
| `docs/development/` | Normative engineering policy, factual registries, and developer runbooks. |
| `experiments/` | Disposable question-scoped research spikes and their instructions. It is not a maintained project tree. |
| `scripts/` | Thin, explicitly invoked bootstrap and developer wrappers. Product behavior remains in Go Modules. |
| `packaging/` | Conditional source definitions for distributable images or operating-system packages after a delivery gate authorizes them. It contains no generated package output. |
| `deployments/` | Conditional environment/deployment definitions after production orchestration is selected. It is not created for Carrier Lab. |
| `.github/workflows/` | Repository CI and release automation after the applicable horizon authorizes it. |
| `.githooks/` | Optional local developer checks; CI remains authoritative. |
| repository root | Project-wide policy and build entrypoints such as `AGENTS.md`, `README.md`, `CONTEXT.md`, `go.mod`, and `Makefile`. |

`packaging/`, `deployments/`, and `tests/` are permitted locations, not
instructions to create empty directories. A new top-level zone requires a real
artifact, a responsibility not owned by an existing zone, and an architecture
review in the same change. Generated output has no repository zone.

R-091 removed the closed `lab/` and `internal/lab/` trees. Their records remain
under `docs/research/`; a future experiment must create a new approved,
purpose-named boundary rather than restore this historical corpus.

## Horizon 3 product trunk

Horizon 3 starts the maintained product from the accepted records; it does not
restore a closed laboratory command or package.

The first real product commands are:

| Command | Stable responsibility |
|---|---|
| `cmd/ardents` | Run the local Endpoint process. It grows only through accepted Endpoint capabilities and remains a thin adapter over product Modules. |
| `cmd/ardents-node` | Run one separately configured Contributor Node identity and one active role per process. Co-resident roles require distinct processes, keys, state, and resource ownership. |

The maintained product Modules are:

| Module path | Stable responsibility |
|---|---|
| `internal/network/state` | Own authenticated Network State acceptance, Epoch/View verification, current and pending decisions, acquisition, durable publication, and restart recovery. |
| `internal/network/source` | Own the finite Direct-Origin Source plan, credential binding, private transport, and exposure identity. |
| `internal/network/duty` | Own durable Endpoint-local Role Domain duty, watermark, expiry, and identity/family conflict truth. |
| `internal/node` | Bind one local Node identity to authenticated assignment, private role-probe TLS/framing/replay/capacity, readiness, duty, drain, withdrawal, and terminal cleanup. |
| `internal/resource` | Own bounded OS/runtime measurement, process placement, hysteresis, and pressure decisions shared by State and Node; each consumer owns its reaction. |

Cross-process tests live under `tests/e2e/<behavior>/`. Live Docker inputs and
their build-tagged Go tests live under `tests/live/`. Test-only fixture builders
remain `_test.go` implementation owned by the scenario that uses them. Images,
keys, state, captures, and generated manifests remain outside Git.

`tests/profiles/` owns the checked profile registry and positive package
membership manifests. Every maintained and Go-bearing e2e package belongs to
the one active profile appropriate to its surface; live and future
Qualification selection is explicit rather than inferred from a directory name
or a negative Make filter. Inactive profiles record the decision required to
activate them and are not passing evidence.

This is a trunk, not a complete future directory tree. Route, Carrier,
Publication, Service Connection, Namespace, Bridge, Release Safety, platform,
and Application Interface paths do not exist until their promoted vertical
slice supplies real maintained behavior. Horizon numbers and stage names never
appear in product package paths or product command names.

The product import direction is:

```text
cmd/ardents -> internal/network/state, internal/network/source
cmd/ardents-node -> internal/network/state, internal/network/source, internal/node
internal/network/state -> internal/network/duty, internal/network/source, internal/resource
internal/node -> internal/network/duty, internal/resource
```

End-to-end and live tests drive product Interfaces and commands but cannot
implement missing product behavior on their behalf. A passing harness shortcut
is a test failure. Test runs are independent: no test consumes a receipt from a
previous run or requires a stage/profile selector.

A disposable Go spike under `experiments/` uses `//go:build ignore` so the root
module and its `./...` quality gates do not treat it as maintained project code.
It does not create a nested `go.mod`; its question record and README own the run
instructions and disposition.

## Current factual structure

The maintained tree at the time of this decision is:

```text
cmd/
internal/
  architecture/                executable repository and quality rules
scripts/
  check-tools.go               build-ignored developer tool-version check
  install-git-hooks.sh         local hook bootstrap
.github/workflows/
  quality.yml                  mandatory ordinary CI quality gate
.githooks/pre-commit           local quick gate
docs/                          product, security, research, ADR, and development records
experiments/README.md          policy for future disposable spikes
go.mod                         the only Go module
Makefile                       common build and quality entrypoints
```

Only the Go packages listed in [package-map.md](package-map.md) exist as
maintained packages. R-091 removed the closed Carrier and Gate C execution
tree; its source-bound results remain research provenance rather than current
directories, commands, imports, or deployment inputs.

## Conditional target map and delivery horizons

The following names are logical product Modules, not approved Go package names
or directories:

- Endpoint Runtime;
- Destination;
- Service Connection;
- Publication;
- Route;
- Carrier Channel;
- Infrastructure Node;
- Service Authority Custody;
- Namespace;
- Network State;
- Release Safety;
- Platform capabilities;
- Carrier Lab and verification.

They guide future placement only after product evidence promotes their behavior:

| Delivery horizon | Permitted growth after its gate |
|---|---|
| Closed Carrier Lab and Named Unlisted Site | Frozen Modules, commands, fixtures, and checks required to reproduce the accepted evidence. No later-horizon product behavior grows inside them. |
| Closed Test Network (current) | Concrete Infrastructure Node, Network State, private resolution/Namespace, public-role process separation, and expanded custody behavior, one promoted vertical slice at a time. |
| Public Beta (conditional) | Release Safety, supported platform packaging, deployment definitions, and complete qualification behavior only after their individual gates and technology decisions. |

The logical map does not require every item to become one package. A cohesive
Module may own several closely related behaviors behind one small Interface;
one broad concept may later require several Modules after evidence reveals real
independent seams. The package name is chosen from implemented responsibility,
not copied mechanically from this list. No directory is created until the first
maintained Implementation and tests arrive in the same change.

## Commands and packages

A new `cmd/<name>` is justified only when a separately runnable supported
behavior has its own invocation, lifecycle, configuration, and exit contract.
The command may parse CLI input, load configuration, construct selected
Adapters, call one or more Modules, render a bounded result, and select an exit
code. Domain state machines, protocol behavior, evidence policy, retry logic,
and security decisions stay in `internal` Modules. The architecture gate
verifies that a command exposes no exported product behavior; semantic review
decides whether it remains a thin adapter over its actual Module boundary.

A new `internal/<domain>` package is justified only when all are true:

1. one cohesive responsibility can be stated without `and everything else`;
2. callers need a small Interface with explicit invariants, errors, operation
   order, resource limits, and operating conditions;
3. a real maintained Implementation and tests exist in the same change;
4. the package provides leverage and locality that another source file in an
   existing Module would not;
5. its name and permitted project imports are registered in `package-map.md`.

A file split, shared type, repeated two-line helper, organizational symmetry,
future roadmap item, or possible technology replacement is not enough. Do not
create generic directories or packages named `util`, `common`, `misc`, `types`,
`interfaces`, `api`, `services`, `models`, `adapters`, `src`, `pkg`, or `sdk`.
Do not encode a transport, storage engine, cryptographic suite, or other
unselected technology in a Module name.

An existing package is considered for division only when its implementation
has at least two independent reasons to change and one or more of the following
is observable:

1. callers use disjoint parts of its Interface;
2. the parts own independent state, invariants, lifecycle, or failure policy;
3. one part introduces dependencies irrelevant to the other;
4. tests must reach past the Interface to isolate one part;
5. the package Interface has become a union of unrelated operations.

Line count, file count, filename prefixes, or a future second Adapter do not by
themselves create a package Seam. A division moves complete behavior and its
tests; it does not introduce a global shared-types or helper package.

A nested directory is a full Go package, not an organizational folder. Parent
and child packages share no private implementation. `internal/<owner>/<module>`
is allowed only when `<owner>` already owns several real Modules and the child
independently meets every package rule above; otherwise use another file in the
owning package or a factual sibling `internal/<module>`.

Every new package, including a nested package, must arrive in one change with:

1. `doc.go` stating the single owned responsibility;
2. a maintained Implementation rather than placeholders or forwarding wrappers;
3. behavior tests at the package Interface;
4. at least one maintained non-test caller;
5. one `package-map.md` row naming its exact permitted project imports;
6. command ownership updated when a command crosses the new Seam.

Directory nesting grants no privileged dependency. The package map states the
direction explicitly, and the architecture gate rejects any undeclared import.
## Go file ownership and size

A Go file is an implementation navigation unit, not a Module. Its name states
one responsibility or one responsibility plus an aspect: for example
`compose_smoke.go`, `compose_evidence.go`, `tooling/role.go`, and
`tooling/role_runtime.go`. Tests use the corresponding responsibility name.
`doc.go` contains only the package comment.

- Every Go file, including tests, has an interim hard maximum of 500 lines.
- A file is divided at independently varying cohesive type/function clusters,
  not merely at a line threshold. Division does not justify another package or
  exported symbol.
- When a file's size or complexity makes cohesion non-obvious, record its one
  named responsibility, invariant locality, rejected obvious split, behavior
  tests, and real hotspot signals. There is no soft line-count threshold.
- Catch-all filenames `model.go`, `support.go`, `types.go`, `helpers.go`,
  `common.go`, `misc.go`, and `util.go` are forbidden.

The architecture gate enforces the facts it can prove: the interim 500-line
limit, command adaptation without exported product behavior, package-map and
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
  caller. It exists only when variation is real.
- An **Adapter** is a concrete implementation role at a Seam. It is not a
  generic package category.

Place an Interface beside the behavior owner or the caller that consumes it,
not in a global `interfaces` package. Callers and tests use the same Interface.
An internal test seam may remain unexported. A general transport, storage, or
cryptography Interface is forbidden until a real Seam and at least two
justified Adapters exist. A test Adapter may justify a Seam only when it stands
in for an actual replaceable dependency; it must not expose implementation
details merely to make tests convenient.

## Dependency direction

An arrow means the source may depend on the target. It does not require the
dependency to exist, nor does it authorize a package:

```text
cmd
  -> Endpoint Runtime
       -> Destination
       -> Service Connection
            -> Route
                 -> Carrier Channel
       -> Publication
       -> Platform capabilities

Infrastructure Node
  -> Route
  -> Carrier Channel
  -> Network State
  -> Platform capabilities

Service Authority Custody
  -> Platform capabilities

Carrier Lab / verification
  -> proven product Modules
```

The following directions are forbidden:

- Route -> Endpoint Runtime;
- Carrier Channel -> Route, Application Interface, or Namespace;
- Service Connection -> Namespace;
- Service Authority Custody -> Endpoint Runtime;
- Infrastructure Node -> User-facing or Application-specific Modules;
- product Modules -> Carrier Lab, test harnesses, `experiments`, or
  `scripts`;
- Ardents core -> an optional Overlay or Application;
- cyclic project imports.

The package map is the executable current-state dependency policy. The logical
future map remains documentary until both endpoint packages exist; tests do not
pretend that hypothetical packages are real.

## Carrier Lab promotion lifecycle

Carrier Lab owns fixed fixtures, orchestration, fault injection, bounded
observations, evidence finalization, and cleanup. It may call a product-shaped
Module through that Module's Interface. The dependency never reverses.

When laboratory behavior is proven and the Product Owner promotes the next
slice:

1. record the evidence and promotion decision;
2. define the smallest product Interface from the accepted behavior rather
   than from laboratory topology or tooling;
3. place the maintained Implementation in its owning product Module and use it
   from both the product caller and tests through the same Interface;
4. update the factual package map, dependency tests, and product records;
5. retain or remove laboratory code according to its evidence disposition.

Promotion is not a wholesale copy of a harness directory. A future product
Module must not import laboratory configuration, evidence schemas, fault
controls, Docker assumptions, or experiment state.

## Tests and test data

- Unit tests and integration tests contained within one Module live beside its
  implementation as `*_test.go` and exercise the Module Interface.
- A test crossing several Modules or processes lives under
  `tests/e2e/<behavior>/` only when the real cross-boundary behavior is
  implemented. It uses the root module and owns no product Implementation.
- A real-container network test lives under `tests/live/`, is selected by the
  `live` build tag, owns its complete lifecycle, and can run independently.
- `testdata/` lives directly below the Module, command, or e2e test that owns
  it. Test surfaces do not import fixtures or golden evidence from each other.
- Test Adapters satisfy the same Interface as real callers. Tests do not reach
  through a Seam to assert private implementation state.

Empty test trees, future profile fixtures, and speculative mocks are not
created. Generated test evidence follows the artifact rules below.

## Docker, infrastructure, and packaging

R-091 retired the closed Carrier Lab and Gate C container source trees. Their
source-bound records remain research provenance, but no tracked Dockerfile,
Compose input, lock, live profile, or workflow is a current reproduction or
product interface. A future experiment must create its own accepted,
purpose-named source boundary; it must not restore those paths by implication.

If supported image or operating-system package definitions become real, their
source belongs under `packaging/<target>/`; image definitions use
`packaging/images/<name>/`. Generated images, archives, installers, SBOMs, and
signatures remain outside the repository. A second image alone does not select
production packaging.

Environment-specific infrastructure or orchestration source belongs under
`deployments/<environment>/` only after an accepted delivery decision chooses
its ownership and lifecycle. Carrier Lab topology stays in its purpose-named
Compose file. Product logic never moves into Dockerfiles, Compose, deployment
templates, CI, or shell wrappers.

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
from laboratory/experiment/script code, formatting, selected unsafe constructs,
and common generated-artifact patterns. The Make targets add vet, tests, build,
module tidiness, race, Staticcheck, and vulnerability analysis.

Human review remains responsible for cohesive responsibility, Interface depth,
whether a Seam and two Adapters are real, product meaning of a dependency,
technology-neutral naming, delivery-horizon authorization, placement of
non-Go infrastructure, promotion from Carrier Lab, and repository extraction.
Those decisions cannot be inferred safely from path names or line counts.
