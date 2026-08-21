# Stage 7 development plan

Status: **accepted execution plan; S7.0 completed and `start S7.1` authorized on
2026-08-20.** Each later slice still requires its behavior, evidence, dependency,
package-map, and quality gates.

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
| R-050 | Which Ubuntu/Windows Installed lifecycle Adapters and minimal Portable executable artifacts meet ADR-0015? | Ubuntu-Docker package/filesystem smoke, current-Windows non-install API evidence, separately authorized MSI lifecycle cells, explicit native durability/residue deferrals, and Portable digest/direct-run/stopped-replacement/state-separation checks | ADR-0015 accepted; external build/package tools recorded; a falsifier reopens it |
| R-051 | Which Ubuntu and Windows private-channel/process-tree facts securely bind a launcher-born principal? | O2 selected profile: inherited unnamed Unix IPC + atomic cgroup-v2/pidfd tree on Ubuntu; inherited anonymous pipes + suspended root/token/Job tree on Windows; generic named-endpoint and co-resident direct-binary controls; theft, PID reuse, substitution, restart, failed query, privilege crossing, bounds, and cleanup | ADR-0016 accepts O2 and the exact bounded Windows `unsafe.Pointer` bridge with dedicated risk tests |
| R-052 | Which stable Ubuntu and Windows mechanisms deny direct networking for a complete helper tree? | exact `ubuntu-bwrap-native-v1` and `windows-appcontainer-native-v1` specifications; source/privilege review plus ingress/egress/child/escape/resource/cleanup campaign; isolated browser is explicitly unsupported | ADR-0016 accepts the profiles; stop/reopen if scheduled native predicates fail |
| R-053 | Which Vault protection and Recovery Bundle profile preserves roots and monotonic signing state on both platforms? | exact password-derived envelope specification; accepted Argon2id/AES-GCM profile, synthetic logic prototype, then the scheduled Docker/current-Windows KDF/RSS/vector/tamper/interruption/cross-restore/reconcile/cleanup subset with weakest-native-host qualification explicitly deferred | ADR-0021 accepted; a falsifier reopens it |
| R-054 | Which canonical Stage 7 evidence profile independently proves the contract? | S7E1 schema vectors and mutation corpus plus the observer-source profile, exact 91-cell/392-episode reference inventory, and manifest-bound `scheduled`, `authorization-pending`, and `environment-deferred` partition in `stage-7-host-campaign-spec.md` | freezes S7 development-evidence identity and claim ceiling; verifier package only with real implementation/caller |
| R-056 | Which exact direct-binary and optional desktop/browser Adapters implement accepted topology O1 without changing ordinary Internet or active VPN policy, and which Application profile can be claim-bearing? | accepted Adapter specification plus CLI/input/output, URI/loopback/origin/process/storage/lifecycle source review and Installed/Portable parity, ordinary/VPN/kill-switch, escape, fallback, restart, browser-update, unsupported-isolated, and cleanup evidence | O1 accepted; ADR only for a dedicated/isolated browser or system-network scope change |

Each record begins with falsification criteria. A popular library or OS feature
is only a candidate. No spike is promoted by copying its directory.

## 3. Planned Module graph

The graph is logical until the owning slice creates real packages:

