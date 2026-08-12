# Horizon 3 Stage 1 implementation brief

Status: **authorized by accepted R-029; prerequisites remain blocking**

Audience: implementation agent working one increment at a time.

Authoritative inputs:

- [R-029 integrated decision](../research/records/r-029-h3-authenticated-node-lifecycle.md);
- [R-027 bootstrap appendix](../research/records/r-027-h3-first-slice.md);
- [R-028 resource/evidence appendix](../research/records/r-028-h3-runtime-resource-contract.md);
- [complete H3 technical design](horizon-3-technical-design.md);
- [repository layout](repository-layout.md);
- [package map](package-map.md);
- [dependency register](dependencies.md).

## Objective

Implement one isolated product-shaped tracer:

> An accepted Network Epoch commits a bounded canonical Candidate View; a
> separately keyed Node process consumes its own proven record and assignment,
> becomes ready, performs bounded role-probe work, refreshes, drains, withdraws,
> restarts, and leaves no overlapping or leaked duty.

The implementation is complete only when authenticated state controls the real
Node process. A parser, state downloader, readiness event, or synthetic fixture
alone is not Stage 1.

## S1-0 blocking prerequisites

Do not write feature code until all are true:

1. R-029 remains `decided` and R-027/R-028 remain adopted only as the referenced
   bootstrap and resource/evidence appendices;
2. the old standalone H3-A brief remains withdrawn and the research queue and
   H3 slice gate continue to point to R-029;
3. the repository-approved patched Go 1.26.5 toolchain is active;
4. `make quick-check` and `make check` pass before the first increment;
5. unrelated workspace changes are committed or explicitly separated from the
   implementation change.

If any prerequisite is absent, report the exact blocker. Do not create a
placeholder package, fake Node, or in-process-only proof.

The multi-host fixture is **not** an S1-0 prerequisite. S1-0 is deliberately
offline and must proceed without SSH aliases, remote inventory, pre-created
evidence roots, or four running hosts. `cmd/ardents-qualify` creates its bounded
local evidence root under an explicitly configured temporary directory outside
Git.

Environment prerequisites are staged:

- S1-0 uses local files, processes, golden vectors, and independent offline
  verification only;
- S1-1 and S1-2 must first work as local black-box multi-process development
  runs; loopback, Docker, or a local Linux VM may support development but make no
  qualifying claim;
- S1-3 requires a Linux cgroup-v2 environment only for the applicable resource
  cells; platform-independent implementation and tests continue while it is
  unavailable;
- only S1-4 official qualification requires one preflighted dedicated Ubuntu
  Docker Engine host carrying the exact isolated E/S1/S2/N1/N2 logical zones,
  fixed candidate budgets, host-owned collectors/management, and external
  evidence partitions.

Missing remote infrastructure blocks only the evidence that actually needs it.
It never blocks earlier implementation, unit/fuzz tests, local black-box tests,
manifest generation, verifier work, or preparation of reproducible provisioning
instructions.

## Fixed scope

### Included

- R-027 Network Epoch envelope, threshold verification, finite source plan,
  retry/backoff saturation, Time Confidence, conflict/fork handling, crash-safe
  generations, source exposure history, and fail-stop persistence behavior;
- R-029 canonical Candidate View commitment, finite input log, deterministic
  rejection set, global summaries, indexed Candidate Materialization, and Role
  Domain assignment;
- one Endpoint state consumer on E;
- separately keyed Node processes N1 and N2 on S1/S2 in distinct OS/cgroup/
  namespace/state zones from the source processes;
- one bounded TLS role-probe Adapter used only to prove process work and drain;
- H3-S resource accounting for E/S1/S2 source work and H3-NP1 for N1/N2;
- external evidence, faults, cleanup, two-hour churn, and one independent
  twenty-four-hour unattended run.

### Excluded

- Route construction, relay forwarding, Introduction, Rendezvous, Service
  publication, Name/Target resolution, Application Interface, Bridge, update,
  installer, Windows, public admission, permissionless publication, DHT, peer
  discovery, production wire format, production deployment, and SDK;
- anonymity, decentralization, operator independence, censorship resistance,
  public availability, and public Node capacity claims;
- any import, copy, extension, or hidden invocation of H2 laboratory protocol or
  lifecycle code.

## Maintained Module map

Create a path only in the increment that supplies `doc.go`, real behavior,
behavior tests, a non-test caller, and the package-map row.

