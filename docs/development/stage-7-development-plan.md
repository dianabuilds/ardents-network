# Stage 7 development plan

Status: **review; planning and disposable research only. Stage 7 maintained
coding remains closed until Stage 6 advances and the Product Owner checks the
S7.0 gate.**

This plan maps R-048 and the Stage 7 lifecycle specification into ordered,
independently reviewable vertical slices. It does not create packages,
commands, dependencies, formats, platform privileges, or implementation
authority by documentation.

## 1. Predecessors and fixed point

The [readiness checklist](stage-7-readiness-checklist.md) sections A-D are the
sole normative coding-start gate. This plan does not restate or weaken them.

Documentation/research during Stage 6 MUST NOT alter Stage 6 semantics, package
map, dependencies, Go module, or maintained command tree for Stage 7.

## 2. Required research before coding

| Record | Exact decision | Required evidence | ADR/dependency consequence |
|---|---|---|---|
| R-049 | Which maintained TUF-compatible Go verifier and exact release profile meet ADR-0006? | conformance/misuse vectors, dependency/license/advisory closure, bounded fetch/cache, role/threshold/expiry/rollback/mix tests | dependency review; ADR only if profile changes consequential trust semantics |
| R-050 | Which Ubuntu/Windows package, stable-bootstrap, immutable-layout, atomic activation, crash durability, repair/uninstall Adapters meet the release-activation proposal? | frozen filesystem/API experiments with interruption at each state, open executable, ACL/mode, cross-volume negatives, residue | promote/revise/reject the proposal; external build/package tools recorded |
| R-051 | Which Ubuntu Unix-IPC and Windows named-pipe/local-channel facts securely bind a launcher-born principal? | peer/token/session/process-tree tests, PID reuse, substitution, restart, failed identity query, privilege crossing | promote/revise/reject the Application-principal proposal; `x/sys` or other dependency review |
| R-052 | Which stable Ubuntu and Windows mechanisms deny direct networking for a complete helper tree? | capability/privilege/source review plus ingress/egress/child/escape/resource/cleanup experiment | promote/revise/reject the Application-principal proposal; stop if only experimental API, driver, cgo, or broad daemon fits |
| R-053 | Which Vault protection and Recovery Bundle profile preserves roots and monotonic signing state on both platforms? | cryptographic-library selection, KDF/format bounds, tamper/wrong-secret/restore/reconcile/lock/export tests, secret cleanup | research and ADR for any hard-to-reverse crypto/platform custody selection |
| R-054 | Which canonical Stage 7 evidence profile independently proves the contract? | schema vectors, mutation corpus, clocks, artifact ownership, exact cells/episodes/resources/cleanup and deterministic calculator | freezes S7 evidence identity; verifier package only with real implementation/caller |

Each record begins with falsification criteria. A popular library or OS feature
is only a candidate. No spike is promoted by copying its directory.

## 3. Planned Module graph

The graph is logical until the owning slice creates real packages:

```text
platform package Adapter
        -> Install Lifecycle Module
        -> stable bootstrap
                 -> Update Transaction Module
                         -> Release Decision Module
                         -> existing readiness/drain owners

Authority Custody Module  (separate state and process authority)

Application -> Application Broker Module -> existing Application Interface
                    |
                    -> Application Isolation Module -> Ubuntu/Windows Adapter
```

Import direction constraints:

- Release Decision knows no downloader, installer, command, Application, Route,
  naming, Vault, or platform package tool.
- Update Transaction consumes a release decision and narrow existing
  drain/self-test Interfaces; it cannot parse or sign release metadata.
- Install Lifecycle composes platform registration and the bootstrap; it cannot
  read Vault secrets or grant Applications.
- Authority Custody exposes only custody operations and never imports updater,
  Application IPC, Route, or platform installer orchestration.
- Application Broker consumes Local Grant policy and a bound platform result;
  it cannot decide release, Route, or Authority.
- Application Isolation returns containment observations; it cannot grant an
  operation or become a transparent proxy.
- Commands remain thin composition adapters and own no domain state machine.

If a proposed package has only one trivial caller/implementation and moving it
back would not spread complexity, do not create it. R-050 must first prove that
the platform dependency is genuinely replaceable. A test Adapter may then stand
in for that real dependency behind an unexported seam, but no package is created
until a maintained platform Implementation and non-test caller arrive together.

## 4. Slice execution

### S7.1 release decision

Behavior-first order:

1. add frozen public vectors and independent expected classifications from
   R-049;
2. implement strict bounded metadata/artifact input and persisted release floors;
3. test consecutive dual-threshold root rotation plus gap, reuse, one-sided,
   expiry, and cross-environment rejection;
4. test automatic Release Safety refresh, the `90-day` ordinary protocol/
   capacity gate, and the bounded `4-of-5` emergency limitations independently
   of build safety;
5. test role authority, thresholds, expiry, rollback/freeze/mix, platform/
   environment, length/hash, cache confinement, unknown fields, and cleanup;
6. add one non-test offline-import caller only after the Interface is deep; and
7. run the S7.1 evidence subset.

No network downloader or package installation is needed for this slice.

### S7.2 update transaction and custody preservation