```text
Ubuntu/Windows Installed Adapter
        -> Install Lifecycle Module
        -> stable bootstrap
                 -> authenticated Endpoint executable
                         -> Update Transaction Module
                         -> Release Decision Module
                         -> existing readiness/drain owners

Portable release target -> same authenticated Endpoint executable directly

Authority Custody Module  (separate state and process authority)

direct binary / Service Link / native Application / optional browser Adapter
                    -> Application Broker Module -> existing Application Interface
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
- The accepted R-056 O1 topology keeps the specified direct-binary Adapter
  first-class in both Distribution Profiles. The optional desktop/browser
  Adapter consumes only an explicit Service Link, uses an ephemeral numeric-
  loopback HTTP seam, and installs no extension/native host/proxy. Neither can
  alter system DNS, routes, default proxy/browser, VPN policy, or turn blocked
  Carrier networking into a direct fallback. R-052 currently selects no Stage
  7 isolated-browser profile; generic browser integration remains unverified
  and native controlled Applications carry the isolation campaign.
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

The complete small-slice contracts, M3 tags, acceptance criteria, frozen
oracles, dependencies, and exclusions are maintained in
[the S7.2 task-contract document](stage-7-s7.2-task-contracts.md). No M3
implementation starts until that document is jointly accepted.
The accepted fresh-attempt instructions for the first implementation slice are
the [S7.2-01 v2 M3 brief](m3-s7.2-01-v2-brief.md); the abandoned first attempt
remains `scope-blocked` and is not an implementation baseline.

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

### S7.3 Ubuntu Installed and Portable lifecycle

1. produce the manifest-bound Installed package and raw Portable executable target
   outside Git from the same declared Endpoint build;
2. run the Ubuntu 26.04 Docker package smoke and Portable target while
   inventorying visible paths/processes/registrations/privilege; record native
   Desktop/boot/filesystem/browser facts as `environment-deferred`;
3. exercise direct-binary use, offline and network start, Installed repair/
   update/rollback/restart, and Portable stopped replacement/deletion;
4. prove package-payload digest equality plus feature, Interface, resource,
   state-compatibility, and security/privacy-claim parity;
5. exercise empty/non-empty Vault removal and explicit purge;
6. compare actual residue to the predeclared cleanup inventory; and
7. run the complete Ubuntu A-D subset.

### S7.4 Windows Installed and Portable lifecycle

Repeat the exact shared outcomes with the selected Windows Adapters. Add no
Windows/profile-specific public outcome except safe diagnostic detail.
Explicitly test ACL owner, reparse/path behavior, file locking/open executable,
Installed registration, Portable no-install direct start and zero-registration
default, power-loss equivalent for Installed activation, repair, removal, purge,
parity, and residue.

Portable and non-install filesystem/API work may proceed on the current Windows
machine. MSI install/repair/update/uninstall/purge and installer-owned
registration stop until a separate Product Owner command names the artifact and
permitted cells. Pristine-host and destructive power-loss facts stay deferred.

### S7.5 Application Broker

1. preserve the existing external Application Interface semantics;
2. replace laboratory-only principal assumptions behind one broker Interface;
3. only after R-051/ADR-0016 acceptance, implement the exact
   [Application Principal specification](stage-7-application-principal-spec.md):
   private inherited channel, non-reusable root process handle, complete Job/
   cgroup tree, and activation before untrusted work;
4. test connection/admin/custody privilege lattice and resource parents;
5. attack with hostile same-user sibling, stolen/replayed capability, PID reuse,
   channel substitution, inherited handle, restart, and revoke/drain; and
6. run E evidence on both platforms.

Do not delete Stage 3 laboratory adapters until Stage 9 unless an accepted
cleanup scope proves they have no retained qualification role.

### S7.6 direct binary and Application Isolation

1. implement the accepted direct-binary, Ubuntu-isolation, and
   Windows-isolation Adapters behind their narrow Interfaces;
2. prove the same binary path works from Installed and Portable without a
   browser, extension, URI registration, or SDK;
3. launch controlled deterministic client/publisher Applications plus nested
   malicious helpers;
4. verify scoped IPC and per-context storage still work;
5. run every ingress/egress/child/escape/resource/restart/cleanup probe;
6. prove generic attachment reports the unqualified limitation; and
7. run E/F/G evidence, including all development platform pairings and both
   Distribution Profiles.

The accepted R-056 O1 topology also exercises its frozen generic browser
Adapter, origin/storage behavior, ordinary Internet/VPN coexistence, explicit
registration cleanup, and the side-effect-free unsupported isolated-browser
result. Failure or absence of browser integration cannot disable direct-binary
use.

Any escape is a candidate `fail`; harness/observer uncertainty is `invalid`.
Neither can become generic success inside the same cell.

### S7.7 combined development handoff

1. freeze source, dependencies, package tools, host images, roots, metadata,
   artifacts, config, manifest, seeds, observers, verifier, and cleanup inputs;
2. build each platform's Installed package and raw Portable executable target
   from clean source without runtime downloads and prove the package contains
   that exact executable digest;
3. run all fast/full checks and complete Stage 7 development matrix;
4. independently recompute verdicts;
5. retain immutable external evidence and remove all owned runtime artifacts;
6. document known limitations and exact H3/H4 claim ceiling; and
7. obtain Product Owner `advance-to-S8`, `redesign`, or `stop`.

The manifest commits to the exact 392-episode reference inventory from the
[development-host campaign specification](stage-7-host-campaign-spec.md) and
partitions every episode before execution. It cannot add a retry, move an
episode between coverage partitions, or drop a scheduled platform/Profile
tuple after seeing a result. Only `scheduled` episodes receive
`pass|fail|invalid`; pending or deferred coverage is never reported as success.

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
