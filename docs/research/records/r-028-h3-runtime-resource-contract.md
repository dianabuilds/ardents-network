---
id: R-028
title: What bounded runtime resource contract applies to Horizon 3?
status: decided
owner: product research
started: 2026-08-11
reviewed: 2026-08-11
---

# R-028 — Horizon 3 runtime resource contract

**Current disposition:** accepted resource/evidence appendix to R-029. References
to standalone H3-A sequencing are historical and non-normative; R-029 controls
integrated Stage 1 scope and adds the distinct H3-NP1 role-probe profile.

## Decision this unlocks

Define how an H3 process obtains, accounts for, refuses, sheds, and releases CPU,
memory, file descriptors, sockets, goroutines, timers, queues, and Go runtime
work. The contract must keep hostile or failed work bounded without suppressing
the security behavior being measured.

This record supplies the reusable Endpoint/source resource and evidence plane
for accepted Stage 1 under R-029. R-029 adds a separate role-probe Node profile;
this record does not set public product SLOs or infrastructure Node capacity.

## Current contract

- [R-023](r-023-interactive-route-performance-budget.md) fixes endpoint floors,
  finite queues, honest backpressure, explicit capacity failure, controlled
  measurement, and a `2 vCPU`/`2 GiB`/`100 Mbit/s` infrastructure comparison
  class.
- R-023 explicitly leaves infrastructure Role capacity open until a selected
  prototype supplies a measurable role/work unit. The client `64/16`, publisher
  `256/64`, endpoint CPU/RSS, and endpoint queue totals cannot be copied to a
  Node or H3 source.
- [ADR-0009](../../adr/0009-go-project-foundation.md) selects Go 1.26.x and
  standard-library-first dependencies.
- [R-027](r-027-h3-first-slice.md) proposes one small persistent Ubuntu slice,
  finite sources, at most two parallel fetches, a `1 MiB` Epoch cap, and one
  bounded ordered readiness-event and verification stream.
- Security work, validation, source exclusion, evidence, and cleanup remain
  charged. A benchmark cannot pass by disabling them or moving work outside the
  measured process tree.

## Decision question

Can one explicit hierarchical resource contract make H3-A bounded now and
extend to later role processes without freezing guessed Node capacity or relying
on Go's GC, the kernel OOM killer, or connection eviction as admission control?

## Hypotheses

- **H1 — fixed profile plus reservations:** OS/cgroup hard boundaries, a fixed
  Go runtime profile, and reservation before allocation make overload explicit
  and measurable.
- **H2 — runtime auto-tuning:** Go and the OS can safely derive limits and adapt
  them automatically from the host without an explicit profile.
- **H3 — library resource manager:** a general library such as go-libp2p's
  Resource Manager can own the complete problem now.
- **H0:** no proposed approach contains H3 without unacceptable performance or
  complexity; implementation stops.

## Evaluation criteria

The selected approach must give every resource a finite parent, reserve before
expensive work, preserve required control/established progress under overload,
produce an explicit capacity result, shut down completely, and be measurable by
the OS rather than candidate self-report. It must fit H3-S without copying
endpoint concurrency floors, allow later measured scale-up, add no production
transport/orchestrator by inertia, and remain maintainable by one Product Owner
and Codex.

## Findings

