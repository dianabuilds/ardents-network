# Testing

Ardents tests behavior through three explicit surfaces. None of them is a
delivery stage, and no surface is a prerequisite receipt for another.

## Unit

Unit and single-Module integration tests live beside their implementation as
`*_test.go`. They use public Module Interfaces, replace real time or external
I/O at the owning seam, and must not start Docker. `make unit` is the normal
feedback loop and should remain fast enough to run after each meaningful edit.
Exact-Target Service Connection transfer, backpressure, cancellation, and
same-connection recovery are tested here through `internal/endpoint`; those
behaviors do not need a second private verifier around the same Module.

## End to end

Cross-process tests live under `tests/e2e/<behavior>/`. They build real product
commands, start separate operating-system processes, and assert only command
output and other public behavior. Each test creates fresh fixtures in its own
temporary directory and cleans them through `testing.T` ownership. Run all
process tests with `make e2e`.

The retained process suites cover authenticated Network Source refresh, Node
lifecycle and pressure, plus Service command readiness, bounded failure,
cleanup, and same-connection recovery. The retired pre-native Route process
suite has no replacement yet; a native Route process suite is registered only
after its peer-facing runtime exists.

## Live

The generic live-network profile remains inactive. A native peer-facing Route and the measured
R-092 Node operating profile must exist before a new bounded live suite and
its explicit entrypoint are registered. Docker or a development VPS can test a
selected implementation path, but cannot select the missing operating profile.
There is no generic `tests/live/` directory or implicit build tag; ADR-0031
requires a selected scenario to own its purpose-named source boundary and
complete lifecycle. The active purpose-named A11 soak profile below is a
project-operated functional-alpha qualification; it does not activate or
substitute for that missing native live-network profile.

## Commands

- `make unit`: deterministic Module tests; no Docker or long campaigns.
- `make e2e`: real product commands in local processes.
- `make fuzz`: bounded fuzzing of a maintained untrusted parser/encoder pair.
- `make qualification-h4-8-a11 ...`: the exact six-cell A11 campaign; all
  immutable release, topology, image, port, and external-evidence inputs are
  mandatory.
- `make quick-check`: architecture, vet, unit, build, and module tidiness.
- `make check`: unit, e2e, race, build, formatting, Staticcheck, and
  vulnerability checks; fast independent checks run concurrently, then the
  wall-clock e2e suite and race suite run separately. The checked race recipe
  fixes the Unix process umask at `077`, so test-owned roots passed to the
  product's owner-only storage boundaries have the same permissions as the
  selected participant profile.

`make check` does not run the wall-clock cross-process e2e suite concurrently
with the CPU-intensive race suite. That scheduling would make finite Route
setup deadlines depend on unrelated host contention and could create a false
failure without exercising a different product behavior. Both surfaces remain
independently runnable and mandatory. Live tests require an explicit
Docker-capable job; lack of Docker is a failed live environment, not a skipped
passing test.

## Execution profile registry

The profile registry distinguishes **active** from **inactive** profiles.
Active profiles have a checked Make entrypoint; an inactive profile has no
current accepted suite and names the decision that must activate it. Inactive
does not mean passed, waived, or unavailable evidence.

- developer, deterministic Module, process, race, and fuzz are active;
- affected-platform is active for H4-4A's purpose-named Windows Firefox
  compatibility qualification, `make qualification-h4-4a-firefox`, owned by
  `tests/qualification/h4-4a-firefox/`. It requires an interactive Windows
  desktop and an explicit installed Firefox executable; absence is an invalid
  selected environment, not a skip;
- the signed-release-input qualifier, `make qualification-h4-4-signed-xpi
  ARDENTS_H4_4_SIGNED_XPI='C:/absolute/path/to/signed.xpi'`, owns the exact
  Mozilla-signed H4-4 alpha XPI check in
  `tests/qualification/h4-4-signed-xpi/`. It requires the exact downloaded
  signed XPI; missing or changed bytes are an invalid selected environment,
  not a skip;
- the signed-Firefox qualifier, `make qualification-h4-4-signed-firefox
  ARDENTS_REFERENCE_C2_FIREFOX='C:/Program Files/Mozilla Firefox/firefox.exe'
  ARDENTS_H4_4_SIGNED_XPI='C:/absolute/path/to/signed.xpi'`, owns the
  clean-profile Firefox Release installation proof in
  `tests/qualification/h4-4-signed-firefox/`. The operator explicitly installs
  the exact signed XPI only into its disposable profile, then opens
  `http://reference.ard/`; lack of that explicit operation or any failure is
  an invalid selected environment, not a skip;