| Path | Module responsibility | Permitted project imports |
|---|---|---|
| `internal/network/{state,epoch,source,store}` | Orchestrate Network State while keeping Epoch semantics, Direct-Origin Source, and durable storage under distinct owners | `internal/resource`, standard library |
| `internal/node` | Run one local Node identity through assignment, readiness, duty, drain, withdrawal, and terminal cleanup | `internal/node/probe`, `internal/resource`, standard library |
| `internal/node/probe` | Own bounded authenticated role-probe transport and accepted work | standard library |
| `internal/resource` | Own common OS/runtime measurement, placement, hysteresis, and `NORMAL`/`PROTECT`/`DRAIN`; State and Node own their reactions | standard library |
| `internal/qualification/{state,node}` | Own independent State verification and black-box Node campaign evidence | Qualification submodules and standard library only; no product verifier |
| `cmd/ardents` | Run the Endpoint Network State consumer and expose bounded local status | `internal/network/state`, `internal/network/source`, `internal/planfile`, standard library |
| `cmd/ardents-node` | Run one separately configured Node identity and one active role per process | Network, Node, Probe, and planfile Modules; standard library |
| `cmd/ardents-qualify` | Start controlled qualification work and render its terminal result | Qualification Modules; standard library |

The S1-3/S1-4 human-authored Dockerfile, Compose topology, and non-secret fixture
inputs belong in `tests/qualification/h3-node-v1/` when they gain a real caller.
Generated images, keys, state, manifests, captures, and evidence stay outside
Git. Do not place H3 qualification assets under `lab/` or `deployments/`.

The product import direction is:

```text
cmd/ardents -> internal/network/state, internal/network/source
cmd/ardents-node -> internal/network/state, internal/node
cmd/ardents-qualify -> internal/qualification/state, internal/qualification/node
internal/network/state -> internal/network/epoch, internal/network/epoch/assignment, internal/network/epoch/merkle, internal/network/framing, internal/network/source, internal/network/store, internal/resource
internal/network/epoch -> internal/network/epoch/assignment, internal/network/epoch/merkle, internal/network/framing
internal/node -> internal/node/probe, internal/resource
internal/resource -> standard library
internal/qualification/state -> internal/qualification/byteio
internal/qualification/node -> internal/qualification/byteio, internal/qualification/node/fixture
internal/qualification/node/fixture -> internal/network/epoch/assignment, internal/qualification/byteio, internal/qualification/epochfixture
internal/qualification/epochfixture -> internal/network/epoch/assignment, internal/network/epoch/merkle
internal/qualification/byteio -> standard library
```

Product Modules and product runtime commands never import qualification or
`internal/lab`. Qualification drives product executables as black boxes and
must independently recompute security decisions rather than importing their
validator. `internal/resource` remains the shared deep Module proven by its two
production consumers; it owns only measurement, placement, hysteresis, and
pressure decisions, while State and Node own their reactions. It must not know
Epoch, Source, Route, Node duty, or qualification. Do not create crypto,
transport, persistence, schema, errors, types, interfaces, adapters, common,
util, API, or SDK packages.

When `internal/qualification` and `cmd/ardents-qualify` first appear, extend the
architecture gate in the same increment so it enforces their non-product status,
standard-library-only package imports, black-box direction, and the prohibition
on product imports of qualification. Do not weaken the existing laboratory
quarantine to achieve this.

### Expected product Interfaces

Keep exact Go surfaces minimal and review them in the first increment. Their
behavior must provide:

- Network State: open one owned state root; refresh through an explicitly
  supplied distribution Adapter; return one immutable current Snapshot with
  verification/freshness/exposure status; close all owned work;
- Node Lifecycle: run one configured local Node against immutable Snapshot
  updates and one bounded role implementation; emit readiness/lifecycle events;
  return one terminal result after cleanup.

Callers must not assemble validation pipelines, mutate Snapshots, select source
order, calculate assignments, or coordinate drain steps themselves. Those are
Module Implementation details. Tests cross the same Interfaces as callers.

## Source and state behavior

Implement the R-027 acceptance order as one side-effect-free decision followed
by one separately tested durable commit. Replace only the synthetic fixture with
R-029 `view_commitment` semantics.

Required behavior:

1. reject size/framing/schema violations before expensive verification;
2. verify network, protocol/profile, digest, signer policy, threshold, epoch
   chain, transition, and strict freshness;
3. verify View and rejection roots, input cutoff, lengths, summaries, and
   assignment algorithm identity;
