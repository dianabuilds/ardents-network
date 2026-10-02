# Testing

Ardents separates deterministic Module checks, local process behavior,
artifact profiles, and explicitly selected qualifications. A historical result
is evidence about its exact candidate; it is not an entrypoint for current
code.

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

## Ordinary checks

- `make unit` runs the positive deterministic package inventory, including the
  canonical Network epoch fixture, with one explicit 15-minute terminal timeout
  per package. The retained 256-stream
  Linux Endpoint setup stays in that profile; exceeding the bound fails with
  the Go runtime's goroutine dump.
- `make e2e` runs the positive local process package inventory.
- `make quick-check` runs the unit inventory, including architecture and
  formatting checks, plus vet, the four named command builds, module tidiness,
  and the canonical artifact representation proof. `make format-check` remains
  available as a focused architecture and formatting check.
- On Linux x86-64, `make headless-check` builds the exact Network command
  inventory, checks the enrollment-v3 artifact, runs bounded Endpoint, Source, Node, and Service
  process evidence, then rebuilds and tests the headless command candidate in
  a fresh temporary tree containing no Browser/Application implementation. It
  also proves that canonical command bytes are unchanged when the same owned
  source is represented as a Git repository or as a VCS-free extraction.
- `make check` runs unit, process, race, command build, formatting,
  Staticcheck, and vulnerability checks. On Linux it also runs the bounded
  two-host qualification fixture command tests through `fixture-network-test`;
  invoking that target elsewhere fails with an invalid environment. This is
  the pre-integration gate. Pull request CI uses `scripts/select-pr-checks.go`
  to run the changed Go owners,
  their imported consumers, and explicitly registered non-Go fixture owners;
  affected valid Fuzz declarations execute their deterministic seed corpus through
  ordinary `go test -run` in direct execution and split matrix jobs. PR selection
  does not enable mutation fuzzing, and unrelated checks remain excluded.
  declaration selection and transitive race requirements reach a monotone closure
  independently of package visitation order, including already selected consumers.
  Package and dependency loading errors fail selection before a matrix is
  published. A deleted Go package without a current owner selects the repository
  architecture check; changed surviving consumers retain their narrow checks.
  Independent selected jobs all finish and report their failures. The exact
  candidate must still pass `make check` before integration, and a push to
  `main` repeats that complete gate.
- `make staticcheck` also runs `standalone-staticcheck`: each maintained ignored
  Go entrypoint in `scripts/` is analyzed separately with `GOOS=linux`,
  `GOARCH=amd64` and `CGO_ENABLED=0`. The select-pr entrypoint uses its two
  source files together. This cross-analysis can run on a Windows host and
  does not execute or qualify Linux release operations.
- Focused Linux race checks retain raw command, stdout, stderr, and exit status
  outside Git. They may compose existing owner tests for a bounded lifecycle
  fact, but do not turn that composition into an end-to-end qualification.

Linux Node command tests can select `ARDENTS_NODE_SCOPE_PROFILE=system` to
place every serving Node in its own systemd scope. The selected profile requires
a running system manager and permission to create and stop system scopes;
missing prerequisites fail. The runner verifies the READY process's cgroup and
sole PID, then joins and verifies scope cleanup. Selected Node process jobs in
Ubuntu 24.04 PR CI use this profile because worker manager queries must not
share the Node's measured cgroup. Ordinary developer-host execution without
this selector retains its shared-cgroup limitation. Scope verification alone
does not qualify installed workers, a second Endpoint, or whole-host protection.
The complete Ubuntu CI gate selects the same profile through
`NODE_PROCESS_TEST_COMMAND`; only the Node process package uses that runner.
The other process packages and the unprivileged component checks retain their
ordinary runner. Both process invocations remain serial and run every package
in the declared process profile.
The full Ubuntu CI job has a 60-minute orchestration budget for the sequential
complete gate, including cold compiler caches and the race suite. The Go test
timeouts and required checks remain unchanged; cancellation is an incomplete
gate, never a passing result.