- the Windows enrollment-v4 qualifier, `make
  qualification-h4-4-windows-enrollment
  ARDENTS_H4_4_SIGNED_XPI='C:/absolute/path/to/signed.xpi'`, owns one
  temporary current-user native-manifest install/remove run using that exact
  XPI. It refuses an existing registration and requires a Windows host; an
  unavailable or changed artifact is an invalid selected environment, not a
  skip;
- the Ubuntu container enrollment-v4 qualifier, `make
  qualification-h4-4-ubuntu-enrollment
  ARDENTS_H4_4_SIGNED_XPI='C:/absolute/path/to/signed.xpi'`, runs the exact
  signed XPI and current Linux command bytes as UID 1000 in the pre-existing
  `ubuntu:24.04` Docker image. It proves only native-manifest mechanics, not a
  desktop Firefox or participant release; unavailable Docker or artifact input
  is an invalid selected environment, not a skip;
- soak is active only through `make qualification-h4-8-a11`, whose frozen
  purpose-named contract and checked runner are owned by
  `tests/qualification/h4-8-a11/`; and
- Qualification is active for H4-1A Ubuntu Portable and H4-1B Ubuntu
  replacement. Their purpose-named entrypoints are `make qualification-h4-1a`
  and `make qualification-h4-1b`, owned respectively by
  `tests/qualification/h4-1a-ubuntu-portable/` and
  `tests/qualification/h4-1b-ubuntu-replacement/`. Each qualifier uses a
  build-tagged command test because it must build and exercise the same exact
  `ardents` artifact as the ordinary command process suite; ordinary `make e2e`
  never selects it. A host without the declared Linux user-session prerequisites
  is an invalid environment and fails the selected target.
- the H4-2 multi-host native Rendezvous qualifier, `make
  qualification-h4-2-multihost`, is owned by
  `tests/qualification/h4-2-multihost/`. It requires a declared VPS literal IP,
  matching SSH key, free public high port plus two remote-loopback State Source
  ports, Docker 29-compatible host networking, and the pre-existing
  `golang:1.26.6` image. It cross-builds the current command bytes and runs the
  exact two-host native TCP/TLS leg and abrupt-remote-Node-loss oracles through
  a temporary remote container; absence or failure of any prerequisite is an
  invalid selected environment, never a skipped pass. The loss oracle proves
  terminal closure only, not VPS-loss recovery or availability.
- H4-5 dedicated Rendezvous preparation is owned by
  `tests/qualification/h4-5-rendezvous/`. Its purpose-named preflight is
  `make prepare-h4-5-rendezvous` on the Product Owner-declared existing Ubuntu
  host. It captures the actual CPU, memory, disk, link, systemd, cgroup,
  service, and port envelope without treating host size as an eligibility
  gate; it rejects an existing Contributor installation or occupied selected
  port. The complete controller entrypoint is
  `make qualification-h4-5-rendezvous`. It requires both declared existing VPS
  hosts and local Docker, starts its installed primary, local, and second-VPS
  Docker shards concurrently, stops execution at minute 50, and reserves the
  final ten minutes solely for evidence and exact cleanup. Missing cells,
  hosts, Docker, evidence, changed reboot identity, or cleanup are failures
  rather than skips. The installed shard performs exactly one bounded reboot
  between its mixed soak and final C-2 smoke. A controller-owned sixteenth
  cell verifies exact campaign residue absent after every shard is stopped;
  `.ppk` access uses installed PuTTY clients in non-interactive batch mode.
- the H4-2 local full-system emulator, `make
  qualification-h4-2-local-emulator`, is owned by
  `tests/qualification/h4-2-local-emulator/`. It requires Windows Docker and
  the pre-existing `golang:1.26.6` image. The runner cross-builds exact current
  test, product, and fixture bytes outside the repository, mounts only those
  bytes read-only in one resource-bounded Linux container with no external
  network, 1 vCPU, 1 GiB memory, and 256 PIDs. It executes the complete
  held-route C-2 test: every fixture and product Node is a separate process;
  the product Rendezvous is hard-stopped only after both endpoint sides report
  setup readiness. The same campaign carries the in-process product C-2 over
  TCP/TLS and QUIC, checks signed v1/v2 Carrier projection and unknown-profile
  rejection, verifies pending QUIC admission before authentication, exact
  TLS/LegBinding on both adapters, and proves no fallback in either direction.
  The resulting outcomes establish the selected functional full-system
  emulation, not a physical host outage, public-path failure, throughput,
  capacity, or availability claim.
