# Horizon 3 Stage 9 stabilization and technical closure brief

Status: **scope accepted by the Product Owner on 2026-08-17. Stage 9 is a
required Horizon 3 gate before Horizon 4.**

Authoritative inputs: accepted ADRs, the product contract and threat model,
accepted Horizon 3 research records, the H3 technical design, factual package and
dependency maps, completed Stage 1-8 evidence, R-040, and repository rules.

## Purpose

Stage 9 turns the functionally integrated Horizon 3 candidate into one stable,
clean, technically documented, reproducibly verified baseline for Horizon 4.

Stage 9 is not a feature stage. It adds no new product behavior, transport,
storage engine, consensus mechanism, public wire protocol, application feature,
privacy claim, capacity target, or supported platform. It may change internal
structure only to remove debt, consolidate ownership, retire temporary machinery,
and make already accepted behavior maintainable.

Cleanup happens before final qualification. A version cleaned after qualification
is a different, unqualified version and cannot become the H3 baseline without
rerunning the affected checks.

## Entry gate

Stage 9 starts only when:

1. Stages 1-8 have produced their required functional and integration outcomes;
2. no earlier stage still depends on an unimplemented placeholder or an open
   product decision;
3. all generated evidence, secrets, captures, caches, images, and build outputs
   remain outside the repository;
4. the Product Owner freezes Horizon 3 functionality and rejects feature additions
   until the Stage 9 disposition; and
5. one inventory identifies every maintained package, command, test suite,
   script, infrastructure file, and active documentation class.

Earlier-stage evidence remains useful input, but the final H3 claim belongs only
to the post-cleanup Stage 9 source and its own frozen verification identity.

## Required outcome

The Stage 9 output is one H3 stable baseline with:

- no placeholders, unfinished branches, temporary adapters, laboratory packages,
  laboratory commands, speculative interfaces, dead flags, or unowned files;
- a small factual production package and command graph;
- maintained qualification tooling that is separate from shipped production
  binaries and no longer presented as disposable laboratory code;
- one active technical source of truth for every mechanism, format, state machine,
  security boundary, operational procedure, and verification profile;
- no active development plans or readiness documents for completed stages;
- a fixed, documented, reproducible set of checks with explicit environment and
  evidence requirements; and
- a final independent verification result and Product Owner H3 closure decision.

## Working rules

1. **No feature work.** Every change must map to removal, consolidation,
   documentation conversion, reproducibility, or verification.
2. **One owner per fact.** Code, documents, tests, and schemas cannot silently
   define competing versions of one contract.
3. **No cleanup after the final freeze.** Any source, dependency, configuration,
   or normative-document change after freeze invalidates the affected result.
4. **Git is the development archive.** Completed implementation plans and
   readiness checklists are deleted after their still-valid content is moved;
   duplicate in-repository archive trees are not created.
5. **Durable decisions remain durable.** Accepted ADRs and research records stay
   available as decision provenance but are excluded from the default agent
   reading set unless a task touches their decision.
6. **Production and qualification stay separate.** Production packages and
   binaries cannot import or embed qualification runners, fixtures, expected
   results, or verdict logic.
7. **Independent verification remains independent.** Necessary duplication of a
   contract in a verifier is explicit and hash/schema-bound; accidental duplicate
   business logic is removed.

## S9.1 - Inventory, classification, and freeze plan

Create one factual disposition ledger covering every relevant repository path.
Each item receives exactly one class:

| Class | Meaning | Required disposition |
|---|---|---|
| Production | Maintained product package, command, configuration, or asset | Keep, simplify, document, and map |
| Qualification | Maintained test/verifier/harness needed to reproduce an H3 claim | Promote from lab status, isolate from shipping, document, and map |
| Technical documentation | Current mechanism/interface/operations/verification truth | Keep or consolidate into one authority |
| Durable decision | Accepted ADR or research record explaining why a contract exists | Retain as provenance; remove from default reading set |
| Transitional | Migration helper, compatibility branch, temporary schema, development mode, or one-use script | Remove after proving no maintained caller |
| Obsolete | Superseded, duplicated, unused, or contradicted material | Delete; rely on git history |

