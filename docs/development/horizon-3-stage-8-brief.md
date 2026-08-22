# Horizon 3 Stage 8 productization and restructuring brief

Status: **execution started by Product Owner authorization on 2026-08-22;
S8.0 current-system truth, S8.1 preservation disposition, S8.2 policy
promotion, S8.3 target architecture, and S8.4 migration planning are
complete.**
This brief replaces the earlier
reassessment-only Stage 8 plan. Stage 7 was stopped by its separate 2026-08-22
disposition, and the exact Stage 8 entry is bound by the
[Stage 8 start record](stage-8-start-record.md).

Authoritative inputs, in order, are accepted ADRs; the product contract and
threat model; completed research and its evidence; the accepted Stage 7
disposition; and the clean repository identity recorded at Stage 8 entry. The
[Stage 8 productization design workbook](stage-8-design-workbook.md) supplies
the prepared G0-G5 analysis. It remains transitional: this brief defines the
stage, while accepted policies and factual architecture are promoted into their
canonical documents during Stage 8.

## Purpose

Stage 8 turns the accumulated Stage 1-7 implementation into one maintainable
product codebase. It first decides which product behavior and claims survive,
then replaces the stage-derived architecture, code rules, tests, commands, and
documentation through finite Module migration waves.

Stage 8 owns every planned mutation before Horizon 3 closure. It may change
Implementation, Interfaces, packages, commands, tests, build/qualification
tooling, formats through an accepted migration, and active documentation. It
does not claim the final multi-platform, adversarial, sustained, or multi-day
Qualification. Stage 9 proves the exact candidate frozen after the last Stage 8
mutation.

## Required outcome

Stage 8 produces one candidate with:

- an accepted and intentionally bounded product contract;
- cohesive deep Modules with small caller-owned Interfaces and explicit trust,
  state, lifecycle, failure, resource, and compatibility ownership;
- no package, file, export, command, Adapter, test, fixture, or document retained
  merely because a prior stage created it;
- one qualitative testing pyramid and separate claim-level Qualification model;
- current technical and operational documentation describing implemented
  behavior rather than development chronology;
- no simultaneous old/new authority or writer after a migration wave; and
- no planned code, policy, test, dependency, infrastructure, format, or
  normative-document mutation remaining at the proposed Stage 9 freeze.

## Entry gate

Stage 8 starts only when:

1. the Product Owner has accepted the Stage 7 disposition and identified every
   incomplete, deferred, or rejected Stage 7 slice;
2. one clean commit identifies the source to be reassessed, with dependency,
   toolchain, platform, configuration, schema, and available evidence inputs;
3. the ordinary repository checks can establish a diagnostic baseline, or each
   unavailable check has a recorded environment finding rather than a waiver;
4. generated evidence, captures, secrets, databases, caches, images, and build
   outputs remain outside the repository;
5. no moving Stage 7 worktree observation is treated as the final inventory;
6. the Product Owner explicitly authorizes Stage 8 reassessment and
   productization; and
7. current rules remain binding until their accepted replacements are promoted
   into canonical policy and executable gates.

All seven entry conditions were accepted on 2026-08-22. The clean source,
toolchain inputs, successful ordinary checks, Product Owner authorization, and
first authorized slice are recorded in the
[Stage 8 start record](stage-8-start-record.md).

## Operating rules

- Product behavior, authority, durable state, security/privacy claims, and
  external compatibility never change under the label of refactoring.
- A Module is accepted for its responsibility and Interface, not for a target
  file count or directory shape. Nested directories are real packages.
- Callers and tests cross the same Module Interface. Internal test seams remain
  private to the Implementation.
- A replaceable dependency receives an Adapter seam only when real production
  and test/platform variation justifies it.
- Every migration has one old owner, one target owner, an atomic cutover, a
  rollback/forward-repair rule, and an explicit deletion outcome.
- New behavior tests replace tests of the displaced seam. They do not layer
  indefinitely over historical package tests.
- Documentation is completed inside the owning migration wave. M14 verifies and
  consolidates it; M14 is not a documentation-writing backlog.