- the H4-3B Docker qualifier, `make qualification-h4-3b-docker`, is owned by
  `tests/qualification/h4-3b-docker/`. It cross-builds the exact Endpoint,
  alpha-control, Node, Reference C-2 fixture, and service-test bytes, then runs
  the four application-transparent HTTP/1.1 terminal cases plus the declared
  request/response head/body-limit and timeout cells independently in a
  read-only, network-isolated Linux container at 1 vCPU, 1 GiB, and 128 PIDs.
  The four C-2 cells prove the selected Target's POST/cookie/redirect/chunked
  behavior, explicit withdrawal, Publisher Application reset, or Publisher
  Endpoint loss without a fallback destination. The companion local-origin
  resource cells prove only the declared HTTP head/body/time ceilings; they do
  not independently authenticate a Target or recreate the C-2 journey. The
  Linux fixture also accepts its separately manifested alpha-control corpus
  before resolution. This is exact current-byte Docker evidence, not a second
  host, participant artifact, selected desktop browser, capacity, availability,
  or public-release claim.
- the H4-3B VPS Docker qualifier, `make qualification-h4-3b-vps`, is owned by
  `tests/qualification/h4-3b-vps/`. It requires a literal VPS IPv4 address and
  matching SSH key through `ARDENTS_H4_3B_VPS` and `ARDENTS_H4_3B_SSH_KEY`.
  The runner uploads only cross-built command/test bytes to an exact temporary
  VPS directory, records their SHA-256 values and the remote Docker/kernel
  envelope, runs the same H4-3B HTTP limit, H4-3B terminal, and H4-6A Docker
  cells without published ports,
  then removes that exact directory. It is a project-operated second-host Docker
  result, not a distributed Publisher/User C-2, participant browser, capacity,
  or availability claim.
- the H4-6A two-fresh-Endpoint qualifier, `make
  qualification-h4-6a-two-endpoints`, is owned by
  `tests/qualification/h4-6a-two-endpoints/`. Its Windows orchestrator requires
  an exact immutable Linux archive, archive digest, Enrollment Pin, Endpoint
  and control-companion digests, cohort, release, one UTC RFC3339 decision time,
  literal VPS IPv4 address, matching root-capable SSH account, and a previously
  absent external evidence directory. It verifies every archive byte locally
  before upload, repeats the verification on Ubuntu 22.04 `x86_64`, and runs
  two Endpoint processes as UID/GID 65534 with distinct fresh XDG roots and
  byte-identical bundle copies. Both must reach ready, commit separate Release
  floors, report the same valid Release outcome, and stop cleanly. The exact
  manifested control companion then uses two distinct fresh inspection roots;
  its catalog, component-root, component-envelope, artifact, and Release
  identities must match the exact manifested bytes and selected release. Both
  complete accepted reports must equal the matching Endpoint outcome and each
  other byte for byte. Failed attempts remain in the denominator and an
  exact remote-root cleanup failure fails the profile. This is project-operated
  equality and lifecycle evidence, not executable self-binding, independent
  control/custody, Windows Endpoint support, capacity, availability, or Public
  Beta readiness.