4. verify exact Node Record signatures, validity, uniqueness, accepted/rejected
   outcomes, canonical order, and Merkle roots;
5. verify requested Candidate Materialization indices and proofs locally;
6. persist immutable generation files, fsync them and their directory, then
   atomically replace and fsync the current pointer;
7. preserve rollback, signer, epoch, time, conflict, and direct-source-exposure
   floors across restart;
8. publish readiness only as the conjunction of verified persistent state,
   current live process observation, current clock/resource placement, and no
   outstanding integrity failure;
9. on a failure that prevents a negative durable event, remove readiness and
   exit within the R-027 fail-stop bound;
10. use finite source waves, deadlines, retry state, and saturating backoff;
    never select the first or fastest valid object and never silently resample an
    unavailable materialization index.

The first increment freezes canonical record/envelope/proof/event/manifest bytes
as reviewed golden vectors before network and process behavior is added. No
generated golden file is accepted without an independent parser/verifier test.

## Candidate View behavior

Use R-029's exact commitment fields and Merkle domains. The controlled corpus
contains positive, rejected, duplicate, malformed, collision, cutoff, and
assignment-transition records, never more than 64 inputs.

H independently recomputes:

- ordered input-log root;
- deterministic accepted/rejected decision for every input;
- canonical accepted View root and length;
- rejected root and length;
- domain and declared-family count/capacity/concentration summaries;
- every materialization proof and requested index;
- every family-to-domain assignment.

Candidate self-report cannot supply a missing input or summary. The manifest
records that all declared families remain under one real project control family.

## Node lifecycle behavior

The process implements exactly:

```text
ABSENT -> PREPARED -> READY -> DRAINING -> WITHDRAWN
                     |           |
                     +-> FAILED <-+
```

Admission requires all of:

- current verified Epoch/View/profile;
- locally owned Node Identity and key match the accepted Node Record;
- record and deterministic assignment are active;
- maximum role-probe duty lifetime fits before every terminal bound;
- no old assignment duty or quarantine remains;
- cgroup/host ancestry and H3-NP1 resources match the manifest;
- persistent state and external-evidence channel are healthy.

In READY, the Node admits no more than H3-NP1 work. On withdrawal, new work is
refused within one second. Existing probes finish or cancel within 15 seconds.
The listener and ephemeral resources close, readiness becomes negative, and the
role reaches WITHDRAWN or FAILED. Restart re-evaluates state and never resumes a
live duty. Reassignment cannot make the new domain ready until old work is
terminal and quarantine passes.

Node and source processes on the same host have separate users, roots, private
keys, cgroups, namespaces, listeners, configuration, and evidence. Tests prove
neither process can read the other's mounted secret/state paths or bind the
other's listener. Root compromise and common operator control remain non-claims.

## Role-probe Adapter

The tracer uses standard-library TLS 1.3 over static TCP with installed roots and
leaf pins. It has no DNS, discovery, resumption, early data, proxy, relay, QUIC,
HTTP, WebSocket, libp2p, Waku, or fallback.

Each request and response binds:

- protocol/profile version;
- network identity and Epoch digest;
- Node Identity and assignment digest;
- fresh harness nonce;
- bounded payload length and terminal deadline.

The probe proves only that the assigned process performs bounded authenticated
work and drains it. Keep the current implementation private to Node Lifecycle;
no probe wire type appears in either product Module Interface. Qualification
supplies only the black-box client, faults, observations, and independent
verdict.

## Resource contract

Apply R-028 hierarchy, reserve-before-allocate rule, cgroup/OS external sampling,
one-second raw evidence, and `NORMAL -> PROTECT -> DRAIN -> EXIT` behavior.

- E/S source candidate processes use the accepted H3-S values from R-028.
- N1/N2 use R-029 H3-NP1 exactly: one CPU, 512 MiB cgroup memory, 320 MiB
  `GOMEMLIMIT`, `GOMAXPROCS=1`, `GOGC=100`, 512 goroutines, 512 FDs, 256
  sockets/PIDs as separately specified, 512 timers, 8 MiB mutable queues, 16
  open/4 active probe sessions, and 16 MiB candidate state/evidence.
- S1/S2 hosts have 4 vCPU/4 GiB with candidate cgroups and explicit host reserve.
- H collectors/harness remain outside candidate accounting and have their own
  manifest budgets.

Every goroutine, process, FD, socket, timer, queue byte, persistent byte, retry,
and evidence event has one owner and finite parent. A pressure transition never
weakens verification, source exposure, assignment, TLS, or cleanup behavior.