- Parallel Codex work is capacity, not independent validation. One integration
  task owns shared Interfaces, state/format decisions, package maps, global
  policy, and final gates.

## S8.0 — Freeze, delta audit, and current-system truth

Freeze the clean Stage 8-entry identity and establish what actually exists:

1. rescan every maintained package, command, process, import edge, runtime
   dependency, durable root, format, Interface, external Adapter, and trust zone;
2. trace each accepted journey and security claim through real callers and
   observable tests;
3. rerun the G2 source audit against the Stage 7 delta and add, close, split, or
   invalidate findings using evidence rather than filename continuity;
4. inventory Module, Adapter, composition/process, Live, Qualification, and
   historical-reproduction tests with timings, skips, external requirements,
   duplicates, and flake behavior;
5. inventory active truth, engineering policy, ADR/research provenance,
   transitional documents, and external generated evidence;
6. run bounded deterministic, restart, recovery, resource, update/rollback, and
   cleanup diagnostics sufficient to characterize the current candidate; and
7. record every open product, algorithm, platform, format, compatibility, or
   evidence question that must be decided before its owning wave.

S8.0 is diagnostic. It may falsify the prepared G0-G5 candidates, but it does
not silently redesign them and cannot emit a final Qualification `pass`.

Exit:

- immutable source and input identities;
- one temporary factual current-system report;
- updated finding and open-decision ledgers;
- complete code/test/document disposition inventories; and
- a clean-baseline discrepancy report against the prepared workbook.

## S8.1 — Product and preservation disposition

Apply G0 to the clean system. The Product Owner selects exactly one product
disposition:

- `continue` — preserve the accepted Product Core and claims;
- `narrow` — explicitly remove journeys or claims whose burden is unjustified;
- `redesign` — supersede affected product/protocol decisions before code
  restructuring;
- `stop` — retain only required closure and provenance material.

Every observable surface receives `preserve`, `migrate`, `replace`, `remove`, or
`decide first`: product behavior, authority, state, wire/persistence/config/CLI/
IPC/evidence formats, commands, external callers, experiments, and claims.
`Remove` is not authorized merely because something is stage-named or ugly. It
requires proof that no retained caller, state, compatibility promise, claim, or
reproduction obligation still owns it.

Exit:

- accepted product scope and honest limitations;
- preservation and compatibility contract;
- product/claim disposition for every lab and deferred campaign; and
- resolved decision authority for every `decide first` item needed by S8.3.

No Module migration begins before this exit is accepted.

## S8.2 — Engineering, testing, and documentation model

Promote the accepted G2, G3, and G5 results into current authorities rather than
adding another policy layer:

- `AGENTS.md`, `go-engineering.md`, and `repository-layout.md` receive the
  cohesion-, Interface-, and risk-based code/package policy;
- numeric file/export/test-size limits cease to be architecture verdicts once
  their replacement review signals and durable graph/security gates are active;
- `testing.md` and executable manifests define Module behavior, Adapter
  contract, composition/process, Live, Qualification, and evidence-reproduction
  surfaces plus developer, deterministic, race, platform, fuzz, live, soak, and
  Qualification profiles;
- duplicate tests are decided from requirement, seam, observable oracle,
  fault/adversary, platform/format, and independence role—not similar names;
- failure, skip, invalid-environment, rerun, flake, fixture, clock, concurrency,
  cleanup, and evidence rules become explicit; and
- the target technical/operations information architecture, ownership triggers,
  consistency checks, and promotion/deletion lifecycle become active.

Exit: one non-conflicting development model and executable enforcement map.
Until this exit is accepted and promoted, Stage 7 rules remain in force.

## S8.3 — Target architecture and compatibility design

Promote the accepted G1 Module map and refine it only where S8.0/S8.1 evidence
requires. The target responsibilities are:

- `endpoint` as product composition root;
- `application/broker` and platform `application/isolation`;
- `network/state`, `network/source`, and `network/duty`;
- `resource` for process/resource coordination without usurping domain policy;
- `naming/namespace` and `naming/resolution`;
- `service/publication` and `service/connection`;
- `entry`, `route`, and the selected `route/webtunnel` Adapter if retained;
- `node` for contributor-role lifecycle;
- `release`, `update`, and `custody` as separate authorities and state owners;
  and
