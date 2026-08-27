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

The live profile is inactive. A native peer-facing Route and the measured
R-092 Node operating profile must exist before a new bounded live suite and
its explicit entrypoint are registered. Docker or a development VPS can test a
selected implementation path, but cannot select the missing operating profile.
There is no generic `tests/live/` directory or implicit build tag; ADR-0031
requires a selected scenario to own its purpose-named source boundary and
complete lifecycle.

## Commands

- `make unit`: deterministic Module tests; no Docker or long campaigns.
- `make e2e`: real product commands in local processes.
- `make fuzz`: bounded fuzzing of a maintained untrusted parser/encoder pair.
- `make quick-check`: architecture, vet, unit, build, and module tidiness.
- `make check`: unit, e2e, race, build, formatting, Staticcheck, and
  vulnerability checks; fast independent checks run concurrently, then the
  wall-clock e2e suite and race suite run separately.

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
- soak remains inactive until a bounded duration/load/observer contract is
  accepted; and
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