## Mandatory increments

Implement and review sequentially. The same agent may continue through S1-0 to
S1-4 in one task. These rows are progress and evidence checkpoints, not separate
prompts or automatic stopping points. After each row, report the observable
result and checks in commentary, retain a scoped diff/evidence snapshot, and
continue when the row is green and no stop condition or Product Owner decision
is pending. Do not start a later row while an earlier row has unresolved
findings.

| Increment | Observable result | Review boundary |
|---|---|---|
| S1-0 contract and offline state | Create Network State, Epoch, Store, `ardents`, State qualification, and `ardents-qualify` with frozen golden artifacts, offline genesis/View acceptance, proof verification, durable generation, and a separate persisted-state verdict | No network, Node process, listener, or readiness claim |
| S1-1 finite state distribution | Add `ardents-node` source mode; E obtains identical state/materialization through S1/S2 under exact R-027 retry, conflict, exposure, clock, and restart behavior | No assigned Node lifecycle claim |
| S1-2 real Node lifecycle | Add the Node and Probe Modules; N1/N2 consume the same state, perform role-probe work, refresh, drain, withdraw, restart, and reassign without overlap | Functional only; no resource qualification |
| S1-3 hostile resource matrix | External cgroup/process accounting, overload, faults, evidence, fail-stop, cleanup, and quiescence pass | No soak or advance result |
| S1-4 official Stage 1 campaign | Complete short matrix, independent 2 h churn campaign, and independent 24 h unattended campaign produce machine-verifiable evidence roots and verdicts | Product Owner records advance/redesign/stop |

Current closure note (2026-08-12): S1-0, S1-1, and S1-2 are re-proven by the
maintained regression and black-box process suites. S1-3 now has a fail-closed
Docker harness with a pre-behavior sealed campaign contract, immutable fixture
and source/image/binary identity binding, candidate runtime diagnostics retained
without verdict trust, external cgroup-v2/process/behavior gates, deterministic two-hour resource churn,
the hostile fault matrix, cleanup, and quiescence gates. S1-4 remains open until `short`, `churn-2h`, and
`unattended-24h` each produce a valid evidence root on the qualifying Ubuntu
host. Local Docker results are development evidence only and cannot change that
status or authorize Stage 2.

Recorded development evidence (2026-08-12):

| Increment | Implementation/check result | Campaign evidence and machine result |
|---|---|---|
| S1-0 | Implemented; independent persisted-state verification and frozen vectors pass the Go 1.26.5 full gate | Not a Docker campaign |
| S1-1 | Implemented; finite authenticated two-source and restart/conflict regression suites pass | Not a Docker campaign |
| S1-2 | Implemented; lifecycle, authenticated probe, withdrawal, restart, and black-box process suites pass | Functional development evidence only |
| S1-3 | Campaign implementation and final review findings are closed in code; a fresh full matrix has not passed after the campaign-contract/runtime-evidence changes | Latest full local result predates those changes and remains rejected: `invalid`, digest `1c3bdd291eeb5e724d139234d2495709644b89d4c63c8577f572196d95464aaf`, root `C:\Users\vitek\AppData\Local\Temp\ardents-h3-local-final-31f58549ce2348b1a2de6f4356e57e91\short-evidence` |
| S1-4 | Not complete; no qualifying Ubuntu result | Official `short`, `churn-2h`, and `unattended-24h` were not run; no machine result or evidence root exists |

The local host was Docker Desktop 4.55.0 / Engine 29.1.3 on a WSL2 Linux
6.6.87.2 kernel with cgroup v2. It is explicitly non-qualifying. Earlier local
runs that exposed evidence-cadence and process-transition defects remain
development findings outside Git; an early apparent `pass` is rejected because
its audit found only 194 samples over 667 seconds. The only remaining execution
path is the exact three-root Ubuntu command in
`tests/qualification/h3-node-v1/README.md`. Until all three official results are
valid passes, Stage 1 is incomplete and the recommendation is `redesign`; do
not start Stage 2.

Every increment runs `make quick-check` while changing and `make check` before
integration. Keep commits scoped to one increment. Do not modify frozen H2
laboratory behavior to make Stage 1 pass.

## Required tests

### Network State Module

- table and fuzz tests for every framing, schema, length, digest, signature,
  transition, time, View, record, proof, summary, and collision rule;