The ledger records owner, callers, imports, replacement destination, verification
impact, and deletion precondition. No unclassified path may enter S9.2.

S9.1 also freezes:

- the intended final production package/command graph;
- the maintained qualification responsibilities and their non-shipping boundary;
- the active technical-document set;
- the fixed verification tiers and exact command owners; and
- the complete rerun scope after structural changes.

Exit: Product Owner accepts the ledger and no item is labelled merely `later`,
`temporary`, `maybe`, or `keep for now`.

## S9.2 - Code and package stabilization

### Production code

- remove every `TODO`, placeholder, `not implemented`, unconditional fail-closed
  development blocker, unused feature flag, dead branch, and test-only control;
- remove `internal/lab/*`, `cmd/*-lab`, disposable experiment code, and production
  references to laboratory fixtures;
- promote only genuinely reusable qualification responsibilities into explicitly
  maintained non-shipping boundaries selected and registered during S9.1;
- delete temporary adapters and compatibility paths once all maintained callers
  use the accepted seam;
- consolidate duplicate production schemas, state representations, error classes,
  parsers, encoders, clocks, and lifecycle rules under one owning module;
- keep verifier recomputation separate without sharing candidate decision logic;
- reduce exported APIs to operations used by real non-test callers;
- remove speculative interfaces and single-implementation indirection unless the
  boundary is security-relevant or independently replaceable by accepted design;
- keep every package cohesive, every file responsibility-named and within limits,
  and every import allowed by the factual package map;
- preserve explicit resource, deadline, restart, cleanup, rollback, and failure
  behavior while simplifying implementation;
- remove abandoned dependencies, commands, scripts, build tags, configuration
  fields, schemas, fixtures, and generated artifacts; and
- prohibit new generic `util`, `common`, `misc`, `types`, `interfaces`, `api`,
  `sdk`, or dumping-ground packages.

### Package acceptance

Every retained package must have:

- one documented responsibility and exact permitted imports;
- maintained implementation and behavior tests;
- at least one real non-test caller where repository rules require it;
- no cycle, hidden reverse dependency, lab dependency, or command-owned domain
  logic;
- bounded errors and data structures using canonical domain terms; and
- a factual package-map row updated in the same structural change.

Every retained command must be a thin adapter with one explicit owner. Commands
used only for qualification are not shipped as product commands.

### Stabilization constraints

Refactoring must preserve accepted behavior. If a simplification changes an
observable contract, security boundary, evidence meaning, dependency, persistence
format, or wire/fixture format, it stops and follows normal research/ADR and
requalification rules rather than being hidden inside cleanup.

Exit: production source contains no laboratory tree or unfinished implementation;
the package graph matches the accepted S9.1 graph; all affected maintained tests
pass before documentation is declared final.

## S9.3 - Documentation conversion and reduction

Stage 9 replaces development-oriented documentation with technical documentation.
The goal is not to preserve every text file. The goal is to preserve every still
valid fact exactly once.

### Final active documentation set

The active set contains only:

1. repository entry and navigation;
2. canonical domain language needed to read current H3 code;
3. current technical architecture and package/dependency maps;
4. mechanism documentation for Network State, Routes, Service Connection,
   recovery, Bridge/blocked entry, naming, lifecycle, persistence, isolation,
   resource control, and other retained H3 components;
5. canonical formats, state machines, error classes, trust boundaries, and
   security/privacy limitations;
6. supported-host, build, configuration, operation, upgrade, recovery, and
   troubleshooting documentation;
7. fixed test and qualification specifications, commands, fixture requirements,
   expected outputs, and evidence retention rules;
8. known limitations and intentionally unsupported behavior; and
9. accepted ADRs/research records as selectively consulted decision provenance.

Product-planning narratives, journey planning, rejected scheduling alternatives,
and completed-stage implementation instructions are not part of the active
technical set. The canonical product contract and threat model remain only where
they still define normative behavior or security limitations required by the
repository authority order.

### Documents to retire

After valid content is migrated, delete:

- completed `horizon-3-stage-*-brief.md` implementation briefs;
- stage development plans and agent execution prompts;
- readiness checklists and temporary gate-status documents;
- intermediate evidence plans replaced by the final qualification specification;
- development fixture instructions for removed runners;
- duplicated architecture summaries and repeated contract prose;
- migration instructions for migrations that can no longer be run; and
- status texts containing stale `pending`, `in progress`, `next`, or provisional
  statements.

Stage 9's own brief is retired after its completion facts are captured in the H3
technical baseline and closure report.

### Migration rule

For each retired document:

1. identify every normative statement still implemented or verified;
2. move that statement to the single owning technical document;
3. replace prose with precise formats, state transitions, field tables, commands,
   invariants, failure behavior, and limitations where appropriate;
4. update inbound links and the active documentation index;
5. confirm no current code/test/package map depends on the old path; and
6. delete the old document rather than copying it into an active archive tree.

### Agent context budget

The final documentation index defines:

- a small mandatory reading set for every task;
- task-routed technical documents by package/mechanism;
- decision records consulted only when their decision is relevant; and
- historical material available through git rather than normal agent discovery.

No technical fact may require reading a chain of stage plans to discover the
current behavior. No agent should load all Horizon 3 history to modify one module.

Exit: every active document describes the final system, has one current owner,
contains no development status, and appears in the documentation index; every
retired document has no remaining unique normative content.

## S9.4 - Test and verification consolidation

Classify every retained check into one tier:

| Tier | Purpose | Normal use |
|---|---|---|
| Fast | Formatting, architecture, unit behavior, static invariants | Every change |
| Full | All maintained unit/integration and platform-independent command tests | Before integration |
| Platform/live | OS, process, socket, container, resource, restart, and cleanup behavior | Declared affected changes |
| Qualification | Frozen hostile, capacity, privacy, recovery, migration, and end-to-end matrices with independent verdicts | Release/H3 gate |
| Reproduction | Rebuild and independently verify retained evidence from frozen source/supply identity | Audit and gate replay |

Stage 9 must:

- remove tests for deleted behavior and duplicate tests that prove the same
  invariant through the same boundary without additional risk coverage;
- retain negative tests for malformed, stale, conflicting, replayed, resource,
  privilege, cleanup, privacy, and rollback behavior;
- replace laboratory names and development-only modes in the retained
  qualification suite without weakening independence or evidence identity;
- remove sleeps, ambient network dependence, uncontrolled randomness, hidden
  retries, order dependence, and mutable shared fixtures;
- freeze seeds, clocks, limits, schemas, source/supply identities, host profiles,
  expected runtime outcomes, and verdict predicates where qualification requires
  them;
- map every retained product/security requirement to at least one owning test or
  explicitly document why it is inspection-only;
- map every test to one exact command and environment owner;
- define maximum runtime and resource envelope for each tier;
- ensure expected runtime failure can yield verifier `pass`, while malformed
  evidence yields `invalid` and trustworthy contract breach yields `fail`; and
- keep generated bundles, secrets, captures, caches, databases, and build outputs
  outside the repository.

The final Make/command surface is selected from real maintained commands during
S9.1. Stage 9 removes aliases and obsolete targets rather than inventing several
ways to run the same suite.

Development and regression checks before Stage 9 are not expected to substitute
for final H3 qualification. They must be strong enough to support implementation,
detect regressions, and preserve accepted stage contracts, but they do not need to
simulate the full pre-H4 stand in every development cycle. Stage 9 consolidates
their durable requirements into the final qualification matrix and removes
temporary harnesses that have no role in that matrix.

Exit: a clean checkout has one documented command per tier, no undocumented
required pre-step, and a fixed traceability matrix from requirement to check.

## S9.5 - Build, dependency, and infrastructure stabilization

- build every shipped binary from a clean checkout using the pinned supported
  toolchain and reviewed runtime dependencies;
- remove unused Dockerfiles, Compose files, images, scripts, service definitions,
  environment variables, ports, volumes, capabilities, and temporary host setup;