- thin retained product commands plus separately owned Qualification tools.

For every Module, S8.3 fixes:

1. cohesive responsibility, deletion test, callers, and command ownership;
2. small Interface including ordering, failures, resources, cancellation,
   cleanup, and observability;
3. durable state/lifecycle writer and trust-zone crossings;
4. consumer-owned ports and justified concrete Adapters;
5. permitted imports and private Implementation locality;
6. normal, failure, restart, race, compatibility, and platform test surface;
7. every affected persisted, wire, configuration, result, command, IPC, and
   evidence representation with `retain`, `read/migrate`, `break`, or `delete`;
8. rollback versus security-forward-only repair behavior; and
9. exact G2 finding and current-package disposition.

Unresolved algorithm, cryptographic, protocol, storage, platform, or support
window choices obtain their required research and ADR before implementation.
Empty target packages and speculative Interfaces are forbidden.

Exit: accepted target Module/command/import/trust map, Interface contracts, and
format/compatibility ledger. This is the design authority for S8.4-S8.5.

## S8.4 — Migration and retirement plan

Create one temporary `docs/development/refactoring-plan.md`. It contains a
finite wave graph and a code-retirement ledger. Every wave row records:

- frozen input and intended target identity;
- owning Module and user-visible objective;
- old and new authority/state/lifecycle owners;
- preserved G0 contracts and resolved G2 findings;
- target Interface, callers, Adapters, formats, and compatibility window;
- characterization and replacement tests;
- observability, rollback or forward-repair point, and residue checks;
- exact packages, files, exports, commands, tests, fixtures, plans, scripts,
  configs, dependencies, and documents to delete; and
- affected checks, platforms, Qualification claims, dependencies, and source
  identity.

The ordered default wave graph is:

| Wave | Ownership transfer and required deletion outcome |
|---|---|
| M0 | freeze governance, policies, manifests, package/format/doc/code-retirement ledgers; remove superseded numeric/historical gates |
| M1 | `releasedecision` -> `release`; preserve non-decreasing floors and delete staging-era exports/tests |
| M2 | `updatetransaction` -> `update`; install real activation/recovery ownership and delete caller-authored transaction choreography |
| M3 | deepen authenticated `network/state`, invert `network/source`, and remove concrete Source/peer-store leakage |
| M4 | `localroles` -> `network/duty`, then consolidate real shared resource policy without a generic dumping ground |
| M5 | seven naming packages -> `naming/namespace`; remove duplicated validators, field bags, and stage fixtures after proof transfer |
| M6 | `nameresolution` -> `naming/resolution` over opaque current views; retire old plan/implementation imports |
| M7 | `bridge` -> `entry`; move the retained Carrier Adapter to `route/webtunnel` or remove it after mechanism disposition |
| M8 | deepen `route`, absorb retained `routeplan` behavior, and delete role/evidence unions and stage workload orchestration |
| M9 | create `service/publication`, deepen `service/connection`, and delete action unions/static authority bags/fixed campaign semantics |
| M10 | introduce real Isolation and Broker, transfer application composition to `endpoint`, then remove old IPC/socket/plan/stage-app surfaces |
| M11 | fold Probe into `node`, move to opaque State/Duty/Resource/Route Interfaces, and delete snapshot translation glue |
| M12 | implement `custody` after its dependent authorities stabilize; remove limitation-string substitutes and foreign custody fields by disposition |
| M13 | consolidate shipped commands/configuration and operator journeys; delete product tracer commands and transitional readers after their support windows |
| M14 | run productization closure, promote current docs, retain only claim-bearing Qualification/reproduction code, and delete obsolete labs, runners, fixtures, tests, stage docs, and the temporary plan |

M1/M2 may move later if Stage 7 release/update work is not a stable input, but
M1 remains before M2. Other reordering requires explicit dependency, authority,
and rollback analysis; wave numbers are not permission to parallelize coupled
state owners.