- **Sourced fact:** Go's memory limit is soft and excludes the binary, kernel
  memory, mmap not managed by Go, and non-Go allocations. It cannot substitute
  for a cgroup limit. [Go GC guide](https://go.dev/doc/gc-guide) and
  [`debug.SetMemoryLimit`](https://pkg.go.dev/runtime/debug), accessed
  2026-08-11.
- **Sourced fact:** Go 1.26 enables Green Tea GC by default. It changes expected
  GC efficiency, not the need for measurement or bounds.
  [Go 1.26 release notes](https://go.dev/doc/go1.26), accessed 2026-08-11.
- **Sourced fact:** container-aware `GOMAXPROCS` periodically observes a CPU
  limit when no explicit value is set, but the Go 1.26 runtime source reads only
  the leaf cgroup and does not notice later cgroup migration. A tighter parent
  can therefore be missed. [Go container-aware GOMAXPROCS](https://go.dev/blog/container-aware-gomaxprocs)
  and [runtime source](https://go.dev/src/runtime/cgroup_linux.go), accessed
  2026-08-11.
- **Sourced fact:** cgroup v2 accounts descendant memory including kernel and
  socket memory, supplies throttling/high and hard/max boundaries, CPU pressure,
  and PID controls. [Linux cgroup v2](https://www.kernel.org/doc/html/latest/admin-guide/cgroup-v2.html),
  accessed 2026-08-11.
- **Sourced fact:** `RLIMIT_NOFILE` fails excess descriptor creation with
  `EMFILE`; it is a fuse, not a fairness policy.
  [`getrlimit(2)`](https://man7.org/linux/man-pages/man2/getrlimit.2.html),
  accessed 2026-08-11.
- **Inference:** H1 is the only approach that is explicit enough for evidence
  and small enough for the one-to-one team. H2 can silently misread the
  effective environment. H3 is tied to libp2p scopes, adds a large dependency
  closure, and still does not provide application fairness or Ardents overload
  semantics.

## Options

| Option | Fit | Rejection risk | Disposition |
|---|---|---|---|
| H1: fixed profile and owned reservations | Exact limits, deterministic evidence, small dependency surface, explicit overload. | Requires us to own a small accounting state machine and prove release correctness. | **Recommend.** |
| H2: runtime/OS auto-tuning | Low implementation effort and may use powerful hosts opportunistically. | Effective parent limits, cgroup migration, Go-external memory, attacker-driven tuning, and evidence identity remain ambiguous. | Reject for fixed H3 evidence. |
| H3: general resource-manager dependency | Mature scope ideas and existing instrumentation. | Pulls a transport-shaped closure, still needs Ardents fairness/readiness, and risks Gate D lock-in. | Reference only. |
| H0: stop | Avoids an unsafe unbounded daemon. | Leaves H3 unable to proceed. | Use if H1 falsifies. |

## Resource model

The accounting hierarchy is:

```text
host/VM
  -> Ardents test installation cgroup
    -> role process cgroup
      -> work class
        -> authenticated peer or Direct Source
          -> Carrier Channel / acquisition
            -> logical stream / operation
```

H3-A implements only the levels it actually uses: process, work class,
source/peer, and acquisition/readiness-event operation. Later H3-B/C may deepen the same
shape. They must not give every child a copy of the parent budget.

Resources are reserved before the operation can allocate them:

- one bounded work token before a goroutine starts;
- one connection/FD token before dial or accept advances;
- byte credit before reading, buffering, or queueing;
- CPU/admission token before expensive parsing or cryptography;
- timer ownership before scheduling retry or expiry;
- disk/evidence credit before retaining output.

Every successful reservation returns an owned lease released exactly once on
success, failure, timeout, or cancellation. A counter repaired by periodic
recount is diagnostic, not correct ownership.

## H3-S fixed experiment profile

These are proposed H3-A **experiment inputs**, not public product promises and
not Node useful-capacity floors.

| Boundary | H3-S value | Meaning |
|---|---:|---|
| Profile identifier | `h3-s-v1` | Binds every value in this table and the exact workload/corpus policy below. |
| Candidate host/VM class | `2 vCPU`, `2 GiB RAM`, symmetric `100 Mbit/s` | E, S1, and S2 in the canonical R-027 topology. |
| Candidate CPU cgroup ceiling | `cpu.max = 160000 100000`; `cpu.max.burst = 0` | Exact `1.6` cores with a `100 ms` period and no burst; leaves `0.4` host core for OS and harness-owned local collection. The systemd fixture uses `CPUQuota=160%` and `CPUQuotaPeriodSec=100ms`. |
| Normal CPU gate | mean `<= 1.12` cores; p95 one-second `<= 1.28` | Retains `30%` mean and `20%` p95 headroom inside the candidate quota; this is not an instantaneous guarantee. |
| `GOMAXPROCS` | fixed `2` after preflight | Avoids unnoticed parent/migration changes during a fixed run. |
| `memory.max` | `1280 MiB` | Process-tree hard boundary, below host RAM. |
| `memory.high` | `1152 MiB` | Kernel reclaim/throttling boundary, deliberately above the normal gate. |
| Normal `memory.current` gate | p95 `<= 896 MiB` | Leaves `256 MiB` before `memory.high` and requires zero new `high/max/oom/oom_kill` events in normal cells. |
| `GOMEMLIMIT` | `768 MiB` | Soft target for Go-managed memory, including goroutine stacks; cgroup headroom covers executable mappings, page cache, kernel/socket memory, evidence, and other non-Go cost. |
| `GOGC` | `100` | Stable first candidate; changes require a new profile identity and full affected rerun. |
| Swap | disabled for the measured cgroup | Prevents hidden latency/state movement in the H3 evidence profile. |
| `RLIMIT_NOFILE` | `4096` | OS fuse; ordinary H3-A working cap is much lower. |
| Internal working FD cap | `256` | Includes listeners, connections, files, pipes, diagnostics, and evidence handles. |
| `pids.max` | `512` | Process/thread fuse for the complete cgroup. |
| Goroutine working cap | `1024`; emergency fuse `4096` | Admission uses the lower cap; the higher value is a terminal invariant violation. |
| Owned timer working cap | `256`; emergency fuse `512` | Every retry, deadline, refresh, expiry, and shutdown timer owns one token. |
| Runtime thread warning/drain threshold | `64` / `256` | Warning triggers protection and `256` triggers DRAIN; H3-A does not call `debug.SetMaxThreads`, whose breach would terminate the process rather than permit graceful drain. `pids.max=512` remains the OS hard fuse. |
| Role socket-handle cap | Endpoint `16`; each distributor `56` | Distributor decomposition is `32` post-auth + `16` incomplete + `1` listener + `1` local diagnostic + `1` rejection reserve + `5` headroom; all remain charged to the FD cap. |
| Endpoint direct requests | `2` concurrent; finite manifest set | Matches R-027 acquisition, not general network capacity. |
| Distributor accepted requests | `32` concurrent | H3-A test-source capacity only. |
| Incomplete distributor handshakes | `16` concurrent, disjoint from `32` post-auth requests | Staged pre-authentication cap; the socket formula above includes both scopes. |
| Epoch bytes | `1 MiB` per object; `64 KiB` read chunk | R-027 hard object and allocation bounds. |
| Aggregate acquisition bytes | `4 MiB` total resident logical credit | Includes two complete raw envelopes plus all parsed representation, canonical-check, digest, and signature temporary bytes. |
| Control queue | `4096` items and `4 MiB`; one item `<= 4 KiB` | Both item and byte credits are required; a full critical queue drains explicitly. |
| Pre-authentication state | `32 KiB` per attempt, `512 KiB` aggregate | Exactly 16 incomplete attempts; no peer-controlled unbounded metadata. |
| Distributor response memory | one current plus at most one retiring immutable Epoch, each `<= 1 MiB` (`2 MiB` object cap); `64 KiB` resident per active writer and `2 MiB` aggregate writer buffers | A writer leases one exact object for at most the `5 s` response deadline, so an atomic swap cannot mix Epoch bytes. Thirty-two clients do not receive thirty-two full resident copies. Total object-plus-writer logical credit is `4 MiB`; normal `32 KiB` corpus objects use at most `64 KiB` for current+retiring. |
| Owned socket-memory pressure | observe cgroup `memory.stat sock`; `128 MiB` Protect, `192 MiB` Drain | This is observable pressure inside total cgroup memory, not a separately enforceable kernel hard cap. |
| Persistent Epoch state | `8 MiB` directory | At most current and previous generation plus one temporary generation, each deterministic state blob `<= 2560 KiB`; active and optional pending Epochs live inside the blob. Pointer and bounded cycle/history metadata share the remainder; surplus/orphans fail cleanup. |
| Read-only valid corpus input | exactly `192` envelopes, each `<= 32 KiB`; envelope bytes total `<= 6 MiB`, complete directory `<= 7 MiB` per distributor | Pre-generated and manifest-bound before start. Kernel file-page ownership is not assumed to migrate to H3-S: it is measured in a separate bounded corpus-input cgroup as declared below. Candidate read/validation/activation CPU and its resident current object remain H3-S work. The general hostile-object parser cap remains `1 MiB`. |
| Candidate evidence/diagnostic memory | `4 MiB` | Periodic diagnostics may coalesce with an explicit count; verdict/security transitions never drop and reserve exhaustion drains/fails. |
| Candidate local evidence disk | `64 MiB` total | Verification-capsule spool: at most `16` capsules and `48 MiB` (`15` ordinary + one reserved terminal); other rotating candidate evidence: `16 MiB`. A valid run requires `dropped_verdict_events == 0` and digest-matching external spool/verification acknowledgement before any capsule is reclaimed. |
| External evidence disk | `2 GiB` per candidate-host stream plus one `2 GiB` harness stream; H reserves `8 GiB` total per campaign | Preflighted outside Git; exhaustion invalidates the run and never deletes earlier samples silently. The frozen schema must project all maximum encoded samples/transitions to `<=80%` of each partition before start. |

The canonical topology is exactly the four VMs E/S1/S2/H fixed by R-027. E, S1,
and S2 each run one candidate H3-S cgroup and one harness-owned local collector
outside it. H contains the central collector, load generators, harness,
mechanically separate readiness verifier, and pre-run offline signer.

### Profile ownership map

| Role/process tree | Accounting parent | Contract applied |
|---|---|---|
| Endpoint Epoch/bootstrap command and every child it starts | one Endpoint H3-S cgroup | Full H3-S table and Endpoint workload verdict. No Epoch work may escape this tree. |
| Each source-only distributor and every child it starts | one separate distributor H3-S cgroup on its own host | Full H3-S containment plus distributor workload verdict; this is test-source capacity, not Node capacity. |
| Immutable corpus staging on each S host | one separate persistent corpus-input cgroup | `16 MiB memory.max`, `32` FD, `32` PIDs; staging completes before candidate start, no process remains during a cell/campaign, and the collector retains `memory.current` plus `memory.stat file`. This is an explicit lab-fixture cost, not H3-S useful work. |
| Offline signer invocation on H | separate one-shot cgroup | `1` core, `256 MiB memory.max`, `64` FD, `64` PIDs, `60 s` deadline; recorded but excluded from online 100% capacity. |
| Local collector on each E/S1/S2 | harness-owned cgroup outside candidate | `0.01` core mean, `16 MiB` RSS, `32` FD, `32` PIDs; candidate never controls it. |
| Two open-loop generators plus paired source/sink probes on H | one harness-owned workload cgroup | Aggregate cap `2` cores, `2 GiB memory.max`, `512` FD, and `256` PIDs. |
| Central collector plus readiness verifier on H | one harness-owned evidence cgroup | Aggregate cap `0.5` core, `512 MiB memory.max`, `256` FD, and `128` PIDs. |
| Campaign/impairment controller and all child fixture tools on H | one harness-owned controller cgroup | Aggregate cap `0.5` core, `512 MiB memory.max`, `256` FD, and `128` PIDs; every `tc`, probe, lifecycle, and cleanup child remains in this tree. |

H is `4 vCPU/8 GiB/1 Gbit/s`; the three online H parents above total at most
`3` cores and `3 GiB`, leaving a host reserve. The offline signer cannot overlap
a measured cell. The execution manifest names every process tree and parent cgroup. Co-residence
or a different physical/link map is useful only for non-qualifying reproduction;
it cannot change the canonical evidence verdict.

Before any candidate starts, the harness places the identical content-addressed
read-only corpus and canonical activation schedule on S1 and S2 through the
management plane, records their directory digests, removes write access from the
candidate identities, and then leaves the online path. Staging occurs in the
manifest-named corpus-input cgroup, which remains as the accounting owner of any
file pages it instantiated; it has no live process after preflight. Its
`memory.current` must stay `<= 16 MiB`, its `memory.stat file` must stay
`<= 8 MiB`, and both are retained beside H3-S evidence. The run never claims
that later reads transfer those file-page charges to the candidate cgroup.

Each distributor's own state owner verifies and loads only the scheduled next
digest at the frozen local monotonic offset, atomically swaps its one shared
current object, and charges read/validation CPU, allocations, resident current
object, timer, and swap work to its H3-S cgroup. H sends no runtime activation
command and performs no online signing. A missing, early, late, mismatched, or
non-atomic activation is an explicit source/campaign failure. The immutable
warm corpus is a declared H3-A fixture rather than a hidden capacity subsidy;
H3-B/H3-C must measure the selected production state-distribution and storage
path instead of inheriting this exclusion.

H3-A load cells prove only local containment, bounded rejection, and absence of
obvious leaks for the lab Epoch path. They establish no useful Node capacity,
multi-peer fairness, route recovery, or production scaling floor; those remain
H3-C even if the H3-A offered workload succeeds.

## Scale-up contract

H3-A uses H3-S only. A stronger machine does not silently increase concurrency,
trust, role count, source selection weight, or security authority.

A future H3-L candidate may use an `8 vCPU`/`16 GiB`/`1 Gbit/s` host, but its
exact process limits remain unset until H3-B/C define one measurable useful-work
unit. Enabling any larger automatic profile requires:

- an immutable profile identifier binding every CPU, memory, FD, socket, queue,
  goroutine, timer, and bandwidth limit;
- at least `4x` demonstrated useful work, not merely more idle handles;
- all security and failure scenarios unchanged;
- mean CPU, p95 one-second CPU, p95 memory, FD, socket, queue, and link use each
  at or below `80%` of its declared parent budget;
- no new role, priority, trust, family, or governance power;
- a full affected evidence rerun.

This preserves scale-up without guessing linearity or hard-coding H3-S as a
product maximum.

## cgroup and runtime preflight

Before a role process becomes live or experimentally ready, the **lab harness**
records and verifies:

1. cgroup v2 controllers available for CPU, memory, and PIDs;
2. the process's complete cgroup ancestry, not only its leaf;
3. effective `cpu.max`, `cpu.max.burst`, affinity/cpuset, `memory.high`,
   `memory.max`, swap, `pids.max`, and parent ceilings;
4. soft/hard `RLIMIT_NOFILE` and current descriptor use;
5. actual `GOMAXPROCS`, `GOGC`, and `GOMEMLIMIT` reported through the fixed local
   candidate evidence seam;
6. distributor-host `net.core.somaxconn=64`; the observed SYN-backlog controls
   are retained but are outside the H3-A accept-queue claim;
7. sufficient host reserve for the collector and operating system;
8. one stable cgroup location for the run.

The unprivileged candidate validates only its accessible leaf cgroup identity,
effective runtime values, rlimits, and fixed profile identity; it receives no
privilege to walk or modify host controls. The effective CPU budget is the
tightest applicable affinity/cpuset and quota
across the hierarchy. After that check H3-S sets `GOMAXPROCS=2` explicitly.
Changing a parent quota, memory bound, cpuset, or moving the process during a
fixed run makes the role non-ready and the evidence run invalid or failed under
its predeclared fault case; the runtime does not silently retune the profile.

### Network impairment ownership

H is the controller, not a data-plane router. Before candidate start, the
harness uses the separate management plane to install and verify host-level
ingress classifiers on every candidate-data interface of E, S1, S2, and the H
load-client namespace. A classifier keys on the manifest's exact remote/local
data IP pair and redirects that peer's ingress flow to its own harness-owned
IFB; every IFB has exactly one root qdisc with the receiver-side profile `netem
limit 1000 delay 40ms 3ms distribution normal loss random 0.1% rate 100mbit seed
<manifest-direction-seed>`. Candidate-data traffic that matches no declared
peer classifier invalidates the run. L1 and L2 have distinct namespace/data IPs,
so their flows cannot share a classifier. Applying impairment at receiver
ingress avoids making H part of E-to-S traffic and follows netem's TCP placement
limitation. There is no classifier, IFB, or impairment on the management
interface.

Each ordered remote/local peer pair therefore has its own precommitted seed and
qdisc rather than pretending one interface-wide netem instance has per-link
randomness. The four qualifying round-trip pairs are exactly `E<->S1`,
`E<->S2`, `L1<->S1`, and `L2<->S2`; there is no H-management probe.
The manifest binds Ubuntu's packaged `iputils-ping` version/digest and one exact
host-level probe: `1000` ICMP Echo Requests, `64` data bytes, one start every
`20 ms`, and `1 s` per-sequence timeout. Sequence IDs, send/receive timestamps,
and losses are retained; lost probes never disappear from the denominator.

The two `40 ms` receiver-side delays add `80 ms` emulated RTT. Before impairment,
every pair must receive all `1000` replies with p50 `<= 5 ms` and
`p95 - p50 <= 2 ms`. After impairment, another `1000` requests must receive at
least `990` replies; successful replies must show p50 in `[80 ms, 86 ms]` and
`p95 - p50 <= 10 ms`, while every loss remains reported separately. Fewer
replies, an unmatched classifier, or a changed probe invalidates the run.
Underlying VM/network scheduling is therefore measured rather than erased from
the claim. Ubuntu's packaged
kernel and `iproute2/tc` versions, qdisc tree, seeds, interface/namespace IDs, and
pre/post `tc -s` output are manifest-bound. Missing privilege, unsupported
options, configuration drift, qdisc backlog reaching the `1000` packet limit, or
wrong placement invalidates the run.

Qdisc drops under the frozen random-loss fixture are retained separately from
NIC/driver, namespace, TCP, and host link error/drop counters; every unexpected
counter delta must be zero. The harness also records host total CPU/softirq and
the non-candidate remainder. On E/S1/S2, collector plus impairment/OS work must
remain `<=0.20` core mean and `<=0.30` core p95 during normal cells; it is outside
candidate H3-S accounting but cannot become hidden host saturation.

Impairment is a harness-owned `try/finally` resource. On success, failure, or
invalid run, the harness removes only the manifest-named ingress filters, qdisc
handles, and IFB devices on E/S1/S2/H-load, then records and compares the full
post-clean network tree with the pre-run snapshot. A missing ownership record,
cleanup of an unrelated handle, or any residual fixture invalidates cleanup and
the campaign.

## CPU and concurrency

Go has no supported goroutine priority scheduler. Work isolation therefore uses
separate bounded queues, semaphores, and worker pools rather than hoping the
runtime will prefer control work.

The following percentages are design rationale only, not admission weights or
acceptance criteria:

- `60%` accepted-state and in-flight refresh progress;
- `5%` safety/readiness/control work;
- `10%` new acquisition/admission and cryptographic verification;
- `5%` refresh, cleanup, and bounded maintenance;
- `20%` unallocated reserve.

They explain why the normative normal CPU gate is `70%` mean and `80%` p95 of
the `1.6`-core candidate quota; the implementation does not claim class-level
CPU accounting. Normative concurrency is concrete: Endpoint has one serialized
state owner, one expensive validation worker, at most two acquisition I/O
operations, and one readiness-event operation; a distributor has at most 16 incomplete
handshakes and 32 authenticated response writers. A goroutine starts only after
its work token is acquired; cancellation reaches every worker, read, write,
timer, and child operation. There is no fire-and-forget retry or metric
goroutine.

## Socket and descriptor management

- listeners, accepted sockets, outbound dials, state files, pipes, profiles, and
  diagnostics all consume the same process FD parent budget;
- the listener owns one reserved accept slot before calling `Accept`; the slot
  transfers to the returned connection. When no slot is available the loop does
  not call `Accept` and relies only on the VM fixture's verified
  `net.core.somaxconn=64`, which is the accept-queue ceiling used by Go's ordinary
  `net.Listen`. H3-A adds no custom listener syscall/`unsafe`; the TCP SYN backlog
  is observed but explicitly not claimed. Outbound work reserves its
  connection/FD token before `Dial`;
- one distributor socket is reserved for bounded rejection. A framed,
  authenticated request over the post-auth cap receives fixed `BUSY` within
  `250 ms` measured from receipt of the complete authenticated request frame to
  completion of the `BUSY` frame write. Before authentication or when the
  rejection reserve itself is busy, reset or client-side timeout by its `5 s`
  deadline is an honest capacity terminal class, never a typed-success promise;
- source connections have `1 s` connect, `2 s` cumulative TLS, `3 s` first-frame,
  `1 s` read/write-idle, and `5 s` hard total deadlines. The server applies the
  same `2 s` TLS, `500 ms` post-TLS request-frame, `1 s` write-idle, and `5 s`
  total bounds;
- authentication mismatch, incompatibility, cancellation, and terminal expiry
  close immediately and do not retry until their governing state changes;
- the complete H3-A source wave has a `15 s` hard total bound;
- sockets are closed explicitly; finalizers are not lifecycle management;
- lowering `RLIMIT_NOFILE` and forcing `EMFILE` must cause bounded protection,
  not a spin loop or loss of established control progress.

Later UDP/QUIC transports may multiplex many streams over one FD. Therefore
descriptor, socket, Carrier Channel, logical stream, and queue limits always
remain separate dimensions. H3-A selects none of those transports.

## Memory, queues, and backpressure

The memory hierarchy accounts logical reserved bytes, while cgroup evidence
accounts physical consequences including copies, Go overhead, stacks, kernel
socket buffers, page cache, and slab.

Rules:

- length is validated against a hard cap before allocation;
- reads are chunked and charged as bytes become resident;
- no untrusted length creates a same-sized allocation before cheap framing
  checks;
- no queue is unbounded and no full queue spills to disk;
- producer success reports only accepted bytes/work; a full queue blocks under a
  caller deadline or returns explicit would-block/capacity pressure;
- there is no silent drop, random connection eviction, false success, or
  borrowing from another source/peer/context;
- security and control queues are reserved but finite; a full critical queue
  enters drain rather than discarding a security transition;
- an ordinary state transition reserves one verification-capsule slot and its
  bounded bytes before commit. Capsule pressure uses the reserved terminal slot
  for `evidence_failure`; exhaustion of that slot invokes emergency fail-stop
  and never deletes an unacknowledged state/event snapshot;
- cache entries have count, byte, time, and governing-Epoch bounds;
- evidence/log rate is bounded independently so an attacker cannot create a
  logging memory/disk DoS.

Before writing a response, the distributor acquires a refcounted lease on the
exact immutable current object. Atomic activation publishes the new object and
marks the old one retiring; no new writer may lease the retiring object, while
existing leases finish or are cancelled by their `5 s` total deadline. A second
activation cannot publish until the retiring refcount is zero; failure to clear
it by `5 s` is a source/campaign failure and enters DRAIN rather than retaining a
third object. Each writer owns one `64 KiB` chunk buffer and never materializes a
second full response. State-generation cleanup keeps only current, previous, and one
temporary generation within the `8 MiB` directory budget. Queue, timer, file,
and evidence limits are independent; exhausting one cannot borrow silently from
another.

R-023's `256 KiB` per Service Connection and direction remains the later
Application Data leaf limit. H3-A does not reinterpret the two raw envelopes
(`2 MiB` inside the complete `4 MiB` acquisition resident cap) as a new Service
Connection queue.

For H3-C, hierarchical deficit round robin is the leading fairness candidate
across work class, peer/Channel, and stream, with token buckets for bandwidth.
It is not implemented in H3-A, which has only finite source acquisitions and one
bounded readiness-event path. A single global FIFO is rejected for later hostile multi-peer work.

## Go garbage collector contract

H3-S starts with:

- the repository-approved patched Go `1.26.5` toolchain and its default Green
  Tea collector; a later security patch creates a new recorded candidate
    identity and reruns affected cells rather than retaining a vulnerable runtime;
- `GOGC=100`;
- `GOMEMLIMIT=768MiB`;
- no `GOGC=off`;
- no `runtime.GC` or `debug.FreeOSMemory` in a hot path;
- no finalizer-dependent release of sockets, files, reservations, or secrets;
- no automatic runtime retuning from attacker-controlled load.

The Go memory limit is deliberately below the cgroup high/max boundaries. It
includes heap, goroutine stacks, and other Go-managed memory, but excludes
kernel/socket memory and some mapped/non-Go memory; it is also soft. It is a GC
goal, not an admission limit. If the live set cannot fit, the response is to
reject or drain work and redesign allocations, never raise `GOMEMLIMIT`
automatically.

Collected runtime signals include:

- live heap, heap goal, allocation/free rates, object count, stacks, and total
  runtime memory classes;
- GC cycles, CPU fraction, p50/p95/p99 pauses, and GC limiter activation;
- goroutine count/state, runtime thread count, scheduler latency, mutex/block
  pressure, and `GOMAXPROCS`;
- configured `GOGC` and `GOMEMLIMIT` as evidence inputs.

For every pressure and acceptance calculation, **Go managed memory** is exactly
`/memory/classes/total:bytes - /memory/classes/heap/released:bytes`, matching the
runtime memory-limit accounting definition. Live heap remains a separate leak
signal and never substitutes for that value. The current OS-thread count comes
from the `Threads` field of `/proc/self/status` for candidate diagnostics and
`/proc/<pid>/status` for the external collector; H3-A does not invent a missing
`runtime/metrics` thread gauge.

Proposed H3 gates use snapshots at each cell boundary and never mix runtime and
OS accounting:

- GC CPU ratio is
  `delta(/cpu/classes/gc/total:cpu-seconds) /
  delta(/cpu/classes/total:cpu-seconds)`: `<= 10%` in normal work and `<= 20%`
  in the declared overload cell. A zero denominator invalidates the cell;
- GC stop-the-world p99 comes only from the bucket-count delta of
  `/sched/pauses/total/gc:seconds`: `<= 10 ms` normal and `<= 50 ms` overload;
- runnable scheduler-latency p99 is a separate bucket-count delta of
  `/sched/latencies:seconds`: `<= 20 ms` normal and `<= 100 ms` overload;
- for each cumulative histogram, subtract start counts bucket by bucket, select
  the nearest bucket whose cumulative delta reaches p99, and use that bucket's
  upper bound. No observations means `not applicable`, never zero latency;
- `/gc/limiter/last-enabled:gc-cycle` must not change in a normal cell; every
  cumulative cgroup/runtime counter is likewise judged by its cell delta;
- after churn stops, wait exactly `120 s`. If fewer than two natural GC cycles
  completed, a test-only local harness hook invokes two sequential `runtime.GC`
  calls. Because `runtime.GC` is synchronous and cannot accept a context, an
  external harness watchdog kills the candidate and fails the cell if both calls
  have not returned within `30 s`. Forced-cycle CPU and latency are excluded from
  workload gates but retained; the post-probe live heap must be no more than
  `max(5%, 8 MiB)` above the comparable pre-churn probe.

The forced-GC hook exists only in the lab command, is unavailable on any network
listener, and is never called by production-path or pressure logic.

Each unattended campaign keeps one exact **soak** workload: E offers one refresh
every `30 s`; S1 and S2 each receive a separate open-loop `1 response/s` schedule;
the pre-generated direct successor becomes available once per hour; and every
hour must durably activate or classify the manifest-expected digest. H
separately verifies every newly emitted readiness event and the terminal
campaign verdict; no Route, Named Site, or Application Data workload runs during
the soak.
Endpoint and per-distributor success gates remain `>=99%` overall and `>=98%`
inside every completed active hour, with no `5 min` interval lacking useful
progress. Scheduled probe windows are absent from the offered denominator.

This is intentionally not the short-cell `100%` load. Before either campaign,
the manifest enumerates every start and object length, computes payload bytes,
adds a `15%` TLS/TCP/retransmission allowance plus Endpoint bytes, and freezes the
maximum candidate-link byte budget. The `72 h` campaign budget is `<= 19 GiB`,
the independent `7 day` campaign budget is `<= 43 GiB`, and their combined
prescribed-soak budget is
`<= 65 GiB`. A projection above those caps does not start.

For this oracle, each packet is counted exactly once: the authoritative value is
the sum of the pre-redirect matched-byte deltas on the receiver-ingress classifier
for every manifest-declared directed peer flow during `[workload_start,
workload_stop)`. This includes request/response headers, TLS/TCP/IP overhead,
ACKs, and retransmissions; sender-interface and post-netem counters are retained
only for reconciliation and are never added again. Management traffic and the
separately recorded pre/post ICMP fixture probes are outside that interval. The
manifest records each directed delta, their integer sum, exact payload bytes,
Endpoint bytes, and the `15%` non-payload allowance. An authoritative sum above
its frozen budget or any counter reset/wrap/identity ambiguity fails the campaign.
This prevents the soak from silently becoming a multi-terabyte operating
commitment or passing through double-counted/implementation-selected counters.

Every normal CPU, throttling, memory, GC, scheduler, FD/socket, queue, and
progress gate is evaluated for each completed active hour and again over the
whole campaign. The fixed probe windows are excluded from workload percentiles
but retained separately. One failed active hour fails the campaign; a whole-run
p95 cannot hide it.

The soak leak oracle is exact. The precomputed schedule contains no offered start
in `[55:00, 57:35)` of each hour. At `55:00`, the harness permits `5 s` for
in-flight work to finish, waits `120 s`, then allows at most `30 s` for the
declared forced-GC hook when needed and captures the probe. If it finishes early,
all roles remain quiescent until the fixed `57:35` resume; if it does not finish
by then, the hour fails. After a `6 h` warm-up, compare the median of
the first six eligible probes with the median of the final six for each E/S1/S2
candidate cgroup separately:

- post-probe live heap and live-object count may grow by at most
  `max(5%, 8 MiB)` and `max(5%, 1024 objects)` respectively;
- each eligible interval is the counter delta from the preceding fixed resume to
  the current probe. Allocated bytes per successful useful unit **and** per
  delivered payload byte in the final six intervals must each be no more than
  `110%` of the first-six-interval value. Every interval must have useful work,
  and the corpus preflight requires all normal object sizes within `1%` of its
  median so changing payload mix cannot hide or invent allocation growth;
- process-tree anonymous RSS and cgroup `memory.stat anon` may grow by at most
  `max(5%, 16 MiB)` and `max(5%, 16 MiB)` respectively. Total
  `memory.current` retains its absolute `896 MiB` normal gate rather than a
  cross-window growth oracle;
- open FD, goroutine, OS-thread, and owned-timer medians may increase by at most
  `16`, `32`, `4`, and `8` respectively;
- open socket handles may increase by at most `4`; cgroup socket memory may grow
  by at most `max(10%, 8 MiB)`;
- candidate `memory.stat file <= 160 MiB` at every probe, and candidate
  `memory.stat slab` may grow by at most `max(10%, 8 MiB)`; each separate
  corpus-input cgroup remains `memory.current <= 16 MiB` and
  `memory.stat file <= 8 MiB`. File cache is therefore bounded under its actual
  observed owner rather than mislabeled as an anonymous-memory leak or silently
  credited to H3-S;
- every candidate queue has zero items/bytes at the probe; cache entry count may
  not increase and cache bytes may grow by at most `max(5%, 1 MiB)`;
- mutable state stays `<= 8 MiB`, read-only corpus on disk `<= 7 MiB`, candidate evidence
  `<= 64 MiB`, each external stream `<= 2 GiB`, H aggregate external evidence
  `<= 8 GiB`, and `dropped_verdict_events == 0` throughout.

A missing probe, zero useful-work denominator, or failed final quiescence probe
invalidates the soak; it is never interpreted as no growth.

An A/B experiment may compare `GOGC=50`, `100`, and `200`, but each value is a
new candidate/profile identity with the same memory, latency, and security gates.
The default is not changed from `100` without evidence.

## Pressure state machine

```text
NORMAL -> PROTECT -> DRAIN -> EXIT
   ^          |
   +----------+  only after every low watermark holds for 120 s
```

Enter `PROTECT` after three consecutive one-second samples of any:

- charged CPU at or above `80%` of the cgroup budget;
- `memory.current >= 75%` of `memory.max`;
- Go managed memory at or above `90%` of `GOMEMLIMIT`;
- FD, goroutine, timer, acquisition-byte, or queue use at or above `80%` of its
  working cap;
- cgroup `memory.stat sock >= 128 MiB`;
- runtime thread count at or above `64`;
- `cpu.pressure some avg10 >= 20.00`, `memory.pressure some avg10 >= 5.00`, or
  `io.pressure full avg10 >= 1.00`.

A positive delta in `memory.events.local high` is latched and enters `PROTECT`
immediately; it does not wait for three samples.

In `PROTECT`:

- `H3-A Epoch Ready (unqualified)` is false and no new readiness-event/acquisition work
  starts;
- stop optional prewarm, probing, profile capture, and background work;
- stop or reduce new expensive acquisition/admission;
- return explicit pressure for new operations whose reserve is unavailable;
- preserve the finite control reserve and already-established validation progress;
- do not raise limits, rotate sources without budget, or evict random work.

Enter `DRAIN` immediately on any:

- `memory.current >= 1152 MiB`, `memory.stat sock >= 192 MiB`,
  `memory.events.local` max/oom/oom_kill activity;
- FD, goroutine, or timer use at or above `90%` of the working cap, or runtime
  threads at or above the `256` fuse;
- full critical control queue;
- failure of established work/control progress within its deadline;
- changed/unknown cgroup contract or unrecoverable accounting invariant.

`DRAIN` removes experimental readiness, accepts no new work, cancels
acquisitions, preserves
only bounded safety/close work, and exits on deadline. Return from `PROTECT` to
`NORMAL` requires **all** resource-specific low watermarks for `120` consecutive
one-second samples: CPU below `60%` of quota; `memory.current <= 896 MiB`; Go
managed memory below `80%` of `GOMEMLIMIT`; socket memory below `64 MiB`; and FD,
goroutine, timer, acquisition-byte, queue-item, and queue-byte use below `60%` of
their working caps; OS threads below `48`; `cpu.pressure some avg10 < 10.00`;
`memory.pressure some avg10 < 2.50`; and `io.pressure full avg10 < 0.50`. It also
requires no new memory-event or accounting error. Admission then returns in
stages. `DRAIN` never returns to ready in the same process run.

## Shutdown contract

On normal stop or first termination signal:

1. at `t0`, publish not-ready, close new listeners/admission, and cancel refresh;
2. by `5 s`, cancel incomplete handshakes and acquisitions;
3. by `30 s`, finish or terminate the one established validation/readiness-event work;
4. at `30 s`, set final socket/file deadlines and cancel remaining workers;
5. by `45 s`, join every candidate-owned tracked goroutine and evidence writer;
6. by `60 s`, close descriptors, flush bounded evidence/state, and exit.

An inability to durably commit or emit a readiness-removing transition uses a
separate emergency fail-stop, because the normal `60 s` path would leave an old
positive event externally plausible for too long. The candidate synchronously
sets in-memory not-ready, closes admission/listeners, and requests exit; it must
exit voluntarily within `1 s`, and the manifest-bound service supervisor sends
the final kill no later than `2 s` after the original failure. No candidate-local
flush is required to claim this path; the external collector records failure,
listener closure, service death, and cleanup. Surviving candidate state is
revalidated at the next start. Missing the `2 s` death bound fails the fault cell.

A second termination signal may close immediately but is recorded as forced,
not graceful. A graceful Node withdrawal is an H3-B network-state operation and
must not be inferred from a process signal or crash.

The harness, not the candidate, owns the external collector. It keeps collecting
through candidate exit, stops it only after recording OS-level cleanup, and then
checks that its own collector/workload-client cgroups are empty. Candidate
success never depends on controlling that outside-candidate observer. It remains
under the same real project operator and is not independence evidence.

## Diagnostics and privacy

External evidence sampled every second is authoritative for resource claims.
The same local collector additionally records service/process/cgroup liveness at
least every `250 ms`; a sample older than `500 ms` cannot support external
readiness and the worst-case encoded liveness volume is included in the `80%`
disk preflight. Resource samples contain:

- process-tree CPU time, RSS, thread/process counts, and FD/sockets;
- cgroup `cpu.stat`, throttling, pressure, `memory.current/peak/events/stat`,
  anon/file/kernel/sock/slab, swap, and PIDs;
- controlled-interface bytes, drops, and errors;
- OS limits and process/cgroup identity throughout the run.

Runtime self-observation is diagnostic and uses `runtime/metrics`; it cannot
replace external accounting. `pprof` and execution traces are created only by a
local administrative action into an evidence directory outside the repository.
They are never served on a public listener. Core dumps are disabled in the
standard fixture.

Metric dimensions are fixed and low-cardinality. No label contains peer/source
IP, Node key, Service Name, Service Target, Entry membership, Route, Isolation
Context, attacker-provided string, or Application Data. Detailed profiles are
sensitive evidence with explicit retention, not telemetry.

Each E/S1/S2 local collector must remain at or below `0.01` core mean and
`16 MiB` RSS. The H central collector remains within its separate `0.5` core and
`512 MiB` cgroup. Exceeding either boundary invalidates the resource run rather
than charging observer cost to a candidate.

## H3-A deterministic calculation rules

These rules apply to every R-027/R-028 latency, CPU, memory, GC, traffic,
success, and soak verdict; H3-A does not inherit an unstated Route Qualification
calculator:

- cell/campaign boundaries are precommitted integer-nanosecond offsets from the
  run-start barrier; every collector records its local monotonic anchor at that
  barrier. For each uninterrupted `N`-second active segment, it captures one
  counter baseline at the start and schedules ticks at `start + k seconds` for
  `k=1..N`. Absolute tick error must be `<=50 ms` and each adjacent actual
  interval must be in `[950 ms,1050 ms]`. Counter sample `k` is the non-negative
  delta divided by its exact actual elapsed nanoseconds; a gauge sample is the
  value at that tick. Probe windows split segments and no delta crosses an
  excluded gap;
- a missing, duplicate, late, identity-mismatched, reset, wrapped, or
  non-monotonic required sample invalidates the applicable cell/campaign. A
  predeclared candidate-death fault ends only the populations explicitly replaced
  by that fault oracle; no absent normal value is imputed as zero;
- p50, p95, and p99 use nearest-rank without interpolation: sort `N>0` values
  ascending and select one-based rank `ceil(p*N)`. Duration observations are
  integer nanoseconds; a failed/incomplete offered unit is positive infinity for
  latency and remains unsuccessful in the exact offered denominator. An empty
  applicable population is invalid, never zero;
- a histogram percentile selects the upper bound of the first bucket whose
  cumulative **cell delta** reaches the same nearest rank. Selection of an
  infinite bucket fails the finite-latency gate. Runtime duration values are
  conservatively rounded upward to integer nanoseconds before comparison;
- mean CPU cores equal total external CPU-time delta divided by exact included
  wall time. Success ratios, CPU means, allocation-per-unit/byte, and every
  decimal threshold are compared as integer rationals by cross multiplication;
  no binary floating-point rounding changes a verdict. Memory/byte/count values
  remain integers, and kernel PSI hundredths are parsed as fixed-point integers;
- all manifest-offered workload units define success populations. The ICMP RTT
  percentiles use successful replies only, while all `1000` sends remain in the
  separate loss denominator. The authoritative campaign traffic population is
  the receiver-classifier formula above;
- each soak hour is a half-open hour from its manifest start. Its workload
  population excludes exactly `[55:00,57:35)` and therefore contains `3445`
  active one-second intervals; the whole-campaign workload population excludes
  the same window in every completed hour. Probe/forced-GC values remain in their
  own declared populations and in evidence.

The retained manifest binds the calculator version and exact input-record schema.
The terminal machine result is invalid if two conforming calculations over the
same canonical records differ.

## Evidence plan

### Primary sources

Accessed 2026-08-11:

- [Go garbage collector guide](https://go.dev/doc/gc-guide);
- [Go 1.26 release notes](https://go.dev/doc/go1.26);
- [container-aware GOMAXPROCS](https://go.dev/blog/container-aware-gomaxprocs);
- [Go cgroup runtime source](https://go.dev/src/runtime/cgroup_linux.go);
- [`runtime/debug`](https://pkg.go.dev/runtime/debug);
- [`runtime/metrics`](https://pkg.go.dev/runtime/metrics);
- [Go diagnostics](https://go.dev/doc/diagnostics);
- [`runtime/pprof`](https://pkg.go.dev/runtime/pprof) and
  [`runtime/trace`](https://pkg.go.dev/runtime/trace);
- [Linux cgroup v2](https://www.kernel.org/doc/html/latest/admin-guide/cgroup-v2.html);
- [`tc-netem(8)` network-emulation contract and TCP placement limitation](https://man7.org/linux/man-pages/man8/tc-netem.8.html);
- [`getrlimit(2)`](https://man7.org/linux/man-pages/man2/getrlimit.2.html);
- [Tor memory exhaustion controls](https://spec.torproject.org/dos-spec/memory-exhaustion.html);
- [go-libp2p Resource Manager](https://pkg.go.dev/github.com/libp2p/go-libp2p/p2p/host/resource-manager);
- [`golang.org/x/time/rate`](https://pkg.go.dev/golang.org/x/time/rate) and
  [`golang.org/x/sync/semaphore`](https://pkg.go.dev/golang.org/x/sync/semaphore).

### Exact H3-A workload model

Percentages below scale a frozen offered workload, not a connection limit or a
claim about future Node capacity. Before implementation comparisons, the harness
creates and content-addresses one immutable valid corpus of exactly `192`
sequential experimental Epoch envelopes. Every object has `64` synthetic
candidate-fixture records and exact recorded envelope bytes `E[i] <= 32 KiB`;
all normal object sizes are within `1%` of the corpus median. The general parser
and fault corpus still exercise the `1 MiB` hard cap. The same valid corpus is
used for every candidate. It covers packaged genesis, all short-cell resets, and
at least `168` hourly direct successors without generating online state. The
`72 h` and `7 day` soaks are independent campaigns: each starts from a freshly
provisioned virgin state root and the packaged genesis, so the corpus is reused
as immutable input rather than by rewinding a surviving state directory. The
second campaign starts at most `7 days` after the first ends. Corpus-creation and
per-campaign preflight enumerate every scheduled activation timestamp and prove
it is within that exact envelope's signed
`[valid_after, fresh_until)` interval and before `valid_until`, through campaign
completion. Missing the window requires a newly signed corpus/candidate identity
and complete `72 h` plus `7 day` rerun; the harness never offsets signed Unix
seconds. Normal cells contain zero invalid responses;
malformed, conflicting, slow, and unavailable responses appear only in named
fault cells.

One **Endpoint refresh unit** is useful work only when all of these complete:

1. both installed source-only distributors authenticate and return the same
   exact `E[i]` in one two-request wave;
2. both responses are fully read and independently pass the exact-byte pipeline;
3. the first unit of a short cell durably activates the next corpus Epoch while
   later units classify the same current digest without lowering state;
4. every committed state transition emits the exact bounded H3-A readiness event
   from R-027 and every positive event receives a matching H verification record;
5. the terminal state, useful-work count, bytes, and deadlines are recorded.

The offered Endpoint refresh cadence is fixed:

| Load | Start-to-start cadence | Offered refresh units in 10 min |
|---:|---:|---:|
| `25%` | `120 s` | `5` |
| `50%` | `60 s` | `10` |
| `75%` | `40 s` | `15` |
| `100%` | `30 s` | `20` |
| `125%` | `24 s` | `25` |

If a cadence tick arrives while the two acquisition slots are still owned, the
new unit receives an explicit capacity result within `100 ms`; it remains in
the offered denominator and does not create a third request. The chosen
`30 min` cell therefore offers exactly `60` Endpoint refresh units at `100%`. In
the soak, refresh cadence remains `30 s`, the next pre-generated Epoch is
activated once per hour, and the offline signer is never needed during the run.

One **Distributor response unit** is one independently scheduled mutual-TLS
connection requesting and digest-verifying one exact current `E[i]` within the
source protocol's `5 s` total deadline. Offered work is open-loop: completion or
slowdown never schedules the next unit.

L1 on H generates only S1 units and L2 only S2 units under the installed pins in
R-027; E retains its separate Endpoint identity. TLS client session caches and
server session tickets are disabled, so every unit includes the same full TLS
1.3 mutual-authentication work. A resumption or key-sharing success invalidates
the cell rather than silently changing CPU/allocation cost.

The work unit is a completed response, not a guessed byte-capacity promise. For
target response rate `R`, the manifest precomputes starts `t[k]` at interval
`1/R` seconds, rounded only to integer nanoseconds, and enumerates every timestamp
and object digest before the run. The exact scheduled payload bitrate is derived
from those object lengths and retained as a result/input check; it never changes
the start schedule.

| Load | Offered responses/s per distributor | Offered units in 10 min |
|---:|---:|---:|
| `25%` | `2.5` | `1500` |
| `50%` | `5` | `3000` |
| `75%` | `7.5` | `4500` |
| `100%` | `10` | `6000` |
| `125%` | `12.5` | `7500` |

For at least 99% of starts, launch-time error is `<= 10 ms`; no start may be more
than `50 ms` late. A late/unlaunched unit remains a harness failure in the
offered denominator. The shaper is only a safety ceiling; the timestamp list,
not completed responses or transmitted bytes, defines offered load. The
receiver-side IFB profile above is the only link shaper. Corpus preflight must
show that the exact `125%` scheduled payload bitrate is at most `4 Mbit/s` per
distributor, leaving ample room for Endpoint refresh units, TLS/TCP overhead, and
retransmission below the `100 Mbit/s` controlled link; otherwise the corpus/run
is invalid rather than silently rate-limited.

The normal combined topology cell runs the Endpoint cadence and both distributor
loads simultaneously. Workload clients and the impairment controller run on H
outside candidate cgroups; the verified IFB/qdisc itself runs at each receiver
as fixed above. Before each cell, a paired H-side source/sink must sustain at least
twice the cell's aggregate exact scheduled payload bitrate with zero integrity
error; during the
cell generator CPU p95 is `<= 1.6` cores, memory p95 `<= 1536 MiB`, and H link
unexpected drop/error counters are zero (programmed qdisc loss is separate), or
the cell is invalid. A distinct simultaneous `40`
complete-request cell crosses the distributor's `32` post-auth cap, and a
distinct `17`th incomplete client crosses the pre-authentication cap. Excess
must receive a bounded explicit result and never changes the normal workload.

Every cell freezes the corpus range, exact byte sizes/digests, cadence, client
count, rate, valid/invalid mix, readiness-event schema/expected status, and expected useful-work count
before execution. A percentage without those manifest fields is invalid
evidence.

### Required H3-A cells

1. positive and negative cgroup/limit preflight, including a tighter parent;
2. declared open-loop work ramp at `25%`, `50%`, `75%`, `100%`, and `125%` offered load,
   ten minutes each, plus a `30 min` chosen-capacity run;
3. slow source/read/write, oversized response, stalled handshake, and cancelled
   operation at every stage;
4. distributor incomplete-handshake saturation within its `16` cap;
5. response/queue byte hoarding without useful progress;
6. repeated source/readiness-event churn sufficient to expose goroutine, FD, timer,
   cache, and live-heap leaks;
7. lowered `RLIMIT_NOFILE`, deliberately forced `EMFILE`, and separate internal
   FD-cap exhaustion;
8. allocation spike and forced near-`GOMEMLIMIT` pressure without changing the
   candidate limits;
9. tighter parent cgroup, runtime limit mutation, and process cgroup migration;
10. exactly `20` shutdown/restart cycles in each of idle, acquisition,
    validation, readiness-event, and PROTECT: within each state `10` process-service
    restarts and `10` full-VM restarts, `100` total;
11. one `72 h` H3-S soak and one independent `7 day` unattended campaign before
    H3-B promotion. Both use the same candidate binary, resource/protocol
    contract, genesis, and corpus, but distinct manifests, virgin state roots,
    evidence roots, and machine verdicts; any candidate/contract change restarts
    the sequence at `72 h`;
12. diagnostics disabled/enabled comparison and the collector-overhead gate.

### Fault-cell oracles

General normal acceptance is not applied blindly to an intentionally injected
fault:

| Cell | Predeclared passing terminal behavior |
|---|---|
| Normal and 100% combined | No `EMFILE`, cgroup mutation, OOM event, DRAIN, or hidden rejection; every normal gate passes. |
| Projected request/queue/timer reservation above cap while actual use is below emergency thresholds | Reject before allocation within the caller deadline, preserve established work, then return to NORMAL after all low watermarks hold. |
| Post-auth `32` request cap / incomplete `16` cap / unavailable rejection reserve | Post-auth excess with the reserved rejection socket gets `BUSY <= 250 ms` from complete authenticated frame; if that reserve is unavailable, or before authentication, the connection may reset or time out by the `5 s` client deadline. All are bounded refusal, never success. |
| Actual FD/goroutine/timer `>=90%`, full critical queue, or OS-thread count `>=256` | DRAIN and exit by `60 s`; this emergency cell is not a recoverable-cap test. |
| Forced `EMFILE` | The injector proves at least one deliberate `EMFILE`; candidate returns a bounded local-resource/protection result without spin or false success and exits/restarts cleanly. The expected injected error is not a normal-cell violation. |
| Tighter parent or cgroup migration | Within one collector sample the harness marks the run invalid, cannot report readiness success, and requests bounded drain. A candidate-visible leaf/profile change must also remove experimental readiness. No daemon privilege expansion is required. |
| Recoverable allocation/socket/PSI PROTECT cell | Established work survives and the same process returns to NORMAL only after every low watermark holds for `120 s`; DRAIN is protective but **fails this recovery cell**. |
| Declared emergency pressure or critical-accounting failure | DRAIN and exit by `60 s` is the expected protective result; it is not counted as capacity recovery. |
| Ordinary verification-capsule slots/bytes exhausted while terminal reserve is intact | Commit and externally verify exactly one `not_ready/evidence_failure` capsule through the terminal slot, accept no new work, and drain; no unacknowledged capsule is removed. |
| Terminal capsule cannot be durably preserved or emitted | Close admission/listeners immediately and satisfy the `2 s` emergency fail-stop plus `500 ms` liveness expiry; this protective result passes only the named evidence-failure cell. |
| Invalid/malicious Epoch or source | Exact R-027 typed rejection with no state advance; resource protection cannot convert it to availability success. |

H3-C later adds high-rate handshake flood, multi-peer DRR fairness, Carrier
Channel/stream hoarding, simultaneous route failures, and role useful-capacity
tests. They are not smuggled into H3-A merely because the resource hierarchy can
represent them.

### Acceptance criteria

At the exact declared normal and combined `100%` H3-A workloads, all must hold
together for each applicable candidate cgroup:

- correct R-027 security and independently recomputed readiness verdict with no
  disabled protection;
- Endpoint successful refresh units are `>= 99%` of offered units, and each
  distributor's complete, digest-valid responses within `5 s` are separately
  `>= 99%` of its precomputed offered units. Every failure remains in its
  denominator;
- all generator-validity and launch-timing gates pass;
- mean CPU `<= 1.12` cores and p95 one-second CPU `<= 1.28`;
- p95 `memory.current <= 896 MiB` and zero delta in
  `memory.events.local high/max/oom/oom_kill`;
- every working FD, socket, goroutine, timer, byte, and queue cap respected;
- no unexpected `EMFILE`, panic, deadlock, false success, unbounded spill, or hidden
  process outside the charged tree;
- cgroup CPU throttling is
  `100 * delta(nr_throttled) / delta(nr_periods) <= 1%` normal and `<= 5%` in
  the declared overload cell. A zero period delta invalidates the cell; both
  counter snapshots, elapsed window, and `delta(throttled_usec)` are retained;
- separate GC CPU, GC-pause, scheduler-latency, live-heap, and limiter gates pass.
  Zero pause samples are N/A/pass in a completed normal workload; the dedicated
  GC-pressure cell must produce at least `100` pause observations and pass its
  overload thresholds;
- each over-cap request receives the reserve-dependent `BUSY` or reset/timeout
  terminal class fixed above by its exact deadline;
- established validation/control progress remains within its declared deadline;
- after a recoverable offered-load drop, the same process reaches and holds every
  resource-specific low watermark for `120 s`; DRAIN fails that recovery cell;
- after comparable churn, open FD delta `<= 16`, goroutine delta `<= 32`, timer
  delta `<= 8`, live-heap delta `<= max(5%, 8 MiB)`, process-tree anonymous RSS
  and cgroup-anon deltas each `<= max(5%, 16 MiB)`, socket handles delta `<=4`,
  socket memory delta `<=max(10%, 8 MiB)`, and slab delta
  `<=max(10%, 8 MiB)` of baseline; total `memory.current` remains under its
  absolute gate and file cache under `160 MiB`;
- every graceful candidate shutdown exits by `60 s` with no candidate-owned
  process, socket, mount, or state temp file left behind; the harness then stops
  and verifies its independently owned collector/workload trees and the exact
  impairment teardown above;
- `dropped_verdict_events == 0`, every verification capsule was externally
  spooled, digest-matched, and verified before local removal, the terminal
  capsule/ordinary-slot rules were respected, local capsule backlog is zero
  after collector acknowledgement, and every retained workload/transition count
  agrees;
- evidence recomputes the same conjunctive R-027 `pass`/`fail`/`invalid`
  machine result.

The `125%` cell may remain NORMAL if it completes every offered unit within all
normal gates. If a cap is reached, excess follows the bounded refusal contract;
PROTECT is required only in the dedicated cells whose inputs cross a stated
pressure threshold.

## Dependency decision

H3-A should begin with the standard library and owned bounded counters/channels.
`x/time/rate` is a mature token-bucket candidate and `x/sync/semaphore` a
weighted-concurrency candidate, but neither supplies hierarchical ownership,
fair queues, overload states, or evidence. Add one only after the normal
dependency review records the exact safety advantage and version.

go-libp2p Resource Manager is a useful reference for hierarchical scopes and a
transient pre-authentication scope. It is not selected: it is coupled to libp2p,
does not implement Ardents fairness/readiness, and would make a transport stack a
resource-management dependency before Gate D.

Prometheus/OpenTelemetry are not required for H3-A. A local fixed-schema
collector avoids remote listeners, high-cardinality labels, and a new telemetry
dependency. If later selected, their privacy and resource cost require their own
review.

Ubuntu `iproute2/tc` is a pre-provisioned lab-fixture tool, not a Go runtime
dependency or product transport choice. Its package version, package/source
digest, kernel compatibility, invocation, and removal evidence are frozen in the
manifest. If the approved host image cannot supply the declared netem/IFB
contract, the run stops; H3-A does not add an installer or silently substitute a
different impairment stack.

## Narrow points and risks

| Risk | Required response |
|---|---|
| `GOMEMLIMIT` is mistaken for RSS/host safety. | Keep cgroup high/max, socket/kernel accounting, and headroom authoritative. |
| Go sees only a leaf CPU limit or misses migration. | Harness inspects full ancestry, candidate validates only its accessible profile, GOMAXPROCS stays fixed, and any changed placement invalidates the run. |
| More goroutines are treated as scalable concurrency. | Token before goroutine, bounded workers, scheduler/GC evidence, and useful-work measurement. |
| OS fuse is used as normal admission. | Internal caps are far below RLIMIT/cgroup fuses; crossing a fuse is failure. |
| Control priority becomes an unbounded privileged queue. | Small finite reserve; full critical queue drains explicitly. |
| Pressure evicts arbitrary established work. | Protect established/control work, reject new work, then bounded drain; no random eviction. |
| GC tuning hides a live-set leak. | Stable profile, live-heap/object evidence, churn baseline, no hot-path forced GC. |
| Resource metric labels leak the network graph. | Fixed aggregate dimensions; sensitive profiles stay local evidence. |
| H3-S becomes a Node product capacity promise. | State that it is a process-tree containment profile; H3-A work units are lab-only and H3-C must define product role work units. |
| Large hosts silently gain authority or lower diversity. | Scale useful work only under a new qualified profile; trust and role remain unchanged. |

## Falsification criteria

Stop and redesign if any is true:

- accepted work allocates memory, opens a socket/FD, starts a goroutine/timer, or
  enters a queue without a parent reservation;
- a child budget can multiply its parent or resource release is not exactly
  owned;
- normal H3-A work needs more than the H3-S envelope without a measured reason;
- overload causes OOM, unexpected `EMFILE`, panic, deadlock, hidden spill, false success,
  random eviction, or an unbounded retry/log/metric loop;
- safety/readiness/security work is disabled to meet performance;
- established validation/control work loses its declared progress while optional
  work continues;
- GC limiter/thrashing or growing live heap persists at the valid workload;
- a candidate-visible cgroup/profile change leaves experimental readiness true,
  or the harness fails to invalidate a changed inaccessible ancestor;
- diagnostics exposes protected identifiers or costs more than its budget;
- shutdown exceeds `60 s` or leaves owned resources;
- the result depends on copying endpoint capacity numbers to infrastructure;
- the implementation requires libp2p, Prometheus/OTel, Kubernetes, or another
  broad dependency merely to enforce the H3-A limits;
- evidence is self-reported only or cannot reproduce the verdict.

## Recommendation

Choose H1 with high confidence for H3-A: one fixed H3-S profile, hierarchical
reservation before work, cgroup/OS hard boundaries, explicit Go runtime values,
the `NORMAL/PROTECT/DRAIN` policy plus terminal `EXIT`, and external evidence.
Adaptation may only reduce new
load; it must never raise limits automatically.

The strongest counterargument is that fixed limits may underuse a powerful host.
The scale-up contract answers that without making unmeasured auto-tuning part of
the first slice: a larger profile is added only after H3-B/C provide a useful
work unit and prove at least `4x` work with `20%` reserve.

## Disposition

- Question state: `decided` as the resource/evidence appendix to accepted R-029.
- Applies to the R-029 Stage 1 Endpoint/source work where the Stage 1 brief names
  it; H3-NP1 remains separately defined by R-029.
- Creates no Node capacity floor, public SLO, dependency selection, or ADR.
- Code and experiment: none created by this research record.
