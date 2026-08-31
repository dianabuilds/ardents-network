---
id: R-092
title: Which measured Linux operating profile can admit one native Route Node role?
status: decided; dedicated Rendezvous Functional Alpha selected
owner: Product Owner and Codex
started: 2026-08-23
reviewed: 2026-08-29
---

# R-092 — Native Node operating profile

## Decision this unlocks

Select the first concrete, fail-closed native Node admission and listener
profile for M8/M11, or retain preannouncement status if no measured candidate
meets the contract.

## Current contract

R-076/ADR-0024 select mutually authenticated TCP/TLS 1.3 legs and R-078/ADR-
0026 selects their closed LegBinding record. R-081 expressly leaves native Node
capacity, host profile, pressure, drain, and listener integration unselected:
the retained H3 probe capacity cannot be renamed or inherited. NET-01A
originally proposed Ubuntu LTS `x86-64`, 2 vCPU, 2 GiB RAM, and symmetric
100 Mbit/s as a reference envelope. The Product Owner superseded host-size
eligibility for H4-5 on 2026-08-29: existing project Ubuntu hosts are accepted
and their actual envelope is measured, while the exact process cgroup/runtime
placement remains invariant. The qualification result below selects that
placement only for the dedicated-host Functional Alpha.

## Hypotheses

- **H1:** a measured native role-carriage workload can select one finite Node
  profile with an explicit reservation, pressure, drain, and withdrawal rule.
- **H2:** a native carrier is functional but no candidate capacity is safe on
  the reference host; preannouncement remains the correct result.
- **H0:** the native role-carriage workload cannot preserve the selected TLS,
  LegBinding, resource, or cleanup contract.

## Evaluation criteria

Before a profile can be selected, the same declared host must show all of:

- State-pinned TLS 1.3 and reciprocal v1 LegBinding on every admitted leg;
- an explicit refusal before allocating a new listener/connection when the
  measured reservation budget is exhausted;
- pressure transition, finite drain, withdrawal, cancellation, and joined
  cleanup with no surviving listener, connection, or goroutine owned by the
  run; and
- raw per-second CPU/RSS/FD/socket observations, exact workload bytes and
  elapsed time, host identity, source/binary digests, and all failed attempts.

The experiment's capacity points are observations, not a promise. No result
from a different operating system, co-resident Endpoint, or H3 probe is a
native Node profile.

## Evidence plan

### Primary sources

- R-076, R-078, R-081, ADR-0024, ADR-0026, R-023, and NET-01A, inspected
  2026-08-24.
- Linux kernel cgroup v2 documentation, inspected 2026-08-24. In particular,
  `memory.high` is a reclaim/throttling boundary rather than a hard stop,
  `memory.max` may invoke the cgroup OOM killer, and the corresponding event and
  pressure files are evidence inputs rather than application admission limits:
  <https://www.kernel.org/doc/html/latest/admin-guide/cgroup-v2.html>.
- Go `crypto/tls` documentation, inspected 2026-08-24. A server must explicitly
  use `HandshakeContext` or connection deadlines to bound a stalled handshake;
  wrapping a listener with TLS does not create that bound by itself:
  <https://pkg.go.dev/crypto/tls#Conn.HandshakeContext>.
- The maintained `internal/node` and `internal/resource` implementations,
  inspected 2026-08-24. They prove the lifecycle and pressure-state shape but
  contain only H3 probe profiles and a probe listener, not a reusable native
  Rendezvous capacity.

### Experiment