- the H4-8 A11 Publisher-to-User campaign, `make
  qualification-h4-8-a11`, is owned by
  `tests/qualification/h4-8-a11/`. Its Windows 11 orchestrator refuses a dirty
  or mismatched source/tag pair, independently verifies the exact immutable
  Linux archive, Enrollment Pin, Endpoint and control-companion bytes, and
  requires one exact UTC candidate reference instant and a previously absent
  evidence directory outside Git. Nine fresh
  remote sub-attempts use caller-fixed non-overlapping ports and distinct
  predictable `/tmp`/container names on Ubuntu 22.04 `x86_64`; each actual
  container is checked at host networking, 1 vCPU, 1 GiB, 128 PIDs, no restart
  policy, cgroup v2, and the exact pre-existing Docker image ID. The normal
  soak, four fault primaries, four immediately following fresh-topology
  canaries, and one deterministic local expiry companion are ten exact,
  no-retry Go invocations combined into the frozen 6/6 denominator. A checked
  125-minute campaign clock bounds every process wait by both its cell limit
  and the remaining profile time; an elapsed deadline forces exact cleanup and
  cannot produce an accepted receipt. The checked Make target invokes the thin
  `invoke-windows.ps1` capture entrypoint, which retains the child runner's
  complete separate stdout, stderr, exit status, and its own digest before
  refreshing the evidence inventory; a zero process status without an accepted
  campaign receipt is still a failed entrypoint. Every
  remote capture must inventory the product `rendezvous-node`, transparent
  `carrier-relay`, their topology/readiness receipts, and their complete role
  streams; the Carrier and product-Node primaries additionally retain their
  exact fault plus relay-reset or Node-kill receipts, while Endpoint loss
  retains its crash-ready, fault-injection, and exact Endpoint-kill receipts.
  Windows
  process trees and remote Docker/cgroup state are sampled once per second;
  a gap above two seconds, missing series, OOM/limit event, restart, ceiling
  breach, incomplete retained role output, or residue fails its original
  attempt. The expiry cell explicitly records remote observation and cleanup
  as not applicable because it owns no remote topology. The runner extracts
  the already-validated exact candidate under that retained attempt, supplies
  its strict manifest/release/platform/environment/network/target/architecture
  identities and caller-fixed reference instant, and binds the reported
  catalog and three component identities back to those exact files. Its
  runner-owned `status.json` requires the candidate report, the three
  deterministic owner markers, and final v1 result. The candidate report must
  accept the supplied reference instant and the instant one second before the
  authenticated Release no-new-work boundary. At that exact boundary and at
  terminal minus one second, the catalog and Network remain current while
  Release is `update-required` and authorizes no work. At the shared catalog,
  component, Network, Release-terminal, and TUF expiry instant, both persisted
  and fresh inspection are refused and direct Release inspection is revoked;
  one second later direct TUF evaluation is expired. The deterministic owner
  companion continues to use the fixed `2030-01-02T03:04:05Z` before /
  `03:04:06Z` at-boundary instants.
  A successful 6/6
  receipt establishes only the frozen project-operated low-resource
  functional envelope in the A11 contract, not capacity, availability,
  recovery, hostile-network, independent-operator, Windows Endpoint, browser
  isolation, or Public Beta readiness.

  The checked Make entrypoint accepts no discovered identity defaults. Invoke
  it from the selected clean checkout with every input fixed explicitly (the
  evidence path must not exist):

  ```powershell
  make qualification-h4-8-a11 `
    H4_8_A11_SOURCE_REVISION='<40-lower-hex>' `
    H4_8_A11_CANDIDATE_REPOSITORY='C:/absolute/clean-tagged-candidate-worktree' `
    H4_8_A11_RELEASE_TAG='<exact-tag>' `
    H4_8_A11_ARCHIVE='C:/absolute/outside-git/candidate.tar.gz' `
    H4_8_A11_ARCHIVE_SHA256='<64-lower-hex>' `
    H4_8_A11_MANIFEST_PIN='<64-lower-hex>' `
    H4_8_A11_ENDPOINT_SHA256='<64-lower-hex>' `
    H4_8_A11_CONTROL_SHA256='<64-lower-hex>' `
    H4_8_A11_COHORT='<exact-cohort>' `
    H4_8_A11_AT='<exact-UTC-RFC3339-second>' `
    H4_8_A11_VPS='<literal-ipv4>' `
    H4_8_A11_SSH_KEY='C:/absolute/private-key' `
    H4_8_A11_VPS_USER='<exact-user>' `
    H4_8_A11_BASE_PORT='<first-high-port>' `
    H4_8_A11_IMAGE_ID='sha256:<64-lower-hex>' `
    H4_8_A11_EVIDENCE='C:/absolute/previously-absent-attempt'
  ```
- the H4-3B multi-host qualifier, `make qualification-h4-3b-multihost`, is
  owned by `tests/qualification/h4-3b-multihost/`. It requires a Windows
  qualification host, local and VPS Docker with the pre-existing
  `golang:1.26.6` image, literal VPS IP and matching SSH key, five free public
  high ports plus three remote-loopback ports, and VPS host-network Docker. It stages synthetic, short-lived C-2
  credentials and current Linux fixture/Node bytes into one exact temporary
  VPS directory, then places the Publisher, its loopback Application, State
  Sources, transit roles, resolution Gateway, and alpha Relay/Gateway there.
  The User runs locally with its own retained alpha floor. Four independent
  cells prove normal dynamic HTTP/1.1, withdrawal, Publisher Application reset,
  and Publisher Endpoint loss. Each requires a clean worktree, logs source
  revision, scenario, port range, complete bundle plus binary/config/runner
  digests, local and VPS envelopes, asserts every normally completed remote
  role's classified result, records expected crash controls/errors, and removes
  the exact generated Docker container and remote directory. The Windows C-2 floor setup does not
  replace A5's Linux alpha-control command evidence. This proves only the
  bounded Publisher-to-User two-host tracer; it does not prove independent
  operators, availability, capacity, hostile-network resilience, artifact
  provenance, or a selected participant browser.
