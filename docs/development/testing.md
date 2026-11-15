# Testing

## Choose the verification route

Start with the affected current owner and the selected issue's acceptance
boundary. Use these routes instead of reading every scenario note below:

| Need | Route | Evidence limit |
|---|---|---|
| Edit one rule or lifecycle | Focused package/causal tests, applicable race; [ordinary checks](#ordinary-checks) and `make quick-check` while coding | Component behavior, not installed acceptance |
| Validate a PR delta | Existing affected selector and consumer closure under [ordinary checks](#ordinary-checks) | Selected impact; full integration gate remains required |
| Integrate a candidate | `make check` plus applicable [current profiles](#current-profiles) | Exact source/environment and declared profile only |
| Accept the installed product | [Qualification evidence](#qualification-evidence), owning native/installed profile and genuine supported commands | Full journey requires its real prerequisites and exact artifact |
| Investigate a dependency finding | [Dependency security evidence](#dependency-security-evidence) | Source review, causal component proof and native evidence remain distinct |
| Improve or remove a test | [Test design and retirement](#test-design-and-retirement), then measure actual cost | Preserve requirement, oracle, fault, platform and independence role |

[Profile definitions](../../tests/profiles/profiles.json) and
[ownership](ownership.json) remain the structural registries; this table adds no
runner or gate exemption. Missing selected prerequisites invalidate the profile.
Use [validity and reruns](#validity-and-reruns) for outcome classification and reruns; a later pass
never erases an earlier failure. Scenario notes below retain their stated scope.

## Selected release Carrier coverage

[ADR-0122](../adr/0122-select-tcp-only-closed-release.md) selects TCP/TLS alone
for the nearest closed release. Current positive Carrier/native/Service profiles
must execute TCP/TLS with all applicable causal refusal, lifetime, resource,
persistence and cleanup cases. Deferred QUIC requires explicit pre-effect refusal
and no TCP fallback; remove obsolete positive QUIC dispatch from checked profile
inventories rather than using skips or ignoring failures. Update discovery and
runner assertions together with their owning tests. Historical both-Carrier receipts describe their original candidate; current
implementation descriptions below cover the selected TCP/TLS release. Mandatory quick/full gates remain enabled. A changed source, profile
or canonical artifact requires new affected evidence; exclusion is not QUIC
security qualification or permission to change other protection parameters.


## Ordinary checks

Run Linux ordinary `unit`, `quick-check` and `check` as an **unprivileged**
user: enrollment/chmod-denial rows require that identity. Root is invalid there;
root-only Installation profiles have explicit separate selectors.

| Command | Required work and boundary |
|---|---|
| `make unit` | Positive deterministic inventory, including architecture/formatting and canonical Epoch fixture; serial packages, shuffle, fresh count, 15-minute package bound. The retained 256-stream Linux Endpoint setup remains included; timeout fails with Go's goroutine dump. |
| `make e2e` | Complete positive local-process inventory; serial ordinary packages and separately selected Node runner. |
| `make quick-check` | Unit, vet, command builds, module tidiness, canonical artifact representation and the selected installed/release-operation/Installation/Network tagged compile checks. `make format-check` is the focused architecture/formatting route. |
| `make check` | Complete quick-check prerequisites plus pinned-tool verification, Staticcheck, vulnerability and exact deadcode gates; process and race suites. Linux additionally runs `fixture-network-test` and root `package-e2e` (`dpkg`, `setpriv`); missing privilege is invalid. This is mandatory before integration; main push repeats it. |
| `make test-race` | Derived deterministic race inventory, serial 15-minute package bounds. Race slowdown is not throughput evidence. |
| `make headless-check` | Linux x86-64 exact Network artifacts, enrollment-v3/process evidence and fresh ownership extraction without Browser/Application implementation. Same owned sources must produce identical command bytes in Git and VCS-free form. |
| `make staticcheck` | `staticcheck ./...` uses the selected environment. Only ignored standalone Go tools use `GOOS=linux GOARCH=amd64 CGO_ENABLED=0`; the selector's two files are analyzed together. Cross-analysis supplies no native release-operation execution. |
| `make fuzz` | Selected State canonical Epoch/Node Record target, 30s mutation budget and digest/key/raw invariants. Missing/failed target fails. Architecture verifies inventory, declarations, entrypoint and exact commands. Ordinary deterministic runs execute seeds, not mutation fuzzing. |
| `make text-role-durable-state-capture` | Linux isolated publication/lookup/withdrawal observations in an existing private external `ARDENTS_TEXT_ROLE_OBSERVATIONS` directory; secret-bearing output stays local. Missing prerequisites invalidate the profile. |

PR CI's existing `scripts/select-pr-checks.go` selects changed Go owners, imported
consumers and registered non-Go fixture owners. Declaration and transitive race
requirements reach an order-independent monotone closure. Loading errors fail
before matrix publication; deleted packages without a current owner select the
architecture gate. Affected valid fuzz declarations run their seed corpus through
ordinary `go test -run`, not mutation fuzzing. Independent jobs all report their
failures; PR-green never substitutes for the exact candidate's `make check`.

Selected Ubuntu 24.04 Node jobs use `ARDENTS_NODE_SCOPE_PROFILE=system`: a real
manager and permission to create/stop scopes are mandatory. READY's cgroup and
sole PID, followed by joined empty-scope cleanup, are independently checked.
`NODE_PROCESS_TEST_COMMAND` selects this elevated runner only for Node; other
components remain unprivileged. Both process invocations stay serial and complete.
An ordinary shared-cgroup developer run retains that measurement limitation.
The full CI orchestration bound is 60 minutes including cold caches and race;
package/protocol bounds stay unchanged and cancellation is incomplete, never green.

Retained Publisher tests use real 4-by-64 loopback composition in the ordinary
Linux profile. Their race counterpart checks the same 256 bindings, identities,
concurrent Readers and readiness at an in-memory stream seam; these are distinct
evidence, not interchangeable latency measurements. Focused race runs retain
command/stdout/stderr/exit outside Git. Installed text-command-network output
uses an explicit owner-private external log; candidate/host inventories accompany it.

Ordinary checks do not build/run Docker or install tools implicitly. Vulnerability
checks need the [Go database](https://go.dev/doc/security/vuln/) or applicable cached
data. Setup and hooks remain in [CONTRIBUTING](../../CONTRIBUTING.md#local-setup).
Source/Control scenario details are [conditional profile evidence](testing/profiles.md#source-command-and-canonical-preparation-details),
not extra root operations hidden inside this ordinary route.


## PR matrix delivery

PR matrix delivery preserves each selected package/pattern/race group, its
existing 16-test ordinary bound, two-test expensive Endpoint bound and dedicated
observation cell. The [GitHub matrix limit](https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-syntax#jobsjob_idstrategymatrix)
is 256 jobs. Larger selections place consecutive unchanged groups into at most 256
jobs, with six minutes of outer envelope per group and no more than 60 groups
per job. Each original group still has Go's five-minute timeout plus the original
330-second external bound and ten-second final kill interval; the job envelope
is their aggregate, not extra time for an individual group. Parallelism remains 16.
Every selected group executes once in a successful run, including compilation
entries, fuzz seeds, race flags and diagnostic-parser prerequisites in any batch
position. A failed/interrupted group fails its job before another group starts
on that runner; other jobs retain independent completion. Each group's raw log
and failure classification remain distinct. This changes CI delivery geometry,
not product authority, protection, test selection or the pre-integration gate.


## Current profiles

[`tests/profiles/profiles.json`](../../tests/profiles/profiles.json) is the
checked registry. Every active profile has one real Make entrypoint and exact
prerequisites. Missing Docker, binaries, privilege, platform, host input, or
artifact is an invalid environment, never a skip or passing result.
The architecture gate checks a declared timeout against that Make target's
recipe and its target dependencies, so another profile's timeout cannot satisfy
the registry entry.

Select a checked profile by the affected requirement; a profile's presence is
not a completed run. Read only the applicable conditional reference:

| Affected boundary | Conditional reference and execution owner |
|---|---|
| Installation; Source/Forwarding/JOIN/Descriptor/Introduction lifecycle | [Installation verification](testing/installation.md); selected profile/Make target and [qualification operations](../operations/qualification-release.md) |
| Route framing, bootstrap, Publication and Connection | [Route verification](testing/route.md); [fixed-worker driver](../../tests/qualification/text-worker-lifecycle/README.md), [installed Endpoint](../../tests/qualification/execution-installed/README.md) and [Publication timing](../../tests/qualification/publication-timing/README.md) |
| Admission issuance/spend and Authority Custody | [Admission verification](testing/admission.md); registered bounded Linux profiles |
| Network preparation, Node process, platform and retained-command oracles | [Profile-specific verification](testing/profiles.md); [Node development recipe](../../scripts/node/README.md) |
| Native stand allocation, custody and disposal | [Native stand lifecycle](testing/native-stand.md); existing shared exclusive development wrapper |

These references retain unique scenario obligations and their evidence limits.
They are not copies of the profile registry or an execution ledger. Update the
owning reference with a changed oracle; record actual results in GitHub and the
external evidence bundle.
Profile, target, directory, build-tag, environment-variable, and test names use
domain language. Historical wire, persisted, release, and artifact identities
remain unchanged where their exact bytes are a compatibility obligation.

There is no generic live or soak profile. Selecting VPS, hostile-load, soak,
platform-matrix, release, or Firefox runtime qualification requires a new exact
product claim, environment, fixture, and Product Owner decision.


## Evidence interpretation

A named scenario is required coverage, not a current passing result. GitHub and
source-matched external receipts own execution/acceptance. Byte vectors establish
grammar; detached facts and failure-only callbacks supply no accepting authority.
Native file/process controls establish only the stated physical boundary, not
Release/State authority, manager action, domain join or installed continuation.
Process crash is not power-loss durability. Shared-controller fixtures cannot
establish independent builders, custody, families or whole-product privacy,
anonymity, availability, capacity or host qualification. Every complete journey
requires its own genuine consumers, causal matrix and declared prerequisites.

Cross-compilation/discovery is preparation. Installed execution retains its exact
accepted or delegated bootstrap scope under [agent execution](agent-execution.md).
The conditional references below specialize these limits; no omitted repeated
caveat expands the scope of a component test.

## Qualification evidence

A Qualification Evidence Bundle is an immutable, content-addressed record for
one exact candidate and its declared conditions. It retains precommitted
inputs, complete raw observations, invalidations, and deterministic verdict
outputs so the result can be recomputed. A selected log excerpt or ordinary
test report is insufficient. Public retention and change-impact requirements
remain NET-14AJ and NET-14AK in the
[requirements registry](../product/functional-map.md#accepted-requirements-registry).
An internal audit uses the separate [audit method](deep-audit.md) and cannot
substitute for independent review.


## Candidate boundaries

The C0 Network candidate is exercised by the deterministic/process/race lanes,
`headless-check`, and the selected Network qualifications. This includes the
Endpoint-owned `internal/application/broker`; its directory is not a separate
Application product. Protected text AAI3 and the separate Administration v1
Interface remain covered at their selected seams. The deleted AAI2 package and
vectors are neither a generic Endpoint workload nor compatibility surface. No
Browser command, Browser implementation, Browser artifact, or Browser
qualification is part of the current candidate.

ADR-0067 retires the completed release-seed and fixed State-genesis ceremony
commands, their deterministic writers, and their separate artifact/process
profile. R-119 through R-121 retain the exact historical result; current test
inventories contain no compatibility exception or hidden replacement route for
those writers.

[ADR-0091](https://github.com/dianabuilds/ardents-network/blob/c58268d72f1eaeae173144e76d32d50986de2a77/docs/archive/adr/0091-retire-uncomposed-legacy-artifacts.md) retired the
`tests/compatibility/browser-endpoint-v4` non-executable source tree that
ADR-0061 had retained pending an explicit supersede or retirement decision.
Completed experiments, the former `reference-c2` fixture, and the retired
compatibility tree are available only from Git history and accepted research
records.


## Historical qualification provenance

Earlier RC candidate verdicts and exact evidence limits are retained in the
[archived testing snapshot](https://github.com/dianabuilds/ardents-network/blob/c58268d72f1eaeae173144e76d32d50986de2a77/docs/archive/development/testing-before-consolidation.md#historical-qualification-provenance).
They are not current profile results and cannot be aggregated into a later C0
claim. Required current profiles and candidate bounds remain in this owner.


## Ownership

Unit and single-Module integration tests live beside their implementation.
They replace real time or external I/O only at the owning seam and do not start
Docker. Cross-process tests live under `tests/e2e/<behavior>/`; they build real
named commands, observe public behavior, create fresh temporary fixtures, and
clean every owned process and file.

Every maintained Go package belongs to the deterministic inventory. Every
maintained repository file—including source/test, command, build and CI input,
documentation, packaging, profile, qualification, and retained evidence—plus
every qualification and artifact lane matches exactly one ownership rule.
Environment-owned `.codex-*`, `.git`, and editor state are not repository
inputs and are excluded from the walk. Every Go-bearing `tests/e2e` suite root
belongs to exactly one process profile. A new package, file, or suite cannot
enter through a negative filter, wildcard exception, or directory naming
alone.

Non-executable compatibility provenance lives in Git history and accepted
research records, not in the working tree. Compatibility evidence must name
its former observer and deletion/reactivation condition and does not belong to
a maintained package inventory.


## Validity and reruns

An environment-dependent run has four possible outcomes: product assertion
failure, harness defect, invalid environment, or nondeterministic/unowned
result. None is green until resolved.

Rerunning is diagnostic evidence and never erases the first failure. There is
no automatic retry, permanent quarantine, skip allowlist, or flake budget. A
timeout must leave enough observed state to distinguish product, harness, and
environment.


## Test design and retirement

Platform-specific tests require a concrete mechanism justification in their
owning source or technical owner: name the native operation or guarantee, the
behavior being checked, and why the common test cannot exercise it. A filename,
an inherited build tag, or the current platform of an enclosing package is not
that justification. Common framing, queues, credit, byte accounting,
cancellation and joined lifecycle use the same behavioral oracles on Windows
and Linux. A test isolated only because these mechanisms are coupled to a
platform adapter must move with their portable owner rather than retain that
coupling as an accepted platform requirement.

Native socket-error controls may distinguish Winsock from POSIX errno values.
Durable integration tests may require the selected exclusive file lease,
permission checks and directory-sync/rename guarantees. Keep such mechanisms
with their actual owners and do not replace unavailable guarantees with a
successful in-memory fixture. Portable component execution, cross-compilation,
full new-domain integration and installed qualification remain separate evidence.

Stateful Modules own injectable wall-clock, monotonic duration, entropy, and
private fault seams at the smallest real owner. Fixed sleeps are not readiness
or eventual-consistency oracles. Every goroutine, process, listener, file, lock,
timer, and fixture has one owner, cancellation path, join, residue assertion,
and cleanup-failure path.

The fixed stream qualification uses a qualification-only Run seam: its report,
observer callbacks, sampling cleanup and joined streams never belong to an
ordinary text Job. Its cancellation oracle delays an old Run's completion
until a replacement exists, then proves the completion remains on the old Run
and that the first sampling cleanup error is immutable.

Two tests are duplicate-removal candidates only when requirement, owning seam,
oracle, transition/fault, platform/format, and independence role are all the
same. A seam migration moves behavior tests to the new Module Interface,
retains only higher-level tests that prove a distinct fact, and retires obsolete
fixtures and runners in the same change.

Security vectors are deterministic. Shuffle and randomized schedules publish
their seed. Fuzz/property coverage belongs beside each retained untrusted
decoder and canonical encoder. Race coverage is required wherever a Module owns
goroutines, callbacks, locks, mutable admission, cancellation, or durable state,
and must assert terminal join and state invariants in addition to race-detector
cleanliness.


## Dependency security evidence

The [dependency acceptance rule](dependencies.md#maintenance-and-vulnerability-acceptance)
covers current and future components and their continued support.
`make vuln` currently runs `govulncheck ./...` for the selected environment;
it does not by itself cover every target, tests, build tool or delivered
artifact. State exactly which of those were inspected. Review source and
test closures for the applicable `GOOS`, `GOARCH`, `CGO_ENABLED`, build tags
and Go version; use `-test` when assessing test dependencies. Inspect the exact
built artifact and its package/tool/environment inventory when qualifying
delivery. Cross-target source analysis does not execute that target's tests
or qualify its artifacts.

Record the full finding classification, including imported-package and
module-only findings, and the dated non-applicability evidence required by the
register. The pinned scanner's
[official documentation](https://pkg.go.dev/golang.org/x/vuln@v1.1.4/cmd/govulncheck)
(accessed 2026-09-07) describes build-specific coverage and limitations around
reflection, `unsafe` and binary call information. Structured output modes can
return success despite findings, so exit status alone is not a verdict for
such reports. Source checks, exact-artifact inspection, primary advisory review
and the relevant behavior tests provide complementary evidence; none proves
the absence of unknown vulnerabilities or continued upstream maintenance.
Required `make check` gates remain binding.

The successor [privacy/anonymity map](privacy-anonymity-map.md#verification-and-test-environment-map)
defines additional evidence obligations for a selected new scheme. These are
design and qualification requirements; no new execution profile or automated
anonymity verdict exists merely because the map names them.


### QUIC return-path reply-budget evidence

The Product Owner's 2026-10-15 methodology decision for
`ARDENTS-QUIC-2026-09-21-RETURN-PATH-REPLY-BUDGET` permits composite evidence;
active native/OOB attack reproduction is not the sole admissible method.
That decision changed the proof method, not the RFC 9000 bound or dependency
acceptance rule. The later ADR-0122 selects TCP/TLS alone for the nearest release:
positive TCP/TLS and explicit pre-effect QUIC refusal remain required. The method
below applies to a future explicitly selected QUIC candidate, not current QUIC
execution or automatic dependency adoption.
The [finding disposition](dependencies.md#existing-quic-finding-return-path-reply-budget)
still requires a supported fix or enforceable reviewed non-applicability for
all affected admitted uses. The method itself supplies neither.

For a fixing candidate, retain and review all of the following:

- The actual supported upstream correction and exact maintained source,
  module, toolchain, configuration, build and canonical artifact identities.
  Establish that the built Ardents actually includes the reviewed correction;
  release notes or a similar patch title alone are insufficient.
- Deterministic causal regressions of the real dependency component at its
  actual send-admission boundary, using the independent RFC 9000 oracle and
  a control distinguishing original and corrected behavior. Cover cumulative
  per-address allowance before output and validation transitions. A separate
  behavioral model, copied implementation or first-party counter is no substitute.
- A complete caller table with source/build identity, server/client role,
  TLS/ALPN, configuration, socket/backend and supported platform. Group only
  callers whose relevant behavior is demonstrably equivalent. Prove the common
  mechanism once per equivalent group, retain controls detecting a divergent
  listener, and cover each distinct path separately. Retained predecessor and
  new isolated Route consumers have separate closure; no migration bridge or
  automatic predecessor repair follows from their inventory.
- Closure over actual output accounting, including padding, retransmission,
  segmentation, backend fallback and other send paths. Explain why these paths
  preserve the reviewed component's admitted byte count and destination. An
  unexplained backend or bypass remains unresolved; a source-only assumption
  or successful ordinary handshake cannot supply its missing proof.
- Ordinary source-matched native checks of genuine authority, Receiving/Hosting,
  original process/producer/TLS custody, real QUIC Service/role consumers for the
  proposed return and retained TCP/TLS regressions,
  finite bounds, cancellation, late work and joined physical resource return.
  Preserve role-specific authority/spend/lifetime checks and affected race
  coverage even when a shared transport mechanism is proved once.

For non-applicability, retain the dependency owner's complete enforceable-boundary
argument and real causal regression instead of assuming an upstream repair.
Every admitted caller/platform/backend still requires explicit coverage.
Preserve failures, historical observations and exact supersession of any earlier
method. Label inspection, component execution, ordinary native integration and
direct wire/OOB measurement separately. The composite proof does not claim a
direct native packet measurement; absent measurements stay absent. Changed
dependencies, artifacts, configuration, callers or backend assumptions invalidate
affected receipts. Full required gates, the normal commit hook and scoped verified
delivery remain mandatory; passing this method's prerequisites does not close the
finding. It authorizes no active attack reproduction, host change or privileged
experiment.

## Reachability audit

`make deadcode` runs `golang.org/x/tools/cmd/deadcode` for the maintained
Windows and Linux production builds, then for their test executables. The test
result must be empty. Production results must match
[`tests/profiles/deadcode-allowlist.json`](../../tests/profiles/deadcode-allowlist.json)
exactly: an entry names every retained symbol, its classification, rationale,
owner, and deletion condition. Prefix, package, or count-based exceptions are
not permitted.

Most retained production entries are intentionally unwired closed-alpha
tracers with deterministic behavior evidence; this is not a claim that they
are a selected product path. A newly reported symbol fails the gate until its
owner either removes it or reviews it with a concrete retirement condition.
An absent listed symbol also fails the gate, so the registry cannot silently
accumulate stale exemptions. `make check` includes this audit.

The headless command inventory declares the four-command Network artifact lane;
the text-worker inventory declares its separately owned Application lane under
`tests/profiles/`. Architecture tests check actual transitive dependency
graphs, the explicit [`ownership.json`](ownership.json) registry, exact
qualification/artifact-lane ownership, and every maintained package and suite.

The AAI2 retirement closure receipt has source `go list -deps -json`, profile
`GOOS=linux GOARCH=amd64 CGO_ENABLED=0`, and command `./cmd/ardents`. Its scope
is limited to repository-owned production Go files in that one Linux command
dependency closure. The architecture gate rejects the removed
`internal/application/interfacev1/connection` dependency and every exact
`RunParticipant` identifier while requiring the selected AAI3 Connection and
Administration dependencies to remain. A temporary local module with the
forbidden import and symbol proves that the gate fails on their return. This is
not an all-platform, all-command, supply-chain, privacy, or release
qualification.

Canonical command builds use the non-overridable
`-trimpath -buildvcs=false -ldflags=-w` policy. Distributed command artifacts
omit DWARF while retaining symbol tables and Go stack/profile metadata.
Installation still freshly compares every byte of the exact signed artifact;
omitting debug sections reduces that repeated work without changing its
integrity, authority or deadline checks. Source-level debugger use requires a
separately built development artifact; it cannot substitute for the pinned
installed command. The public Make proof builds all four current
headless artifacts from two independent normal clones, a linked worktree, and
two independent VCS-free ownership extractions,
including the Endpoint source dependencies owned by the selected protected text
Connection and Administration Interfaces and text-application without changing
their ownership or the artifact lane,
then requires byte-identical
outputs without implicit `vcs.*` settings. Source revision and builder
provenance remain explicit authenticated release-metadata,
attestation, and receipt inputs; command bytes never infer those facts from the
builder's clone or linked-worktree representation.

Current platform-specific allowance explanations and retained-command oracles
are in [profile-specific verification](testing/profiles.md#production-reachability-classifications).
The exact registry remains authoritative; a historical no-consumer explanation
cannot keep a newly reachable or removed symbol exempt.


## Local diagnostic environment

The explicit [protected installation capture driver](../../tests/qualification/protected-installation/README.md)
invokes public provision and installation-check, then asks the admitted system
manager to start the actual installed unit. It retains the first command failure
without retry or reset. This is a preparation/capture surface, not the complete
two-Endpoint Service oracle or evidence that any admitted host was exercised;
manager-start success alone does not establish participant readiness. Its
independent program digest and genuine operator request are selected prerequisites.

The [local diagnostics owner](local-diagnostics.md) and its
[recipes](../../scripts/diagnostics/README.md) define the explicit Docker tools,
private live process mode, event/resource collection, race/profile separation,
source identity, failure/loss and privilege limits. `make diagnostics-build`
is explicit setup; `make diagnostics-check` exercises the Linux collector.
The Linux architecture deterministic profile runs that collector's meaningful
negative/terminal evidence tests without Docker. Process diagnostic behavior
belongs to `internal/diagnostics/process` in the deterministic/race profile.


## Scenario reference links

The following stable fragments route older links to their current conditional owner.
They add no separate profile or acceptance.

<a id="disposable-development-stand-lifecycle"></a>
- [Disposable development stand lifecycle](testing/native-stand.md)

<a id="preliminary-gate-a1-regression-coverage"></a>
- [Preliminary Gate A1 regression coverage](testing/profiles.md#retired-command-regressions)

<a id="route-mechanism-and-composition-tests"></a>
- [Route mechanism and composition tests](testing/route.md#route-composition-and-causal-controls)

<a id="independent-offline-admission-profile"></a>
- [Independent offline Admission profile](testing/admission.md#offline-profiles)

<a id="portable-route-public-boundary-controls"></a>
- [Portable Route public-boundary controls](testing/route.md#portable-route-public-boundary-controls)

<a id="new-installation-generation-authentication"></a>
- [New Installation generation authentication](testing/installation.md#new-installation-generation-authentication)

<a id="bounded-authority-custody-native-lane"></a>
- [Bounded Authority Custody native lane](testing/admission.md#bounded-authority-custody-native-lane)

<a id="original-direct-origin-source-development-profile"></a>
- [Original Direct-Origin Source development profile](testing/profiles.md#original-direct-origin-source-development-profile)

<a id="forwarding-local-protection-during-held-authority"></a>
- [Forwarding local protection during held authority](testing/installation.md#forwarding-local-protection-during-held-authority)

<a id="join-offline-public-artifact-custody"></a>
- [JOIN offline public artifact custody](testing/installation.md#join-offline-public-artifact-custody)

<a id="read-only-fixed-node-slot-observation"></a>
- [Read-only fixed Node slot observation](testing/installation.md#read-only-fixed-node-slot-observation)

<a id="join-original-delivery-and-read-only-preflight"></a>
- [JOIN original delivery and read-only preflight](testing/installation.md#join-original-delivery-and-read-only-preflight)

<a id="join-offline-release-production"></a>
- [JOIN offline Release production](testing/installation.md#join-offline-release-production)

<a id="join-public-declaration-preparation"></a>
- [JOIN public declaration preparation](testing/installation.md#join-public-declaration-preparation)

<a id="initial-stopped-join-native-boundary"></a>
- [Initial stopped JOIN native boundary](testing/installation.md#initial-stopped-join-native-boundary)

<a id="join-stopped-replacement-recovery"></a>
- [JOIN stopped replacement recovery](testing/installation.md#join-stopped-replacement-recovery)

<a id="descriptor-role-offline-public-preparation"></a>
- [Descriptor-role offline public preparation](testing/installation.md#descriptor-role-offline-public-preparation)

<a id="descriptor-original-input-preflight"></a>
- [Descriptor original-input preflight](testing/installation.md#descriptor-original-input-preflight)

<a id="descriptor-initial-stopped-native-profile"></a>
- [Descriptor initial stopped native profile](testing/installation.md#descriptor-initial-stopped-native-profile)

<a id="descriptor-first-start-native-profile"></a>
- [Descriptor first-start native profile](testing/installation.md#descriptor-first-start-native-profile)

<a id="introduction-offline-public-artifact-custody"></a>
- [Introduction offline public artifact custody](testing/installation.md#introduction-offline-public-artifact-custody)

<a id="introduction-declaration-and-read-only-input-controls"></a>
- [Introduction declaration and read-only input controls](testing/installation.md#introduction-declaration-and-read-only-input-controls)

<a id="introduction-installed-exchange-input-preparation"></a>
- [Introduction installed exchange input preparation](testing/installation.md#introduction-installed-exchange-input-preparation)