- golden accepted/rejected bytes and cross-parser round trips;
- property tests for canonical ordering, Merkle inclusion, root changes, and
  assignment determinism;
- exhaustive crash-point tests for generation/pointer commit and cleanup;
- restart, rollback, conflict, stale clock, fail-stop, finite retry, and
  saturating-counter boundaries;
- same-index withholding and no-resampling proof.

### Node Lifecycle Module

- deterministic state-machine tests for every legal and illegal transition;
- admission conjunct tests removing each prerequisite in turn;
- activation/expiry boundary tests with controlled monotonic time;
- new-work refusal, established-work drain, deadline cancellation, restart, and
  terminal cleanup;
- record removal/revocation, profile change, assignment change, quarantine, and
  old/new domain non-overlap;
- capacity and resource-pressure behavior without weaker security state.

### Cross-process laboratory

- separate PID/user/cgroup/namespace/key/state/listener evidence for source and
  Node roles;
- positive bounded probe workload and tamper/replay/wrong-node/wrong-epoch/
  wrong-assignment/expired-work negatives;
- source and Node death, slow/partial frames, connection flood, `EMFILE`, disk
  full, memory/CPU pressure, cgroup drift, clock uncertainty, evidence failure,
  and harness-invalid cells;
- no undeclared sockets, DNS, route, proxy, shared-memory shortcut, or management
  traffic in candidate namespaces;
- post-run zero owned process/listener/socket/queue/timer/temp-resource residue;
- repeated churn and quiescence without monotonic leak.

## Evidence output

Generated evidence stays outside Git. Retain:

- source/build/toolchain identity and dependency graph;
- redacted and sensitive manifest layers with digest binding;
- exact corpus, golden schema/proof identities, workload, fault, resource, clock,
  topology, cgroup, namespace, and expected-result inputs;
- raw one-second external OS/cgroup/process samples;
- ordered candidate events and external verification records;
- traffic attribution/capture summaries and forbidden-flow verdicts;
- persistent-state hashes, crash/restart results, lifecycle transitions, resource
  high-water marks, cleanup tree, and quiescence samples;
- terminal `pass|fail|invalid` campaign record and Product Owner disposition.

Sensitive topology and key-path mapping is owner-reviewable, not public
independent evidence. Redacted output must not contain private keys, raw
credentials, User/Service/Name/Target data, complete route history, or reusable
source/management access.

## Definition of Done

Stage 1 is done only when:

1. all six approved product/qualification paths exist with the registered import
   direction, real behavior, tests, non-test callers, and no speculative package;
2. accepted state commits and externally verifies a canonical Candidate View and
   real Candidate Materialization, not the R-027 synthetic fixture;
3. at least one separately keyed Node process reaches READY and completes probe
   work solely because its accepted record/assignment permits it;
4. every withdrawal cause removes admission, drains bounded work, prevents
   assignment overlap, and reaches terminal cleanup;
5. stale, conflicting, corrupt, resource-unsafe, or evidence-unsafe state cannot
   leave readiness positive;
6. H3-S and H3-NP1 functional, security, pressure, cleanup, and quiescence gates
   all pass under external observation;
7. the complete short matrix, 2 h churn run, and independent 24 h run have valid
   evidence roots and machine result;
8. `make check` passes with the patched toolchain;
9. the report states the centralized operator/host co-residence and role-probe
   limitations without an anonymity, decentralization, Route, or capacity claim;
10. an implementation review finds no H2 import, hidden parallel state path,
    unbounded resource, silent fallback, or selected future foundation.

## Stop conditions

Stop and return to R-029 rather than improvising if:

- the canonical View or assignment cannot be represented without changing the
  accepted product contract;
- product Modules need H2 lab imports or the probe leaks into their Interfaces;
- a new external dependency, public encoding, transport framework, database,
  orchestration platform, or custom cryptography becomes necessary;
- process/key/state/cgroup/network isolation cannot be proven on the controlled
  Docker fixture;
- resource limits make required security/lifecycle work impossible;
- a failure cannot be classified as candidate `fail` or harness `invalid`;
- the work expands into Route, naming, Service, Bridge, installer, updater,
  Windows, or public governance.

## Agent handoff result

At the end of each increment, report only:

- observable outcome achieved;
- files and package-map rows changed;
- selected dependencies, normally none;
- tests/checks run and exact failures;
- evidence location and machine result when applicable;
- remaining stop condition or recommendation for the next increment.

Do not claim completion from code volume, unit tests alone, process startup, or
self-reported readiness.