Installation filesystem component checks select
`ARDENTS_INSTALLATION_COMPONENT_PROFILE=root` and run
`go test -count=1 ./internal/endpoint/installation` with actual UID 0.
The selected prerequisite test fails under a non-root UID; an ordinary
unprivileged run cannot supply Root filesystem evidence. In Docker explicitly
select `--user 0` and the profile environment variable rather than inferring
privilege from the image name. Enrollment's caller-owned package test retains
its separate unprivileged profile. Root filesystem checks do not qualify a
system manager, cgroup containment or an admitted installed Ubuntu journey.
The completion-channel component also requires writable `/run` and permission
to spawn a child with kernel UID/GID 65534 and cleared supplementary groups.
The child connects through the Root-owned directory and socket, while actual
kernel permission refusals prevent private-intent reads and directory writes.
Unavailable credential-switch privileges fail this selected profile. This is
local Unix credential/access evidence, not systemd namespace qualification.

The two-Node lifecycle process test retains TCP port reservations throughout
State and plan preparation. It releases each reservation immediately before its
Source or Node starts, while other addresses remain owned. This prevents its
own ephemeral allocator from assigning a Node address to a Source. The ordinary
child bind still has a release-to-bind gap in which an external process could
claim that address; bind failures remain failures without a retry or waiver.
These reservations do not qualify installed networking or systemd/cgroup use.

- The retained text Publisher setup keeps its real 4-by-64 authenticated
  loopback composition in the ordinary Linux profile. Its race counterpart
  checks the same 256 workload bindings, retained identities, concurrent Reader
  ownership, and ready barrier at an in-memory stream seam; race-detector
  slowdown is not a wall-clock throughput verdict for the loopback fixture.
- Its serial race inventory gives each package an explicit 15-minute terminal
  timeout. This keeps the race-instrumented Linux Endpoint's cryptographic
  fixtures inside the checked profile without inheriting Go's shorter default;
  exceeding the bound remains a failure with the runtime's goroutine dump.
- `make text-role-durable-state-capture` runs the Linux-only isolated Role
  publication/lookup/withdrawal observation. It requires
  `ARDENTS_TEXT_ROLE_OBSERVATIONS` to name a writable capture directory outside
  Git; the test writes secret-bearing raw observations there and retains them
  locally. A non-Linux host or missing capture directory is an invalid profile,
  not a passing skip.
- The installed `text-command-network` runner can retain its complete raw
  output in an explicitly supplied owner-private directory outside Git. It
  prints the file path before the long-running test and keeps failures and
  timeouts for diagnosis; the caller retains candidate and host inventories
  separately.
- `make fuzz` mutation-fuzzes the selected State target for a bounded
  30 seconds. State owns canonical Epoch/Node Record framing and checks
  successful parser digest/key/raw invariants. The command fails if the named
  target is absent or fails, and does not claim coverage of every untrusted
  grammar. The deterministic architecture profile independently
  checks the selected inventory, its declarations, the Make entry point, and
  the exact mutation commands. It does not mutation-fuzz under an ordinary
  deterministic profile.