Exit: a Product Owner-accepted finite plan with no unowned deletion, migration,
format, test, documentation, or Qualification work.

## S8.5 — Contract-first Module replacement

Execute one accepted wave at a time through the target Interface:

1. freeze the wave input and reconfirm open decisions;
2. characterize retained observable behavior and compatibility at the old seam;
3. implement the target Module and its internal seams without creating a second
   external authority;
4. pass new Module behavior and real Adapter contracts;
5. migrate durable state/formats with restart, corruption, rollback/repair, and
   mixed-version checks;
6. cut real callers, commands, and composition to the new Interface;
7. replace—not copy—old seam tests with target behavior/process/platform tests;
8. update package/dependency maps and current technical/operational docs;
9. remove the displaced code and all ledgered scaffolding in the same wave or
   retain one bounded compatibility reader with an explicit expiry; and
10. run affected, full deterministic, race/platform, and bounded Live profiles
    required by the wave.

A wave is complete only when the new owner is the sole authority/writer, normal
and failure behavior is observable through its Interface, cleanup and residue
are proven, callers no longer import displaced Implementation, and no orphan
file/package/test/document remains.

If a migration exposes an undecided product fact, dual-write requirement,
unbounded compatibility reader, unrecoverable state ambiguity, security
downgrade, or untestable rollback, stop the wave. Restore the last safe identity
or enter an explicit repair state, revise S8.1-S8.4, and repeat; do not hide the
problem with forwarding layers.

## S8.6 — Productization closure and Stage 9 freeze proposal

After M14:

1. verify every G0 row, G2 finding, target Module, format, current package,
   command, test profile, dependency, infrastructure asset, and document has a
   final disposition;
2. run all full deterministic, race, compatibility, affected-platform, clean
   install/update/rollback/restore/remove, and bounded integrated/Live readiness
   profiles from a clean checkout;
3. demonstrate the contributor, auditor, operator, and Product Owner document
   routes against implemented behavior;
4. confirm no laboratory/experiment code remains in the product graph and every
   retained Qualification/reproduction artifact has one named claim and expiry;
5. delete the completed refactoring plan, disposition ledgers, S8.0 report,
   workbook, completed-stage materials, and obsolete fixtures after their
   unique current facts reach canonical owners;
6. freeze source, build, dependency, toolchain, configuration, schema, format,
   fixture, Qualification tool, verifier, supported platform/host, active
   normative document, and external evidence-location identities;
7. freeze the Stage 9 acceptance matrix, schedule, stand topology, resources,
   fault plan, clocks, observers, cleanup rules, and evidence retention; and
8. obtain the Product Owner `admit-to-stage-9`, `return`, `redesign`, or `stop`
   disposition.

Stage 8 readiness is not final Qualification. A green development profile or
earlier-stage evidence cannot waive Stage 9.

## Required Stage 8 outputs

1. clean Stage 8-entry identity and factual current-system/delta report;
2. accepted product/preservation/compatibility disposition;
3. promoted engineering, testing, documentation, and collaboration model;
4. accepted target Module/Interface/Adapter/command/import/trust and format map;
5. completed migration waves and code/document retirement ledger;
6. one maintained product Implementation with current technical/operations
   documentation and no stage-derived duplicate ownership;
7. clean readiness results and explicit remaining limitations; and
8. accepted immutable Stage 9 freeze proposal and external evidence location.

## Pass, return, redesign, and stop

- **Pass:** every required output exists, all planned mutations and transitional
  deletions are complete, the readiness profiles pass, and the Product Owner
  admits the frozen candidate to Stage 9.
- **Return:** a wave, policy, document route, compatibility obligation, or
  readiness check is incomplete; resume its owning S8 workstream without
  pretending the candidate is frozen.
- **Redesign:** new evidence invalidates product scope, a hard-to-reverse
  decision, Module responsibility, state authority, or feasible support model;
  return to S8.1/S8.3 with the required research/ADR authority.
- **Stop:** the selected product is incoherent, cannot be maintained by the
  actual one-Product-Owner-plus-Codex team, cannot preserve its honest claims,
  or has no safe/testable migration path.