- separate production packaging from qualification images and tools;
- prohibit runtime downloads, ambient proxy/DNS dependence, implicit credentials,
  mutable tags, and undeclared external services in qualification paths;
- pin and document every required external binary/image/source identity and its
  update/removal owner;
- verify least privilege, owner-only secret/state roots, bounded resources,
  deterministic cleanup, and zero owned residue;
- ensure supported-host setup, startup, shutdown, restart, recovery, diagnostics,
  and removal procedures match the final implementation;
- remove repository-local caches, generated evidence, captures, secrets, databases,
  temporary roots, and build outputs;
- verify dependency/license/advisory records and remove dependencies without a
  retained production or qualification caller; and
- freeze the final infrastructure and supply manifest before S9.6.

Exit: production and qualification can be built reproducibly from their declared
inputs, run without hidden infrastructure, and leave only explicitly retained
external evidence.

## S9.6 - Final H3 qualification and handoff

After S9.1-S9.5 complete:

1. freeze the exact source, dependency, toolchain, configuration, schema,
   fixture, image, external binary, and supported-host identities;
2. freeze the dedicated pre-H4 stand profile: non-overcommitted host classes,
   topology, capacity, network conditions, clocks, fault schedule, sustained-run
   duration, observers, collectors, cleanup bounds, and external evidence roots;
3. deploy the frozen clean candidate to the dedicated powerful stand without
   rebuilding or substituting supply during the campaign;
4. build all shipped and qualification binaries from the frozen clean source;
5. run the complete fixed Fast and Full tiers;
6. run every required Platform/live and Qualification cell against the cleaned
   source on the declared stand, including all affected Stage 1-8 contracts,
   high-capacity operation, sustained work, hostile/fault conditions, restart,
   recovery, migration, privacy boundaries, and resource pressure;
7. independently recompute all required verdicts from immutable evidence;
8. verify clean install/start/restart/shutdown/removal, bounded resources, no
   forbidden fallback, no privilege expansion, and zero owned residue;
9. produce the final active technical-document index, package/dependency maps,
   verification matrix, known limitations, and H3 closure report; and
10. obtain the Product Owner `advance-to-H4`, `redesign`, or `stop` disposition.

Any source, dependency, configuration, schema, technical contract, or
qualification change after step 1 creates a new candidate identity and reruns the
complete affected scope. A favorable earlier-stage result cannot waive this rule.

## Required final artifacts

- one source/supply identity for the stable H3 baseline;
- factual production and qualification package/command maps;
- active technical-document index and agent context routing;
- requirement-to-test/command traceability matrix;
- fixed environment and verification profile;
- frozen dedicated pre-H4 stand topology, capacity, supply, clocks, and fault
  schedule;
- independent final verdict set and bounded diagnostics;
- known limitations and unsupported behavior;
- complete external evidence/cleanup inventory; and
- Product Owner Horizon 3 closure disposition.

## Pass condition

Stage 9 passes only when all conjuncts hold:

- no product functionality remains incomplete or represented by a placeholder;
- no laboratory/experiment package, command, document, or infrastructure file
  remains in the final maintained tree;
- no active completed-stage development plan, readiness checklist, or temporary
  evidence narrative remains;
- every retained mechanism has current technical documentation and one owner;
- every retained package, command, dependency, test, and infrastructure file is
  classified, mapped, called, and justified;
- production artifacts exclude qualification tooling and secrets;
- the fixed verification tiers pass against the post-cleanup frozen source;
- independent verdicts and cleanup are complete; and
- the Product Owner accepts the H3 stable baseline for Horizon 4.

## Stop/redesign conditions

- cleanup requires changing accepted product or security behavior without a new
  decision and requalification;
- a laboratory component cannot be removed or promoted without becoming a hidden
  production dependency;
- current behavior cannot be derived without retaining contradictory documents;
- qualification depends on runner-authored verdicts, mutable supply, hidden
  services, ambient credentials, or repository-local generated evidence;
- a retained package has no cohesive owner or real caller;
- the final suite cannot be reproduced from a clean checkout and declared external
  inputs; or
- the project attempts to call H3 complete before qualifying the cleaned source.