Ordinary checks do not build or run Docker and never install tools implicitly.
`make check` also verifies pinned tool versions and the dead-code inventory.
Its vulnerability check uses the [Go vulnerability database](https://go.dev/doc/security/vuln/)
and requires network access unless the needed data is already cached. On Linux,
it additionally runs `package-e2e`, which uses root to exercise `dpkg` and
`setpriv`; missing privilege invalidates that selected profile. Dedicated
Rendezvous qualifications have their own targets and prerequisites. Local
setup and hook installation belong in [CONTRIBUTING.md](../../CONTRIBUTING.md#local-setup).

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

The selected installed text Endpoint and its context, token-journal, protected
publication and read component tests compile on Linux. Run those behavior
checks in the existing Linux deterministic/race profiles; a Windows unit pass
does not execute or qualify them. Windows retains the pre-existing Endpoint
journey and an explicit unsupported text-command refusal. Shared State,
Custody, Node and protocol consumers keep their own platform contracts. The
[measured Linux Endpoint test cost](https://github.com/dianabuilds/ardents-network/blob/4764ae1c567e93180f3aa643b2544abfdb2a67dc/docs/development/endpoint-test-cost.md) records one
development baseline and the deliberate workload and Permission-hour waits;
it does not change this checked profile. The Endpoint-originating Route
bootstrap/prefix client and its stream, credit,
JOIN and network-issuance tests also execute in the Linux profiles. The
Endpoint role-network fixture names its selected Carrier, resolution, Publisher,
and JOIN roles explicitly; it substitutes accepted State and worker qualification
while starting the actual Node runtimes, Custody allocation, token stock,
Route forwarding, and role TLS. Its reserved-window variant is used only when
the child observation process inherits a Permission hour selected by its parent.
On a failed test, the fixture reports the last 16 bounded state events per
Node after their cleanup attempts. Periodic resource samples cannot displace
the transition history; the retained fields are time, kind, state, Carrier,
and reason, without resource samples or assignment material.
Receiving Node checks that do not require that client remain on both platforms. Codec
round trips, private-capsule cryptography and network tests that construct
Endpoint operations execute with the Linux client. Shared outer-handshake
admission and receiving-listener address/certificate fixtures remain separately
available to the platform-independent Node tests.
The Linux Node deterministic/race checks run actual TCP/TLS and QUIC forwarding
open success, refusal, cancellation, and concurrent same-key one-HELLO cases.
The Linux parent-reader regression holds one downstream HELLO, then proves a
distinct selected TCP child and lane-zero control progress; its pending CLOSE
variant observes the first carrier close before accepting that result. Route
queue bound and Node Stop/invalidation tests remain separate owner evidence,
not one end-to-end lifecycle qualification.
Portable session checks retain blocked-HELLO/ready-unrelated progress and
independent waiter/creator cancellation ownership.
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

The dedicated Initiator closure audit removed the production-dead Entry
admission engine, receiving relay grammar, and direct credential/reachability
OHTTP adapters. The User Route closure oracle additionally requires the
uncalled Open/Attach owner, private reachability exchange, and exclusive relay
and Introduction sender files and declarations to remain absent. It also names
the shared Attachment evidence and current Introduction receiver surfaces that
must remain. Endpoint and Node behavior tests may use bounded test-local
reciprocal fixtures; those fixtures do not restore production reachability.
Shared credential-relay codec leaves and the standalone reachability Relay
remain exactly classified under their current consumers in the deadcode
registry.

The old Node-leg dial, TCP/QUIC client adapters, and client confirmation
entrypoint are absent. The retained reciprocal decoder remains in its exact
reviewed grammar group. Current closed TCP/TLS and QUIC tests prove exact peer
rejection, cancellation before handshake, caller-owned lifetime after a
completed handshake, and unchanged Carrier profile bounds.

Removing the old cross-platform Rendezvous listener once exposed the shared
server-side Carrier and admission closure as production-dead. The current
analysis reaches it through the composed `ardents-node node --config` command,
and ADR-0101 resolved the last two Node entries that still carried the stale
"no selected C0 command" claim: the `dutyFacts` DutyView test-seam projection
moved into test scope and the unimplemented local admission-timeout helpers
were retired, so `internal/node` holds no common deadcode allowance.
ADR-0104 subsequently replaced the getter seam itself with one copied
`state.NodeDuty` value, deleting the test-scope projection along with the
Interface it satisfied.
Additional shared framing leaves are unreachable only in the Windows
projection and remain in its platform allowance because Linux production
still has retained Route consumers. That allowance does not permit removing
or reconnecting shared mechanics in the one-engine retirement slice.

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
`-trimpath -buildvcs=false` policy. The public Make proof builds all four current
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

## Preliminary Gate A1 regression coverage

The current deterministic and race inventories retain the accepted preliminary
Network authority/lifecycle repairs:

- the alpha-corpus diagnostic cannot accept or create an Endpoint floor, while
  the separately enrolled acceptance command remains the sole floor mutator;
- the enrollment package, Windows behavior tests, and both alpha-bundle scripts
  share the canonical `.exe` control-artifact identity;
- revoked, draining, or closed Connection admission reaches no State, Entry,
  issuer, Route, or Introduction work; pending capability expiry does not end
  active Endpoint work; Connection and Administration budgets are separate;
  and active sessions are canceled, drained only under a permitted finite
  non-extendable bound, and released once;
- Node rejects old-generation Entry and Transit Grant admissions immediately
  after a successor and reports terminal cleanup faults without publishing
  `WITHDRAWN`; and
- concurrent caller and Route Attachment close join one exactly-once cleanup
  and return the same terminal result; and
- Entry close cancels acquisition, joins and terminalizes active attachment
  cleanup before releasing its root, including concurrent and failure cases.

The command tests exercise the retired `accept-alpha-corpus` and
`inspect-alpha-corpus` refusals before effects. These are regression checks
for the repaired candidate, not completion of the formal deep audit or
authorization to begin its later security, concurrency, or wire tracks.

## Current profiles

`hosting-budget-linux` runs `make hosting-check`: isolated durable Hosting and
compiled command lifecycle with race checks on Linux amd64. The pinned Go 1.27.1
bookworm image already selected for diagnostics can provide this component
environment with read-only sources and caches outside Git. It requires actual
proc/sys counters, loopback, flock, rename, fsync and a race-capable C compiler;
non-Linux or missing prerequisites fail. This is not provider invoice,
installed Endpoint, network-role or C0 qualification evidence.

[`tests/profiles/profiles.json`](../../tests/profiles/profiles.json) is the
checked registry. Every active profile has one real Make entrypoint and exact
prerequisites. Missing Docker, binaries, privilege, platform, host input, or
artifact is an invalid environment, never a skip or passing result.
The architecture gate checks a declared timeout against that Make target's
recipe and its target dependencies, so another profile's timeout cannot satisfy
the registry entry.

The maintained local profiles are:

- developer, deterministic, process, package-e2e, headless-network, race, and fuzz;
- `qualification`, the aggregate selected Ubuntu Endpoint lifecycle profile;
- `endpoint-portable-ubuntu` and `endpoint-replacement-ubuntu`;
- `service-credential-response-linux`;
- `text-role-durable-state-capture`, the Linux-only isolated Role and Publisher
  durable-state observation; it retains secret-bearing raw observations only in
  an existing, writable, non-symlink directory outside the Git worktree. State,
  OS placement and pressure/owner-use are explicit component models: participants
  share a container cgroup, so its departing processes cannot supply isolated
  Role owner-use evidence. Duties still run through Node with real durable
  hosting allowance accounting. This profile does not qualify systemd/cgroup
  placement, resource use, latency or full P3 protection;
- the retired two-Endpoint alpha-control runner is no longer a contributor
  route. Its historical receipts remain in their existing evidence locations;
  its last source revision is retained in Git as `6ac9cba0856a7a0b92f2e8a4e11b246625f6716d`;
  current enrolled-bundle and inspection regressions stay with
  `headless-evidence` and its owned package tests;
- `text-worker-network`, the installed confined worker/network composition invoked
  by `make text-worker-network-check`; requires all eight document/Carrier and elapsed-refresh cases,
  exact invocation evidence and terminal service success. State and authority setup
  remain explicit fixtures; ordinary command and full host qualification are separate.
- `text-worker-recovery`, the installed confined-worker recovery smoke invoked by
  `make text-worker-recovery-check`; it interrupts one accepted request over each
  Carrier, requires one fresh matching Route and joined worker/Grant/cgroup cleanup.
  Its two episodes do not establish NET-14 percentiles, impairment, directional
  byte/bitrate accounting, P10 restart safety or complete Route Qualification.
- `text-command-network`, the separately pinned installed ordinary-command journey
  invoked by `make text-command-network-check`; it exercises the real `ardents`,
  `ardents-custody`, `ardents-node` and `ardents-text` candidate binaries through
  both Carriers, with exact empty, 64 KiB and 4 MiB command cases. It supplies
  journey evidence with each Node's separate systemd scope and actual single-
  process membership verified at READY, joined empty-scope cleanup, and no
  observed topology loss of READY before completion. Orchestrator commands
  remain outside Node pressure owners. This is not continuous-duty evidence.
  It does not qualify full host, privacy, hostile-network or p95 behavior.

- `text-worker-policy`, the root-driven installed authorization matrix invoked
  by `make text-worker-policy-check`; it does not qualify the complete host.
- `text-worker-lifecycle`, the separately selected installed Endpoint launch/Grant/
  cgroup profile invoked by `make text-worker-lifecycle-check`; its tagged test binary
  and exact temporary unit must be independently pinned. No-tests success is
  refused; this profile does not replace hostile-worker or Service journey tests.
- the fixed stream-qualification worker's deterministic scheduled-byte corpus is
  checked locally for all 256 byte values (including NUL and invalid UTF-8), exact
  verification across unequal fragment boundaries, wrong-offset/corrupted-byte
  refusal and a workload-verdict failure after one-byte truncation. This is
  binary-corpus conformance for the pinned laboratory caller, not an installed
  profile result or support for arbitrary Applications.
- `text-worker-tree`, the separately pinned installed hostile child/grandchild cleanup
  profile invoked by `make text-worker-tree-check`; it requires both role receipts,
  original-cgroup cleanup and independent Publisher sibling snapshot progress. It
  does not replace the escape matrix, protected Route admission or full host
  qualification.
- `text-worker-escape`, the separately pinned installed P6/P7 escape matrix invoked
  by `make text-worker-escape-check`; it tests hostile worker access attempts under
  the effective selected unit policy and does not establish whole-host qualification.

Profile, target, directory, build-tag, environment-variable, and test names use
domain language. Historical wire, persisted, release, and artifact identities
remain unchanged where their exact bytes are a compatibility obligation.

There is no generic live or soak profile. Selecting VPS, hostile-load, soak,
platform-matrix, release, or Firefox runtime qualification requires a new exact
product claim, environment, fixture, and Product Owner decision.

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

[ADR-0091](../adr/0091-retire-uncomposed-legacy-artifacts.md) retired the
`tests/compatibility/browser-endpoint-v4` non-executable source tree that
ADR-0061 had retained pending an explicit supersede or retirement decision.
Completed experiments, the former `reference-c2` fixture, and the retired
compatibility tree are available only from Git history and accepted research
records.

## Historical qualification provenance

Only the concise verdicts below remain in the current documentation surface.
Detailed runners, process chronology, and split-candidate ledgers are historical
Git material.

| Exact source | Retained result | Limitation |
|---|---|---|
| [`70bf425eec937edcc22e8f0534db992aa2002a16`](https://github.com/dianabuilds/ardents-network/commit/70bf425eec937edcc22e8f0534db992aa2002a16) | Historical RC1 supplied A1-A10 bounded project-operated evidence. | It has no A11/A12 closure and does not qualify C0. |
| [`2c18bdf92f11f84075915576f595202f48eb05bc`](https://github.com/dianabuilds/ardents-network/commit/2c18bdf92f11f84075915576f595202f48eb05bc) | Historical RC2 supplied a separate two-fresh-Endpoint control result and accepted A11 campaign. | It does not inherit RC1 A1-A10; no candidate has an aggregate A1-A12 result. |
| [`fbb42034757513ac009114a00b933aefa76d8ddf`](https://github.com/dianabuilds/ardents-network/commit/fbb42034757513ac009114a00b933aefa76d8ddf) | Last source snapshot containing the retired experiment implementations, `reference-c2`, generic Update tracer, stage runners, and planning chronology. | Provenance only; none of those sources qualifies or belongs to the cleaned C0 candidate. |

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

The closed text Endpoint's durable Entry Set owner and its persistence and
class-specific adjacency tests compile on Linux, matching their sole production
consumer. Legacy Entry Invite validation and retained admission-history
decoding remain portable; receiving-admission checks were retired with the old
Initiator closure.

Holder-request construction and its end-to-end Custody allocation tests run on
Linux with the text Endpoint client. The shared request decoder, proof verifier,
Custody command and authority storage remain portable; independent authority
creation and allocation-journal tests stay in the portable profile.

The closed blind-token client batch and result round-trip tests run on Linux
with the Endpoint that retains the opaque pending state. Issuer request parsing,
signature checks and fixed result encoding remain available to receiving Nodes.

Private Descriptor issuance round trips and Store tests that produce those
proofs run on Linux with the Publisher. Independent receiving Gateway and wire
checks remain portable. Protected context and coalesced initial-authentication
tests run with the Linux text Endpoint; shared stream lifecycle tests remain
portable, including the common receipt-consumption representation.

Snapshot construction/response and published-Link client round trips execute on
Linux with their text command consumer. Shared document-read and raw published-Link
server framing tests remain portable; their failure checks are unchanged.

AAI3 server admission, lifecycle, cleanup and request round trips execute on
Linux with the Endpoint. Portable client cancellation/close tests retain their
independent socket peer and explicit terminal-frame fixture; they do not require
the selected Linux server to produce a response. The public AAI3 client/server
conformance oracle writes every byte value exactly once through unequal input
fragments, closes only the input direction, and then reads the reversed bytes
through different response fragments. Existing client tests retain cancellation,
concurrent close, and Write-before-CloseInput ordering coverage; request tests
retain refused Name and explicitly send a complete AAI2 request plus data frame
to prove refusal before the Application owner is called. This mapping does not
retain the removed AAI2 test suite or claim generic Endpoint workloads.

The headless command decoder rejects both a previously valid
`ardents-headless-runtime-v1` plan and mixed v1/v2 fields before creating any
named root or socket or emitting runtime output. A separate dispatch oracle
shows valid and broken v2 plans reach the selected v2 runtime or decoder refusal
without returning the retired-v1 outcome. The command dispatch and plan-decoder
tests exercise the selected runtime and the retired-v1 refusal.

The exact legacy `endpoint open` syntax returns its stable retirement error at
the command adapter. Its regression supplies missing and existing file paths
and an available Unix socket, then proves no output creation or mutation and no
IPC acceptance. The removed accepting client fixtures no longer qualify AAI2;
the AAI2 codec/server/client and exclusive Endpoint adapter are absent, while
the independent Administration client round trip remains covered. Shared
directional-close evidence remains with AAI3 and native Service Connection.

The `ardents-node node --config` retirement oracle submits each old
`rendezvous`, `initiator`, `introduction`, `responder`, and `transit_issuer`
reservation plus a mixed old/closed plan through the real command adapter. Each
must return the typed old-duty retirement outcome while deliberately absent
key, certificate, Source-root, State-root, and role-root paths remain absent.
The complete command package retains independent positive and refusal coverage
for `closed_issuer`, `closed_forwarding`, `closed_resolution`,
`closed_introduction`, and `closed_data_join`. The old Initiator, Responder,
Introduction, Transit-issuance, and Rendezvous engines and their direct server
tests are absent. State and Node tests exercise the State-to-Node value
interface, its projection, and the receipt-time bound. Historical file
inventories do not guard these
boundaries: command behavior proves the old starts refuse before effects, and
the current closed-duty tests prove their accepting paths.

The `ardents-node source --config` retirement oracle submits old-only and
mixed old/closed `native_rendezvous_profile` plans through the real command
adapter. Both return `old Source profile is retired` with no output while the
deliberately absent State and local-role roots remain absent; deliberately
missing TLS inputs also prove the refusal precedes key access and listener
creation. The closed Source process test independently starts the explicit
`ardents-route-v3` profile with its previously pinned signer and verifies the
same signed State bytes through the real Source transport.

The retired `ardents-node contributor` start-retirement oracle and the
Contributor no-start recovery oracle were removed together with the whole
retirement mechanism and its package (ADR-0114). The standard command-usage
test keeps the unrecognized `contributor` shape failing before any effect.

The `ardents name encode/resolve/control` retirement oracle submits all three
recognized command shapes, including absent inputs and incomplete remaining
arguments, through the real command dispatch. The complete inputs are shaped
like the former adapter inputs against one authenticated State root and one
synthetic root shaped like the retired Namespace store. Every retired command
case returns the exact refusal with no output; a recording default transport
observes zero attempts, and all files in those same durable roots remain
byte-for-byte unchanged. The retired `name encode` verb refuses like the
others; its former exact byte vector survives only as inlined fixture bytes
that never execute an encoder (ADR-0113). Removing the command-only
HTTP/OHTTP adapters made the exact private Resolution and adjacent Namespace
verification closure production-dead; ADR-0105 and ADR-0113 then deleted that
closure with its packages, and their deadcode allowance groups were retired
with them.

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
The live Unix Interface is explicitly enabled by `ARDENTS_DEBUG_SOCKET` only
for Endpoint/Node commands; it supplies no worker-confinement or Network
readiness qualification. Product runtime dependencies are unchanged.

The deterministic inventory has an explicit Linux-only supplement in
`tests/profiles/deterministic-linux-packages.txt` for Endpoint introduction,
source, Service, token and publication owners whose files have Linux build
constraints. Linux unit/race gates include it; Windows checks retain the common
inventory. Architecture checks validate both inventories and Linux selection;
no build-error-based filtering or passing platform skip supplies that membership.

## Independent offline Admission profile

The separate `issuer-key-material-linux` profile runs `make issuance-check`:
real RSA keys, owner-only files, exclusive lifetime leases, fault/crash/reopen,
compiled initialize/inspect/export commands, Admission SPKI compatibility and
actual OTLP. It uses the same pinned Go 1.27.1 bookworm image and shares the
immutable command build. It requires proc/sys/loopback for the existing command
tests. Windows cannot replace Linux acceptance; filesystem tests do not qualify
power loss, full storage rollback or network security.

`make admission-check` requires Linux and executes the standard-library
successor Admission file, lease, crash/reopen and concurrency tests under race,
plus the real compiled `ardents-next` cycle and actual OTLP checks. The checked
profile is `admission-ledger-linux`; the Docker acceptance image is
`golang:1.27.1-bookworm@sha256:69a7b9788769bec032d238959b61854e9ae87f57be9029ec04e9885fabf99195`.
Command tests share one immutable compiled executable while keeping separate
mutable roots. Linux prerequisites cannot be replaced by a Windows passing
result. These tests provide no live issuer/State, power-loss storage or whole
Network qualification.

The token-issuance-linux profile executes make token-issuance-check with all
three successor owners and the compiled CLI under race in the same pinned image.
It includes real client blind/finalize and stdlib PSS verification, opaque debit
confirmation, quota retention, journal pairs/floors, faults, crash/replay and OTLP.
CIRCL uses the populated reviewed module cache. No power-loss/rollback claim.