- `make prepare-h4-2-net-01a` is not a qualification result or an execution
  profile. It owns only the exact-host preflight and external evidence-directory
  initialization for R-092's future decision-bearing campaign. It rejects a
  non-Ubuntu-LTS/`x86-64` host, anything other than two visible CPUs, a clearly
  non-2-GiB memory class, absent cgroup v2, or missing separately captured
  symmetric-link evidence. It deliberately does not substitute container limits
  for the physical host, run a capacity cell, or select a native profile.

## Profile ownership and validity

Every retained test belongs to one primary profile: deterministic Module,
Adapter/process, affected platform, live, or Qualification. A higher profile proves a distinct
process, platform, network, evidence-independence, or claim fact unavailable at
the lower profile. A test's profile registration records its requirement,
owning seam, observable oracle, fault/adversary or transition, platform/format,
environment prerequisites, and deletion or migration condition.

The current Make targets are the profile entrypoints, not a claim that every
current package has its final product role. Their checked manifests list
maintained and process packages positively; the
checked registry also assigns every current Go-bearing e2e/live suite root to
one execution profile. A new package or suite root cannot enter a profile
through a negative filter or merely by directory naming.

[`tests/profiles/profiles.json`](../../tests/profiles/profiles.json) is the
checked current registry for all active and inactive execution profiles. It
records entrypoint, activation condition, and environment classification, while
scenario ownership remains with the test that proves the behavior. The
architecture gate verifies its schema, required profile set, state, and active
Make-target wiring.

The deterministic and process package memberships are positive and explicit in
the adjacent `*-packages.txt` manifests. A package changes profile only with
its source, test, and product/evidence disposition. A claim qualifier may
select an existing command suite through an explicit build tag when that is the
only way to exercise the exact distributable command; its purpose-named
`tests/qualification/<claim>/` owner and Make entrypoint remain outside ordinary
process execution.

An environment-dependent selected profile has four outcomes: product assertion
failure, test/harness defect, invalid environment, or nondeterministic/unowned
result. None is green until resolved. Missing Docker, an external binary,
required privilege, a platform capability, a pinned image, toolchain, or host
orchestrator is an invalid environment for the selected profile, never a
successful skipped test. Helper subprocesses and unselected matrix cells may
skip only under an orchestrator that proves that every required cell ran.

Rerunning is diagnostic evidence; it never erases the first failure. There is
no automatic retry, permanent quarantine, skip allowlist, or flake budget. A
flaky test may leave a developer profile only with a named owner, captured
reproducer/seed, risk statement, repair deadline, and continued blocking
presence in full/release profiles. A timeout reports the last observed state,
owned resources, seed, and host envelope.

## Test design and retirement

Stateful Modules own injectable wall-clock, monotonic duration, entropy, and
private fault seams at the smallest real owner. Fixed sleeps cannot be readiness
or eventual-consistency oracles: wait for an observable state/event with a
derived deadline. Every goroutine, process, listener, file, lock, timer, and
fixture created by a test has one `testing.T` or scenario owner, cancellation
path, join, residue assertion, and cleanup failure path. `t.Parallel` is
allowed only where fixtures, ports, environment, runtime settings, process
names, and resource budgets are isolated.

Two tests are duplicate-removal candidates only when this identity tuple is
the same: requirement, owning seam, observable oracle, input/transition class,
fault/adversary, platform/format, and independence role. Similar names, source,
or fixtures are insufficient. A migration replaces old shallow-seam coverage
with the target Module Interface suite, retains a higher-level test only for a
distinct fact, and deletes obsolete tests, fixtures, exports, and runners in
the same wave. Compatibility characterization must name its old observer,
format/caller, and deletion condition.

Security vectors are deterministic. Shuffle and randomized schedules publish
their seed. Fuzz/property coverage belongs to every retained untrusted decoder
and canonical encoder pair at Module scope; a minimized retained failure becomes
regression corpus. Race coverage is required wherever a Module owns callbacks,
goroutines, locks, shared admission, cancellation, cutover, pressure, or
mutable durable state, and asserts terminal join and state invariants in
addition to detector cleanliness. Golden compatibility tests cover only accepted
wire, persisted, configuration, command, evidence, and migration formats with
explicit unknown-version and rollback behavior; they do not freeze incidental
struct layout.