1. create an immutable versioned filesystem fixture through the selected
   Adapter seam;
2. test the state machine and restart recovery at every durable transition;
3. integrate stop-new-work/drain/self-test through existing narrow Interfaces;
4. exercise Contributor stop-new-assignment, per-role drain, non-revival, and
   fresh rejoin-or-withdraw behavior;
5. keep Vault and every monotonic floor outside the rollback tree;
6. add copy-on-write schema commit and safe rollback/refusal;
7. test pressure and bounded cleanup; and
8. run C/D evidence subsets.

The selected first platform Implementation and its non-test caller may promote
the R-050-proven internal seam; the test Adapter exercises the same Interface.
The test surface remains the Update Transaction Interface, not its internal
files.

### S7.3 Ubuntu lifecycle

1. produce the frozen laboratory package outside Git from declared inputs;
2. install on a clean frozen image and inventory created paths/processes/
   registrations/privilege;
3. exercise offline and network start, repair, update, rollback, restart;
4. exercise empty/non-empty Vault uninstall and explicit purge;
5. compare actual residue to the predeclared cleanup inventory; and
6. run the complete Ubuntu A-D subset.

### S7.4 Windows lifecycle

Repeat the exact shared outcomes with the selected Windows Adapters. Add no
Windows-specific public outcome except safe diagnostic detail. Explicitly test
ACL owner, reparse/path behavior, file locking/open executable, platform service
or startup registration, power-loss equivalent, repair, uninstall, purge, and
residue.

### S7.5 Application Broker

1. preserve the existing external Application Interface semantics;
2. replace laboratory-only principal assumptions behind one broker Interface;
3. implement launcher-before-untrusted-code and bound local channel per R-051;
4. test connection/admin/custody privilege lattice and resource parents;
5. attack with hostile same-user sibling, stolen/replayed capability, PID reuse,
   channel substitution, inherited handle, restart, and revoke/drain; and
6. run E evidence on both platforms.

Do not delete Stage 3 laboratory adapters until Stage 9 unless an accepted
cleanup scope proves they have no retained qualification role.

### S7.6 Application Isolation

1. implement the accepted Ubuntu and Windows Adapters behind one Interface;
2. launch controlled deterministic client/publisher Applications plus nested
   malicious helpers;
3. verify scoped IPC and per-context storage still work;
4. run every ingress/egress/child/escape/resource/restart/cleanup probe;
5. prove generic attachment reports the unqualified limitation; and
6. run F/G evidence, including all development platform pairings.

Any escape is a candidate `fail`; harness/observer uncertainty is `invalid`.
Neither can become generic success inside the same cell.

### S7.7 combined development handoff

1. freeze source, dependencies, package tools, host images, roots, metadata,
   artifacts, config, manifest, seeds, observers, verifier, and cleanup inputs;
2. build packages from clean source without runtime downloads;
3. run all fast/full checks and complete Stage 7 development matrix;
4. independently recompute verdicts;
5. retain immutable external evidence and remove all owned runtime artifacts;
6. document known limitations and exact H3/H4 claim ceiling; and
7. obtain Product Owner `advance-to-S8`, `redesign`, or `stop`.

## 5. Test strategy

### While writing

- write behavior tests at the owning Module Interface before or with each
  observable state transition;
- run the narrow package test after each behavior change;
- run `make quick-check` regularly;
- run platform/live tests only on frozen supported images with declared external
  roots; and
- never weaken a gate, replace a failure after seeing it, or treat candidate
  self-report as evidence.

### Before slice integration

- behavior, misuse, negative, restart, pressure, and cleanup tests pass;
- new files satisfy line/responsibility rules;
- package map and dependency record are factual and updated in the same change;
- platform-specific files compile/test for both targets where applicable;
- no generated artifact, cache, secret, evidence, installer, database, or VM
  state is inside the repository; and
- `make check` passes from the intended clean commit.

### Before Stage 7 disposition

Run the exact R-054 campaign against post-implementation packages, not a spike
or pre-cleanup surrogate. Verify that a complete valid contract breach is
`fail`, corrupt/missing/contaminated evidence is `invalid`, and expected
fail-closed runtime behavior can be `pass`.

## 6. Change and commit policy

- one scoped commit per accepted research result, ADR, or vertical slice;
- preserve unrelated Stage 6/user changes;
- do not combine dependency introduction with unrelated platform behavior;
- a changed release format, trust root, cryptographic profile, activation
  durability contract, principal identity, isolation mechanism, evidence
  meaning, or privilege requires research/ADR review before implementation;
- later cleanup does not rewrite retained historical evidence; and
- no commit message may call development evidence qualification or imply H4
  independent supply.

## 7. Slice acceptance report

Each slice handoff states:

- exact source commit and predecessor decision;
- behaviors implemented and explicitly absent;
- new/changed packages, commands, dependencies, privileges, formats, and state;
- tests and platform cells run with exact results;
- external evidence identity and cleanup result;
- security/privacy claim in five-part format;
- known limitations and deferred qualification; and
- Product Owner disposition.

Failure of any conjunct leaves the slice incomplete. Progress in one platform or
profile cannot compensate for another required failure.