[`experiments/r-092-native-node-profile/`](https://github.com/dianabuilds/ardents-network/blob/fbb42034757513ac009114a00b933aefa76d8ddf/experiments/r-092-native-node-profile/)
contains disposable synthetic mTLS plus reciprocal-LegBinding baseline and
role-carriage scenarios. The latter carries a bounded set of synthetic legs,
withdraws its test listener, holds them, drains or cancels them, joins its
workers, and samples the Linux process through final cleanup. It has no product
Node listener, State root, H3 reader, or capacity decision. A reference-host
follow-up must inject and measure the complete selected resource-pressure rule
and retain its raw Linux observations, host identity, and source/binary digest
outside Git.

[`tests/qualification/h4-2-net-01a/`](../../../tests/qualification/h4-2-net-01a/)
owns a deliberately narrow preflight for that follow-up. It creates an external
evidence directory and fail-closes before measurement unless the declared host
is native Ubuntu LTS `x86-64`, has two visible CPUs, falls in the documented
2-GiB raw-memory observation band, exposes cgroup v2, and has a separately
captured link-evidence file. It is preparation only: it does not run a workload,
assert pressure, or select capacity.

[`tests/qualification/h4-5-rendezvous/`](../../../tests/qualification/h4-5-rendezvous/)
owns the superseding H4-5 declared-host contract and preflight. It captures
the actual existing Ubuntu host envelope without rejecting a stronger or
differently sized host, while still requiring systemd, cgroup v2, exclusive
Contributor managed paths, and a free selected listener port. It is also
preparation only until its frozen two-host matrix has an executable complete
runner and retained result.

### Failure scenarios

- an H3 profile or capacity appears in any native result;
- TLS/ALPN or a reciprocal binding is absent, substituted, or downgraded;
- cancellation leaves a listener, connection, or worker alive; or
- a partial baseline is presented as a selected operating profile.

## Findings

- **Inspection:** the current Node implementation still owns only the H3
  probe runtime; the native Route Module has no Node listener or profile.
- **Inspection (2026-08-24):** the current maintained `ardents-node node`
  command composes authenticated Network State, Node admission/lifecycle,
  resource observation, and one private role-probe listener. It does not
  implement the selected two-leg Rendezvous pairing/pump duty. Running two
  copies in Docker would exercise two independent probe lifecycles, not create
  a Rendezvous network; that result must not be labelled a multi-Node H4-2
  tracer.
- **Measurement:** the disposable harness completed two sequential 4,096-byte
  synthetic loopback legs with TLS 1.3, the exact ALPN, reciprocal v1 bindings,
  and byte-identical echo on the local development host. It is a harness sanity
  result only: that host is not the R-092 Linux reference environment and its
  timing/allocation output is not a capacity measurement.
- **Measurement (2026-08-23, preliminary Linux sanity):** an initial sandboxed
  WSL enumeration returned `E_ACCESSDENIED`; a subsequent local read identified
  `Ubuntu-24.04`, Linux
  `6.6.87.2-microsoft-standard-WSL2`, `x86_64`, 12 logical CPUs, and
  `31,603,916 KiB` advertised memory. This is materially different from the
  NET-01A 2-vCPU/2-GiB reference host.
- **Measurement (2026-08-23, preliminary Linux sanity):** the disposable
  harness, cross-compiled with `go1.26.6 windows/amd64` from commit
  `9769d6065e6a4af834770b3ea389054dbcda978d` to Linux/amd64 (SHA-256
  `9d84818ebb77b306fcb9a4fb2f4297b5485c788af426f1315a4579badf3b0108`),
  completed four sequential 4,096-byte legs under that WSL runtime. It carried
  32,768 bytes in 4,845,675 ns; RSS changed from 5,505,024 to 7,340,032 bytes,
  CPU ticks from 1/0 to 1/0 (user/system), and FDs remained 8. The WSL guest has
  no Go installation, so this is an executed Linux binary rather than a
  source-native guest build. It confirms only the synthetic TLS/ALPN,
  reciprocal-binding, echo, and basic FD-cleanup path; it has no pressure,
  listener, concurrent workload, per-second sampling, socket count, or network
  shape evidence.
- **Measurement (2026-08-23, preliminary Linux role-carriage):** the expanded
  harness was cross-compiled to Linux/amd64 (binary SHA-256
  `6f55a1fdca9e2704fd8863cd29a3b92ddee2bf5dafc6f0ba1501e1dc6435bb68`)
  and executed in the same non-reference WSL guest. Two simultaneous 4,096-byte
  legs completed reciprocal binding and became active; the synthetic listener
  withdrew, one subsequent dial was refused, and a two-second hold then drained
  and joined both legs in 2,002,981,579 ns. Four begin/one-second/end process
  samples saw RSS from 5,636,096 to 7,471,104 bytes, FDs/sockets at 10/3 then
  11/4 while active and 7/0 after cleanup; Go goroutines fell from 2 to 1.
  This verifies only synthetic capacity-triggered withdrawal and cleanup in
  WSL. It is not pre-kernel admission refusal, a real resource-pressure test,
  a NET-01A measurement, or a selected profile.
- **Measurement (2026-08-23, preliminary Linux cancellation):** an initial
  cancellation run showed that the sampler inherited the cancelled workload
  context and could report a pre-cleanup final sample. The harness was changed
  so the sampler stops only after all clients and server workers have joined,
  then rebuilt to Linux/amd64 (binary SHA-256
  `77aaeb5eb884d07a989f7404d863c3dec8e0ea9e0eaed0c00f6d4091de954608`).
  In the same non-reference WSL guest, two active 4,096-byte legs withdrew,
  one subsequent dial was refused, and cancellation joined both clients and
  server workers in 2,003,160,204 ns. Its post-cleanup sample was 7 FDs and
  zero sockets (from 11/3 before work); Go goroutines fell from 2 to 1. This
  confirms only the synthetic cancellation cleanup path in WSL, not an OS
  resource-pressure rule, a NET-01A result, or a selected profile.
- **Measurement (2026-08-23, non-reference VPS sanity):** source commit
  `699b5930b3127e4995393b0838f112953a2fafbb`, cross-compiled with
  `go1.26.6 windows/amd64` to Linux/amd64 binary SHA-256
  `77aaeb5eb884d07a989f7404d863c3dec8e0ea9e0eaed0c00f6d4091de954608`,
  ran on Linux `5.15.0-185-generic`, `amd64`, four logical CPUs, and
  `8,303,755,264` bytes of memory. The external raw transcript records two
  10-second, two-leg, 65,536-byte runs. In both, two legs completed the
  synthetic reciprocal TLS/LegBinding path, listener withdrawal refused one
  later dial, and terminal cleanup left six file descriptors, zero sockets,
  and one goroutine (from two before work). The VPS is double the declared CPU
  class and about four times its memory; it is an independent Linux regression
  sanity check for the refactored synthetic path, not NET-01A reference-host
  evidence, resource pressure, pre-kernel refusal, a production Node listener,
  or a selected capacity/profile.
- **Measurement:** no Linux *reference-host* result has yet been captured.
- **Environment measurement (2026-08-24):** the already available local Docker
  Desktop host and project-operated Ubuntu 22.04 Docker VPS completed R-094's
  real separate-host TCP/TLS and QUIC oracle. The remote host has 4 CPUs and
  about 8 GiB RAM, so it is suitable for isolated functional and link-fault
  tracers after Rendezvous exists, but it is not the selected 2-vCPU/2-GiB
  NET-01A reference host. Container CPU/memory limits can test bounded refusal;
  they do not make the underlying stronger host a reference-host measurement.
- **Environment/measurement (2026-08-26):** the fixed `golang:1.26.6` image
  used by the H4-2 two-host runner contains neither `tc` nor `ip`; the shared
  VPS host has both but also runs unrelated long-lived services. No host qdisc
  was changed. Instead `TestH42MultiHostRendezvousKernelNetemRelay` cross-built
  one static qualification-only relay and ran it in three disposable Docker
  namespaces with only `NET_ADMIN`, 128 MiB, 0.5 CPU, and 64 PIDs. The relay
  received only its own binary; `/usr/sbin/tc` and matching host libraries were
  read-only mounts for the child command. A 200-ms qdisc delay retained exact
  byte carriage, and 100% qdisc loss exposed no authenticated attachment before
  the caller's two-second deadline while `tc -s` reported nonzero drops. A
  third sidecar fixed 20 ms ±5 ms delay, 5% loss, and 10% reorder and carried
  one exact 256 KiB transcript while retaining declared qdisc state and
  nonzero requeues. A successful random-loss transcript does not establish an
  observed loss event; the 100%-loss cell is the measured loss outcome. The
  current full four-case runner passed in 79.505 seconds; its netem case passed
  in 39.22 seconds. The sidecar/host bridge is
  still not public-path loss, MTU, NAT, active probing, host loss, recovery,
  or availability evidence.
- **Inspection (2026-08-24):** the maintained lifecycle already separates Node
  readiness (`ABSENT/PREPARED/READY/DRAINING/WITHDRAWN/FAILED`) from resource
  pressure (`NORMAL/PROTECT/DRAIN`), rechecks assignment freshness, and joins
  accepted work after listener shutdown. That behavior is reusable. Its
  `h3-np1-v1`/`h3-s-v1*` limits, probe capacity, fixed 15-second probe bounds,
  and probe-specific admission are not.
- **Inspection (2026-08-24):** a Go TCP process cannot make the kernel's listen
  backlog disappear. The native contract can bound application work only after
  `Accept`: reserve a finite handshake slot before TLS work or a worker is
  started, close immediately when no slot exists, and record the kernel backlog
  separately. Therefore “pre-kernel admission” is not a valid selection
  criterion for the first implementation.
- **Prototype (2026-08-24):** a throwaway pure reservation model and interactive
  terminal shell were driven through healthy pairing, full handshake/waiting/
  pair budgets, duplicate-side binding, `PROTECT`, graceful `DRAIN`, forced
  drain, and post-withdrawal admission. Every revised sequence retained its
  declared bounds, never produced a one-sided active pair, and reached zero
  reservations after withdrawal. This is state-shape evidence, not a network,
  concurrency, timeout, or capacity measurement.
- **Prototype finding:** the first model incorrectly attached attempt/side facts
  to a pre-TLS socket and retained pre-admission work through `PROTECT`. The
  corrected contract keeps handshake slots identity-free until authenticated
  LegBinding, closes all handshake and unmatched-leg work on `PROTECT`, and
  closes other pre-admission work as soon as the last active-pair slot is taken.
  Otherwise the model can strand an attempt that has no capacity to complete.
  The answer is promoted below; the throwaway model and Make target were deleted
  rather than retained as a second state machine.
- **Rendezvous tracer result (2026-08-24):** a new build-ignored, disposable
  process tracer replaced the earlier echo-only shape with the selected three
  reservations and real pairing/pump path. It uses mutual TLS 1.3, the native
  ALPN, the maintained canonical `route.LegBinding` encoding, certificate-to-
  binding identity checks, one Initiator plus one Responder per Attachment ID,
  fixed connection deadlines, and two 32 KiB-buffered pumps. It creates no
  Authority, Network State, Entry, Service, maintained package, or capacity.
- **Local matrix measurement:** five predeclared process cases passed on the
  Windows development host without retry. The exact pair carried the same
  262,144-byte transcript in both directions with SHA-256
  `743cdf7849dc6ffdc775371adb60313afb6ecb74e2a8ef22f63c8d419e0b36ec`.
  A duplicate side was rejected while the first leg remained until its one
  expiry; one unmatched leg expired and released its reservation; a held sole
  pair caused two new sockets to close before TLS without evicting the pair;
  and an owned drain terminated a held pair and joined all work. Every server
  result ended with zero handshake, waiting-leg, active-pair, and connection
  counts. [Tracer evidence](https://github.com/dianabuilds/ardents-network/blob/fbb42034757513ac009114a00b933aefa76d8ddf/experiments/r-092-rendezvous-tracer/README.md)
- **Concurrency measurement:** the Windows race build failed before execution
  because the current MinGW/cgo link requested unavailable `-ldl`; it is not a
  tracer result. The focused exact-pair cell was then built and executed under
  Go 1.26.6's Linux race runtime in the already-local Go image. It completed the
  exact pair with no race report, joined cleanup, and zero final connections.
- **Separate-host measurement:** the exact Linux binary SHA-256
  `13343c6562cbf1b33dd6eef811dacd980193f186413dc628dced085c3f9e9f19`
  and scratch image ID
  `sha256:710126c98253b554a9fc71a7867af39366561a10603f0a41b23f075fabdbf3ef`
  ran as one remote Ubuntu server and two local clients in separate Docker
  networks over public high TCP port 47926. All containers were read-only,
  unprivileged UID 65532, capability-free, `no-new-privileges`, and limited to
  128 MiB, 0.5 CPU, and 64 PIDs. Both clients reported the exact transcript,
  FD `7→7`, goroutines `3→3`, and joined cleanup. The server observed peak
  handshakes `2`, waiting legs `1`, active pairs `1`, one successful pair, zero
  final reservations/connections, exit zero, and no OOM. All marked resources
  and the listener were removed without changing existing workloads or ports
  80/443.
- **Maintained command measurement (2026-08-26):**
  `TestNativeDutyProcessesUseTheirExactStateAssignments` built the product
  `ardents-node` command and started separate Initiator, Introduction,
  Rendezvous, and Responder processes. Each process used its own State root,
  role root, identity, certificate, listener, clock observation, and State
  materialization from the same signed native-profile Epoch with two configured
  authenticated Source inputs. Every process reached `READY` only with the
  exact Epoch, role, and Role Domain assignment digest. This proves the command
  boundary can materialize all four existing duties; it does not carry a Route,
  prove a Source refresh, graceful process withdrawal, multi-host operation, or
  select a resource/capacity profile.
- **Maintained full-route measurement (2026-08-26):**
  `TestReferenceC2CarriesOneRouteThroughProductNodeCommands` replaces the
  C-2 fixture's Initiator, Introduction, Rendezvous, and Responder processes
  with four separately configured product `ardents-node` commands. One local
  Publisher-to-User C-2 journey completed through those commands using the
  same signed State Epoch and authenticated Source inputs. This establishes the
  present command topology as functional rather than merely ready. It remains
  a local controlled test: the other C-2 roles are fixtures and the accepted
  State is established at startup. Its Linux Docker run sends the same
  `SIGTERM` used by Docker or a service manager after the completed journey and
  requires every product Node to emit `DRAINING` then `WITHDRAWN` before exit.
  The Windows compatibility harness has no equivalent child-service signal and
  retains forced test cleanup.
- **Maintained State-change measurement (2026-08-26):**
  `TestReferenceC2RefreshesStateAndWithdrawsProductNodeCommands` first
  completes that same C-2 journey, then replaces both authenticated Source
  processes with the same authority's signed, linked Epoch 2. Each of the four
  product Node commands observes the successor and emits `DRAINING` followed
  by `WITHDRAWN`. This proves one current-State change triggers terminal
  withdrawal; it is not a full-C-2 held-work or multi-host result.
- **Maintained held-work measurement (2026-08-26):**
  `TestReferenceC2RefreshesStateAndWithdrawsProductNodesWithHeldRoute` holds
  the established Publisher Application-to-Service connection after all four
  product Nodes have carried its C-2 setup. It replaces both authenticated
  State Sources with the same authority's linked successor, requires every
  Node command to emit `DRAINING` then `WITHDRAWN`, and only then releases the
  two held fixtures for their classified terminal outcomes. This is local
  full-C-2 withdrawal-with-active-work evidence; it is not a hostile-failure,
  multi-host, resource-pressure, or capacity result.
- **Maintained negative and active-work measurements (2026-08-26):**
  `TestReferenceC2ProductNodeCommandsReportUnavailableAfterPublisherGoesOffline`
  retains an authentic descriptor but removes the Publisher-side availability;
  the User returns `service unavailable`, receives no Reference URL, and makes
  no browser request while the four transit duties are product commands.
  `TestRendezvousNodeProcessPairsOnlyStateAuthorizedLegs` rejects an
  unauthorized native `LegBinding`; the Linux-only
  `TestRendezvousNodeProcessDrainsActivePairOnSIGTERM` carries an authenticated
  active pair, sends `SIGTERM`, observes `DRAINING` then `WITHDRAWN`, and sees
  the pair close. `TestRendezvousNodeProcessBoundsIncompleteTLSHandshakes`
  sends three deliberately incomplete TLS records to the product command;
  exactly the configured two handshake reservations remain held, the third is
  refused, and a normal authenticated pair works after release.
  `TestRendezvousNodeProcessExpiresIncompleteTLSAdmission` then retains no
  client close: an explicit `admission_timeout_ms` closes the incomplete TLS
  reservation and a later authenticated pair works. The required timeout is
  capped by State expiry and is carried by every native duty's local plan; it
  has no default and does not alter the State expiry for established work.
  `TestRendezvousNodeProcessKeepsActivePairPastAdmissionDeadline` holds an
  authenticated pair beyond the short admission timeout, then carries bytes;
  this guards the required switch to the authenticated
  `LegBinding.NotAfter` deadline after admission.
  `TestIntroductionSlotExpiresAtItsRegistrationDeadline` uses a shorter
  authenticated transit/registration expiry than the duty State expiry and
  requires the registered Publisher connection to close at that deadline.
  This is bounded reservation/recovery evidence, not DoS resilience, a
  selected release value, inter-host hostile-network evidence, or a capacity
  result. A fresh one-host Linux Docker execution of the exact product C-2 test
  exited 0 in 18.11 seconds with a read-only source mount.
- **Project-VPS product-command measurement (2026-08-26):** the current
  checkout of `TestReferenceC2CarriesOneRouteThroughProductNodeCommands` ran in
  one temporary Docker 29.4.1 container on the project Ubuntu VPS. The source
  mount was read-only, no port was published, the command exited 0, and the
  test passed in 22.51 seconds. The container and its exact temporary source
  directory were then removed and their absence checked. This is a
  project-controlled one-host functional result; it is not an inter-host,
  resource-pressure, capacity, or independent-operation result.
- **Maintained inter-host product-Rendezvous measurement (2026-08-26):** the
  purpose-named `make qualification-h4-2-multihost` target cross-built the
  current `ardents` and `ardents-node` commands on the Windows development
  host, transferred only those bytes plus ephemeral signed State/material and
  test credentials, and started one temporary host-network Docker container on
  the project VPS. Its public State-authorized TCP 47926 listener was a real
  product Rendezvous; two mutually authenticated product State Sources stayed
  on the VPS loopback. Direct native Initiator and Responder TLS legs from the
  Windows host carried the fixed transcript and a wrong-role LegBinding whose
  Initiator certificate still matched its declared Node ID was rejected. The
  verbose run fixed profile `ardents-interactive-route-v1`, State epoch 1,
  State digest `d22bfef964b59bd327e7d5c53b6a80840ffa2397ba76e0e1b2244b2e31cf3971`,
  `ardents` SHA-256
  `8b55bb78de84e16f3a4043e36b22f309b1a4e019d7e9d5d217698db1ca8fa9f7`, and
  `ardents-node` SHA-256
  `c9f7f714e1897fe0be3c5a97a7d85e2b8a028bbd091864e4d410829ef62ebd9e`.
  It passed in 11.58 seconds. A companion loss cell first carried an active
  transcript, abruptly killed that exact remote Node/container, then required
  both local TLS legs to return a terminal read error rather than time out; it
  passed in 15.11 seconds. The remote envelope was Docker 29.4.1,
  `golang:1.26.6` image ID
  `sha256:0d1d3a794be25f809dd2cb3160d8c73276c4056a9f8242a138e908ddeee7b6b6`,
  Linux 5.15.0-185-generic x86_64, 4 vCPU, and 8,109,136 KiB reported memory.
  Its generated container and exact `/tmp/ardents-h4-2-multihost-*` directory
  were removed and their absence separately checked. The direct test legs are
  intentionally not Endpoint/Service duties. This is one controlled two-host
  product-byte/path result plus a remote Node/container terminal-closure
  result. `TestH42MultiHostRendezvousTCPFaultRelay` then placed a transparent,
  test-owned local TCP relay before that real remote Node: with 200 ms delay in
  each direction it retained exact carriage; local RST produced terminal
  closure of the active legs and a new pair succeeded through the unchanged
  Node; a byte blackhole produced only the caller's explicit one-second read
  timeout. It passed in 13.32 seconds. The relay does not terminate or inspect
  TLS and is not a maintained Node, Route, carrier, or recovery mechanism.
  `TestH42MultiHostRendezvousKernelNetemRelay` adds three isolated Docker
  sidecars in front of that same remote Node. The static relay has only
  `NET_ADMIN`, mounts only itself plus read-only `tc`/matching libraries, and
  applies qdisc only to its own `eth0`; it receives no State or credential
  bytes. Its 200-ms-delay cell retained exact carriage and its 100%-loss cell
  returned no attachment before the caller's two-second deadline while the
  qdisc showed nonzero dropped packets. A third sidecar fixed 20 ms ±5 ms
  delay, 5% loss, and 10% reorder and carried one exact 256 KiB transcript
  with declared qdisc state and nonzero requeues. A successful random-loss
  transcript does not establish an observed loss event; the 100%-loss cell is
  the measured loss outcome. The current full four-case runner passed in 79.505
  seconds; its netem case passed in 39.22 seconds. This is container-namespace
  kernel-netem evidence, not a
  public-path loss, MTU, NAT, active-probe, host-loss, recovery, or
  availability result.
  This remains not a full C-2 multi-host workflow, independent operations,
  true VPS/host-loss recovery or availability evidence, resource pressure,
  hostile-network evidence, or NET-01A capacity selection. In particular, it
  does not model public-path packet loss/reordering, MTU, NAT, active probing,
  or network recovery.
- **Local full-system fault-emulator measurement (2026-08-26):** the selected
  `make qualification-h4-2-local-emulator` profile cross-built exact current
  Linux `ardents`, `ardents-control`, `ardents-node`, and C-2 fixture bytes
  outside the repository, then mounted only those bytes read-only into one
  disposable `golang:1.26.6` container with no external network, one CPU, a
  1-GiB memory ceiling, PID limit 256, and temporary owned test state. The
  complete held-route C-2 scenario ran as separate processes. It waited for
  both Publisher Application and User setup acknowledgements, hard-stopped the
  actual product Rendezvous process, and required the existing endpoint
  terminal classifications after release. The command passed in 5.14 seconds.
  This is the selected full-system composition and simulated fault-domain-loss
  evidence. It does not represent a physical host/provider outage, public-path
  failure or recovery, capacity, availability, or independent operation.
- **Configuration-boundary measurement (2026-08-26):** a native
  `ardents-node` plan now rejects every nonempty `node_resource_profile` before
  it opens State or a listener. This prevents the retired `h3-np1-v1` and
  `h3-s-v1*` capacity numbers from being relabelled as native Route limits.
  The rejection is intentionally not a replacement profile; only NET-01A can
  select and name one.
- **Implementation (2026-08-26):** the purpose-named NET-01A preflight is
  executable now. It records the exact source state, toolchain, host envelope,
  cgroup v2 facts, and operator-provided raw symmetric-link transcript outside
  Git. Its fixed host-envelope checks make the known 4-vCPU/8-GiB project VPS
  ineligible. It is not a reference-host observation until run on a separately
  declared eligible host, and it neither runs nor weakens the full selection
  matrix below.
- **Inference:** the selected pairing/reservation/pump shape is executable in
  separate processes and one current product Rendezvous can carry State-bound
  native legs across the available real public path. The remaining R-092
  uncertainty is no longer whether two bounded legs can be joined, whether a
  current-State successor withdraws a command, whether it terminates held
  full-C-2 work, whether the Rendezvous command can start across two hosts,
  whether an abrupt Rendezvous fault domain gives the held full C-2 route a
  classified terminal outcome in the selected local emulator, whether a remote
  Node loss immediately terminates its active legs, or whether incomplete TLS
  admission expires without a client close. The controlled relay establishes
  only the named delay/RST/blackhole outcomes; it is not hostile-network
  qualification. Remaining uncertainty is hostile abuse/error cells, any
  physical-host/provider-outage claim beyond the selected emulator, selection
  of an access-path-valid admission duration, resource-pressure placement, and
   selection of numeric limits on the exact NET-01A host. The tracer and
   controlled product-command cells cannot supply those claims.
- **Implementation (2026-08-29):** H4-5A/B binds the existing Rendezvous duty
  to the `h4-5-rendezvous-alpha-v1` resource placement only. Its
  fixed placement is one CPU, 256 MiB cgroup memory maximum, 192 MiB memory
  high boundary, 128 MiB exact Go memory limit, 64 tasks, and 256 file
  descriptors with separate NORMAL/PROTECT/DRAIN thresholds. The Node and
  command boundaries reject that profile for Initiator, Introduction,
  Responder, probes, or mixed duties. This was implementation evidence before
  the declared-host disposition below selected the narrow profile.
- **Implementation (2026-08-29):** the candidate now has one closed
  dedicated-host lifecycle behind `ardents-node contributor`: an independent
  manifest SHA-256 is checked before parsing a fixed executable/key/plan
  inventory; generation one installs one hardened systemd unit; exact
  generation successors stop and withdraw before switching; failed and
  interrupted installs or updates recover only authenticated owned state; and
  diagnose, restart, drain, withdrawal, and exact-ID removal expose no generic
  service operation. Last lifecycle and resource events are bounded private
  files. These are behavior-tested filesystem/supervisor semantics and a
  cross-compiled Linux command, not a native systemd execution result.
- **Implementation defect finding (2026-08-29):** pre-qualification review
  found three container-masked measurement/lifecycle defects. The Rendezvous
  pressure adapter treated cumulative completed `RelayedBytes` as live queued
  bytes, which could terminally drain a healthy process after one route; the
  Linux sampler read the cgroup namespace root rather than resolving the
  process's systemd cgroup; and its socket count covered the complete network
  namespace rather than the process's own socket FDs. Narrow regressions now
  require live reservations only, fail-closed cgroup-v2 path parsing from
  `/proc/self/cgroup`, and bounded own-FD socket inventory. The accepted
  installed workload and final smoke later demonstrated those corrections
  under the systemd unit.
- **Implementation boundary finding (2026-08-29):** Rendezvous has no
  application queue to measure: handshakes, unmatched legs, and active pairs
  are independent finite reservations, while paired bytes pass through direct
  bounded pumps. The candidate therefore reports queue items/bytes as exactly
  zero instead of inventing nonzero queue headroom. Its two mutable roots are
  now measured as one managed-storage dimension: `PROTECT` at 320 MiB,
  recovery below 256 MiB, and terminal `DRAIN` at 384 MiB or more; more than
  5,000 regular files fails closed. Installed generations, the caller-owned
  bundle, journal retention, and provider snapshots remain separately bounded
  or external residue and are not hidden inside that runtime-state claim.
- **Product Owner selection (2026-08-29):** available existing project Ubuntu
  hosts are eligible for H4-5 regardless of their physical CPU, memory, disk,
  or link characteristics. The campaign records those characteristics and
  enforces the candidate's exact service cgroup/runtime limits instead of
  rejecting a host for being stronger than NET-01A. Temporary project fixture
  co-location is acceptable evidence for this functional-alpha decision only;
  it does not select a co-resident Endpoint product profile or support
  independence, public capacity, or availability language.
- **Declared-host preflight measurement (2026-08-29):** the first H4-5
  preflight passed at commit
  `bdb9a66523c26558a09c063aa06399b49c8fa4cf`. At
  `2026-08-29T07:40:49Z` the selected existing VPS reported Ubuntu 22.04.5 LTS,
  Linux 5.15.0-185 `x86_64`, four online CPUs, 8,109,136 KiB `MemTotal`,
  running systemd 249, and cgroup v2 with `cpu`, `memory`, and `pids`. Port
  49152 was unused and the Contributor unit and managed paths were absent. The
  exact outcome is `eligible-for-h4-5-campaign; no qualification result`.
  Host observations, runner bytes, and their verified SHA-256 inventory are
  retained outside Git under
  `C:\Users\vitek\Ardents-Release\evidence\ardents-h4-5-preflight-bdb9a665`.
  At that point the next action was the complete lifecycle/C-2 campaign; the
  preflight alone did not select capacity. The earlier `1b810813` attempt is
  retained but superseded because its absent-unit oracle checked the shortened
  wrong unit name; the accepted rerun checked the exact
  `ardents-rendezvous-contributor.service` and again found it absent.
- **Installed-product smoke measurement (2026-08-29):** four retained attempts
  on the selected existing VPS produced one complete pass and three classified
  implementation failures. The failures exposed a transferred executable-mode
  loss, missing synchronous first-State acquisition from the two authenticated
  Sources, and a non-idempotent drained-to-withdrawn systemd transition. At
  commit `174283d5`, the corrected generation-1 run installed the exact unit,
  bootstrapped an empty Network State root from those Sources, reached
  `READY`, diagnosed and restarted, carried the maintained Publisher-to-User
  C-2 route, drained, refused a new TCP connection, withdrew, removed the
  confirmed deployment, and left no unit, managed root, runtime root, or
  selected listener. It passed in 43.62 seconds. Evidence is retained outside
  Git under
  `C:\Users\vitek\Ardents-Release\evidence\h4-5-smoke-174283d5-attempt4-20260829`;
  the failed attempts remain under their separately named evidence roots.
  At that point this established an installed-product tracer and operator
  lifecycle, not the later workload/fault disposition.
- **Accepted qualification measurement (2026-08-29):** the original bounded
  controller campaign ran once in less than ten minutes across both existing
  VPS hosts and local isolated Docker. Nine cells passed, four failed with
  classified qualifier/product defects, and three downstream cells were absent
  after the controller path stopped. The complete campaign was not replayed.
  Corrected-cell reruns closed the host-envelope quoting, SSH transport,
  released-capacity, State-sampling, and sampler-stop defects while preserving
  every failed attempt in the denominator. With source content later committed
  as `e3ff7ba7`, the replacement installed workload completed 260/260 cycles paced at 250 ms in
  65.129399 seconds, used one proxy TCP connection with zero redials, completed
  four no-fallback probes, acknowledged all 107,559 accepted bytes, received
  51,786 bytes, and closed cleanly. Its P95 cycle latency was 136.883 ms and
  maximum start lag 246.373 ms. A separate final smoke passed apply, State
  bootstrap, readiness, C-2 carriage, update, restart, `PROTECT` recovery,
  drain, withdrawal, removal, and residue checks. The primary unit, managed
  roots, listener, secondary shard roots, and named containers were absent at
  final inspection. Raw attempts remain under the external
  `h4-5-campaign-87bc4ab3-20260829` and
  `h4-5-repair-9fcf33c3-20260829` evidence roots. The eight-minute gate was
  rejected as disproportionate and was not rerun; the accepted bounded workload
  proves the named operation and lifecycle behavior, not availability duration.
- **Operator-burden and utility measurement (2026-08-29):** the original
  controller produced 13 retained result records over a 416-second observed
  span: nine passes and four classified failures. The factual preparation
  history records one manual command, three earlier failed attempts, four
  clarifications, three repairs, three provisioning actions, and five input
  verifications. Preparation active-human seconds were not continuously
  instrumented and remain `null`; no total human-time value is fabricated. In
  the two accepted installed runs, apply took 1.871-1.930 seconds, diagnose
  0.059-0.071 seconds, ordinary restart 1.237-1.296 seconds, idle update
  1.372-1.444 seconds, drain 0.122-0.141 seconds, withdrawal 0.390-0.400
  seconds, and exact removal 0.298-0.347 seconds; restart after resource
  recovery took 1.257 seconds. The with/without-duty comparison used the same
  declared C-2 topology: with the installed Rendezvous, the 260-cycle workload
  completed; after drain/withdrawal, a new connection was refused and exact
  removal left no route listener or managed residue. This demonstrates useful
  role completion and a closed scripted operator surface, but not a public
  availability benefit or an independent-capacity increment.

## Options

### First-duty candidates

1. **Rendezvous first.** One temporary two-leg joining duty over the selected
   authenticated TCP/TLS Carrier. It exercises reciprocal peer binding,
   reservation, bounded per-connection state, pressure refusal, drain,
   withdrawal, and joined cleanup without first adding Endpoint-adjacent Entry
   Set, publication, or Introduction semantics. It does not by itself produce a
   complete User-to-Service route.
2. **Entry first.** One endpoint-adjacent Initiator or Responder duty. This is
   closer to a visible connection journey but immediately adds Entry Invite,
   long-lived set, ordinary-location exposure, retry, public listener, abuse,
   and blocked-entry obligations before the common Node lifecycle is measured.
3. **Introduction first.** One sealed-invitation control duty. It is bounded and
   carries no Application Data, so it cannot establish the first useful carrier
   profile or measure transit utility by itself.
4. **All five logical carrier positions together.** This can create an early
   end-to-end tracer but makes listener, role, Route, resource, and cleanup
   failures difficult to attribute. A passing combined run would still not
   select an individual Node operating profile.
5. **One combined relay/gateway process.** Fastest byte-forwarding
   demonstration, but rejected as a Node-profile candidate because it collapses
   accepted Role Domain and Route Knowledge Separation boundaries into a
   different product.

**Product decision (2026-08-24):** option 1 is selected. Rendezvous is the first
native Node duty. It has the smallest useful data-plane boundary and supplies
reusable Node lifecycle evidence before Entry and Introduction add their
distinct exposure and state. The duty and exact one-pair profile were
subsequently implemented and accepted by the qualification measurement above.

### Selected Rendezvous operating profile

This is the exact shape selected for the project-qualified dedicated-host
Functional Alpha, not a public capacity or availability profile:

- one unprivileged Ubuntu process owns one Node Identity, Rendezvous assignment,
  state root, cgroup v2 placement, public TCP/TLS listener, and terminal
  lifecycle;
- the listener address is the exact State-authorized literal address and port;
  operator diagnostics remain local and no HTTP administration or metrics
  listener is exposed to the Internet;
- admission has three independent finite reservations: identity-free accepted
  sockets performing bounded TLS/LegBinding, authenticated single legs waiting
  for their reciprocal leg, and active Rendezvous pairs. Attempt and side become
  usable only after authenticated LegBinding. One active-pair reservation owns
  exactly two authenticated legs and both bounded directional pumps;
- every reservation has a deadline no later than the duty's Work Safety Lease.
  A stalled TLS peer, unmatched leg, slow reader, or non-reading writer therefore
  consumes a known finite slot and byte budget, never an unbounded goroutine or
  queue;
- capacity exhaustion is an explicit local refusal. It causes no arbitrary
  eviction, hidden retry, peer-selected alternative, or new source contact; and
- counters and events contain only aggregate local duty facts: pressure mode,
  reservation use, accepted/refused/terminal counts, bytes, queues, RSS, Go
  memory, CPU, FDs, sockets, threads, and drain duration. Peer addresses,
  Targets, Names, bindings, and complete Route histories are excluded.

The operating contract keeps two state machines distinct:

| Condition | New work | Admitted work | Required transition |
|---|---|---|---|
| `NORMAL` pressure while Node is `READY` | Admit only after the relevant reservation succeeds | Continue within deadlines and queue caps | Remain `READY` |
| Last active-pair reservation is taken while pressure remains `NORMAL` | Refuse newly accepted sockets and close every other handshake/unmatched leg with an explicit capacity result | Preserve active pairs | Advertise new-work readiness only after a pair slot is actually free |
| `PROTECT` pressure | Refuse new sockets and close every handshake/unmatched leg | Preserve active pairs only | Return to `NORMAL` only after a measured hysteresis interval below every low watermark |
| Sticky `DRAIN`, assignment change/expiry, listener failure, or explicit shutdown | Close admission and withdraw new-work readiness | Drain only inside the remaining Work Safety Lease, then close | Join every listener, connection, pump, sampler, and worker before `WITHDRAWN` or `FAILED` |

`PROTECT` is not “the Node is healthy and accepting.” It keeps already admitted
active pairs alive, cancels pre-admission work, and makes new-work unavailability
observable. Capacity-full is an admission outcome, not a fabricated pressure
transition. `DRAIN` is terminal for that process/duty generation. An external
supervisor may start a fresh process only from fresh accepted State; the Node
Adapter does not restart or select another transport by itself.

### Functional-alpha topology candidates

| Candidate | Useful evidence | Honest limitation |
|---|---|---|
| Several isolated role processes/identities on one project-operated VPS | Cheapest complete functional tracer after all required duties exist | One host/operator; no host-loss, independence, availability, or privacy claim |
| Role-domain processes split across two or three project-operated VPS | Real inter-host paths and selected host/link fault injection | Still correlated project control and not independent operators |
| One host per logical carrier position | Clean process/host fault attribution | Highest early operating cost; common project ownership still is not independence |

**Product decision (2026-08-24):** the first complete functional-alpha tracer
may use one project-operated VPS with separate processes, identities, state
roots, assignments, listeners, resource ceilings, and terminal lifecycles. It
is a cost-bounded tracer only. It cannot replace per-duty NET-01A measurement or
later multi-host fault evidence, and a single process performing conflicting
domains remains rejected.

On the selected existing alpha VPS, the installed Rendezvous still has its own
systemd cgroup and fixed ceiling, and the declared ceilings plus observed host
reserve must fit together. A passing co-hosted campaign can qualify only this
project-operated functional-alpha service profile. It does not qualify a
general co-resident Endpoint product, host independence, public capacity, or
availability.

## Executed declared-host selection campaign

The executed experiment replaced the synthetic echo with the real Rendezvous
reservation/pairing/pump path and used this predeclared matrix:

1. Freeze one Product Owner-declared existing Ubuntu LTS `x86-64` host, its
   observed CPU/memory/disk/link/kernel/network envelope, cgroup v2 placement,
   build/source digests, and the exact candidate process limits. Physical host
   size is evidence, not an eligibility gate.
2. Measure idle/readiness, then sweep simultaneous pair capacity geometrically
   from one pair until the first safe refusal or resource falsifier. Pair count,
   accepted sockets, authenticated legs, and OS sockets are separate counters.
3. At the last passing point and the next higher point, run healthy full-duplex,
   stalled TLS, unmatched-leg, slow-reader/backpressure, reset/half-close,
   connection-churn, and saturated-new-attempt cells. The workload generator,
   byte totals, deadlines, and seeds are frozen before the retained run.
4. Run explicit `PROTECT`, resource `DRAIN`, assignment expiry/change, listener
   failure, `SIGTERM`, and abrupt-process-loss/restart cells. Abrupt loss may
   prove recovery behavior only; it cannot be relabelled graceful drain.
5. Run each deterministic decision-bearing cell once from fresh state, plus
   one bounded 260-cycle mixed workload paced at 250 ms. Execute independent supporting
   shards concurrently across both declared existing Ubuntu VPS hosts and
   local isolated Docker containers. Enforce a 60-minute total wall-clock
   bound, stop starting cells at minute 50, and reserve the final ten minutes
   for evidence collection and cleanup. Retain every attempt, including
   failures. After a correction repeat only the affected cell and one short
   ordinary installed-product smoke, never the complete campaign; a later pass
   does not remove the earlier failure from the denominator.

A capacity point passes only when every admitted pair authenticates the exact
TLS/ALPN/LegBinding contract, healthy pairs keep making useful progress, queues
and reservations remain within their declared bounds, saturated arrivals do no
new cryptographic/worker allocation, pair saturation closes other pre-admission
work, `PROTECT` closes pre-admission work while preserving active pairs, and all
terminal paths finish inside the Work Safety Lease with zero owned sockets and
workers. Any cgroup OOM, `memory.max`/PID/FD emergency, unbounded backlog in the
application, hidden retry, leaked worker, missing observation, or security
violation fails the point.

The selected capacity is the highest complete passing point below the first
failing point, with its measured high/low/emergency watermarks and hysteresis;
it is not inferred from RAM divided by one connection or from an H3 profile. If
even one pair cannot pass the complete matrix, R-092 selects no native profile.

## Recommendation

Historical recommendation: retain `h4-5-rendezvous-alpha-v1` as the sole
project-qualified dedicated-host Contributor profile. The maintained canonical
identity is now `ardents-rendezvous-dedicated-host-v1`; readers accept the
historical identity only for already pinned input. Keep the measured one-pair
reservations, resource placement, authenticated State inputs, terminal
lifecycle, and operator surface frozen.
Do not generalize the measurement into public capacity, availability,
co-resident contribution, Source independence, or independent-operation
claims. Initiator, Responder, Introduction, R-093, and permissionless admission
require separate future decisions.

## Disposition

Decided. H1 is accepted for one project-qualified dedicated-host Rendezvous
Functional Alpha profile. The maintained State/Node integration, installed
systemd lifecycle, bounded mixed workload, pressure/fault cells, update,
withdrawal, exact removal, and two-host/local supporting matrix supply the
decision evidence. H2 and H0 are not selected for this scope. This record adds
no public capacity, availability, co-resident, permissionless, incentive,
Source-independence, or independent-operator claim and requires no new ADR or
dependency.
