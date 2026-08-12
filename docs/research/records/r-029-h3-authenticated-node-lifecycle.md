---
id: R-029
title: What integrated first H3 slice joins authenticated state to a real Node lifecycle?
status: decided
owner: product research
started: 2026-08-11
reviewed: 2026-08-11
---

# R-029 - Authenticated state and real Node lifecycle

**Accepted decision:** the Product Owner accepted H1 for Horizon 3 Stage 1 on
2026-08-11. R-027 is adopted only as the bootstrap/source/freshness/persistence
appendix and R-028 only as the resource/evidence appendix. This decision
authorizes Stage 1; it does not authorize Stages 2-8 or any public-network claim.

## Decision this unlocks

Choose the first implementation-shaped Horizon 3 slice. It must prove that
authenticated shared state has a real process consumer: a separately keyed Node
is admitted, assigned, made ready, refreshed, drained, withdrawn, restarted, and
resource-bounded by that state. It must not stop at validating a synthetic
bootstrap object or silently select the later Route transport.

This record replaces the recommendation to implement R-027 as a standalone
bootstrap project. R-027 remains the detailed source, freshness, persistence,
and conflict appendix. R-028 remains the resource-accounting and evidence
appendix. Neither appendix is independently implementation authority.

## Current contract

- [Horizon 3 scope](../../product/scope.md#horizon-3--closed-test-network)
  permits one bounded, visibly centralized vertical slice at a time.
- [H3 technical design](../../development/horizon-3-technical-design.md)
  defines Stage 1 as Authenticated State and Real Node Lifecycle.
- [ADR-0004](../../adr/0004-authenticated-epochs-and-separated-control-roots.md)
  and [R-009](r-009-hostile-bootstrap-and-bridge-entry.md) select an expiring,
  threshold-authenticated Network Epoch whose authorization is independent of
  distribution.
- The [operating model](../../product/operating-model.md) and
  [threat model](../../security/threat-model.md) require a logical complete
  Candidate View, deterministic Role Domain assignment, bounded source
  exposure, finite Work Safety Leases, and explicit stale/conflict behavior.
- [ADR-0009](../../adr/0009-go-project-foundation.md) selects Go 1.26.x and a
  standard-library-first maintained implementation.
- [R-027](r-027-h3-first-slice.md) already specifies bounded source selection,
  Epoch verification, freshness, fork/conflict handling, crash-safe generations,
  and externally verified readiness, but deliberately stops at a synthetic
  candidate fixture.
- [R-028](r-028-h3-runtime-resource-contract.md) already specifies hierarchical
  reservations, cgroup-v2 observation, Go runtime controls, overload states,
  cleanup, and machine evidence for Endpoint/source work. It explicitly does
  not define Node capacity.

H3 remains one project-controlled operator family. Different fixture labels,
VMs, keys, or processes do not prove independent operators or decentralization.

## Decision question

Can one small maintained product seam join an accepted Network Epoch and
Candidate Materialization to a real, separate Node process lifecycle while all
Route, Service, Namespace, Bridge, installer, and public protocol decisions stay
replaceable and out of scope?

## Hypotheses

- **H1 - integrated state-to-Node tracer:** one bounded Candidate View and a
  role-probe Node process prove the state, process, lease, withdrawal, resource,
  and evidence seams needed by later Route roles.
- **H2 - bootstrap first:** implementing R-027 alone reduces uncertainty enough
  to justify postponing every real state consumer.
- **H3 - inherit Gate C:** extending the H2 Named Site or native circuit is a
  cheaper path to persistent Node lifecycle.
- **H0:** none can produce a real consumer without prematurely selecting a Route
  or building a parallel temporary architecture.

H1 is recommended. H2 creates a durable subsystem with no product consumer. H3
would mix frozen laboratory semantics into the greenfield product. H0 is chosen
if the interfaces below cannot remain transport-neutral or resource-bounded.

## Evaluation criteria

The accepted candidate must satisfy all of these together:

1. authenticated state controls at least one separately keyed, separately
   running Node process rather than ending at a parser or fixture;
2. the Network State and Node Lifecycle Interfaces remain independent of the
   role-probe transport, Route, Service, Namespace, Bridge, and Application
   Interface;
3. Candidate View construction, rejection, materialization, and assignment are
   deterministic and independently recomputable from a finite frozen corpus;
4. stale, conflicting, corrupt, resource-unsafe, or evidence-unsafe state cannot
   leave Node readiness positive or overlap old/new Role Domain duty;
5. sources, retries, queues, state, work, resources, evidence, drain, and cleanup
   have explicit finite bounds;
6. the implementation uses the accepted Go foundation and standard library
   without a new runtime dependency or custom cryptography;
7. the exact controlled Docker campaign can produce a deterministic `pass`, `fail`, or
   `invalid` result while making no Route, capacity, anonymity, independence, or
   decentralization claim;
8. the complete implementation and evidence path remain maintainable by the
   Product Owner plus Codex team.

## Findings

- **Sourced fact:** ADR-0004 and R-009 require authenticated expiring state whose
  authority is separate from distribution; R-027 supplies the accepted bounded
  source, freshness, conflict, and persistence appendix.
- **Sourced fact:** the accepted product and threat contracts require a logical
  complete Candidate View, same-index materialization retry, deterministic Role
  Domain Assignment, finite Work Safety, and no overlapping old/new duty.
- **Sourced fact:** RFC 9162 specifies the largest-power-of-two recursive Merkle
  tree shape used as the reviewed construction reference. Stage 1 adds its own
  domain-separated lab leaf encodings and makes no public wire-format claim.
- **Sourced fact:** RFC 8446 defines TLS 1.3, resumption, and early data. The
  role-probe and distribution fixtures deliberately disable resumption tickets
  and early data so every bounded connection exercises a full handshake.
- **Assumption:** one dedicated Ubuntu Docker Engine host can provide separate
  cgroup-v2, network, PID/IPC, mount, key, and state zones for E/S1/S2/N1/N2,
  while a host-owned qualifier observes them externally. Failure of that
  preflight blocks the official Stage 1 verdict, not local implementation.
  Evidence roots are created by the qualifier in its configured
  out-of-repository output directory; they are not supplied by the Product
  Owner as an implementation prerequisite.
- **Inference:** bootstrap-only H2 would create maintained state machinery with
  no product consumer, while extending H2 laboratory code would violate the
  product/laboratory dependency boundary. H1 is the smallest candidate that
  tests the reusable state-to-lifecycle seam without selecting the Route.
- **Inference:** the role probe is acceptable only while it remains a private,
  deletable lifecycle implementation detail; leakage into either product
  Interface falsifies the candidate.

## Options

| Option | Product/security fit | Main risk | Disposition |
|---|---|---|---|
| H1: integrated state-to-Node tracer | Proves authenticated state, real process duty, bounded withdrawal, and independent evidence through product Interfaces. | The probe could leak transport assumptions or the one-to-one fixture could exceed its resource/maintenance envelope. | **Accepted.** |
| H2: bootstrap first | Reuses the detailed R-027 mechanics. | Ends without a real state consumer and creates a parallel temporary product architecture. | Superseded as a standalone order; retained only as the R-027 appendix. |
| H3: inherit Gate C | Reuses already working laboratory processes. | Imports frozen H2 architecture and semantics into the greenfield product. | Rejected. |
| H0: choose none | Avoids an unjustified foundation. | Stage 1 cannot proceed. | Select if the accepted Interfaces, resource bounds, fixture, or evidence contract falsify. |

## Exact Stage 1 outcome

On a persistent project-controlled Ubuntu network:

1. E accepts one threshold-authenticated current Network Epoch from an installed
   candidate or the finite R-027 source plan;
2. the Epoch commits a bounded canonical Candidate View rather than R-027's
   synthetic candidate fixture;
3. E verifies one precommitted Candidate Materialization and the external
   verifier recomputes the complete input log, View, rejection set, assignments,
   and summaries;
4. N1 and N2 independently accept the same Epoch, verify their own Node Records,
   and run separate Node processes only while their assignments and Work Safety
   Leases permit;
5. a bounded role-probe workload proves admission, readiness, established work,
   refresh, drain, withdrawal, restart, reassignment quarantine, and cleanup;
6. every result is externally recomputed as `pass`, `fail`, or `invalid`.

The role probe is an H3 lifecycle tracer. It is not a Route hop, discovery
protocol, Application transport, public Node capacity result, or production wire
protocol.

## Controlled topology

Use one dedicated remote Ubuntu host with Docker Engine, cgroup v2, and an
externally owned qualification process. The logical E/S1/S2/N1/N2/H topology is
preserved as isolated process zones, not claimed as separate physical hosts:

| Zone | Process | Qualification limit |
|---|---|---|
| E | Endpoint Network State consumer | No Application, Route, DNS, or direct fallback |
| S1/S2 | two independently keyed source processes | Source identities, state, listeners, and budgets remain distinct |
| N1/N2 | two independently keyed Node processes | Node identity, assignment, state, listener, and budget remain distinct from sources and each other |
| H | host-owned orchestrator, workload clients, collector, and independent full-view verifier | H is management/evidence only, never a candidate data-path proxy |

E/S1/S2/N1/N2 have separate unprivileged containers, users, keys, read-only root
filesystems, state/evidence mounts, cgroups, network namespaces, PID/IPC
namespaces, listeners, and explicit network allowlists. Candidate containers
receive no Docker socket, host network, shared PID/IPC namespace, privileged
mode, or cross-role secret/state volume. H owns container lifecycle and
host-level cgroup/network observation outside candidate containers.

The host may observe or compromise every zone, shares one kernel, scheduler,
clock source, physical NIC, power domain, and operator, and is therefore one
failure/control family. Stage 1 claims no kernel isolation, physical-host
failure recovery, real inter-host latency, operator independence, diversity, or
anonymity from this fixture. Those properties require later distributed Route
Qualification; they are not silently inferred from containers.

The manifest records one real control family, `project-controlled`, for every
process. Any additional declared family values exist only to exercise canonical
assignment arithmetic and are labelled synthetic; they cannot support an
independence, diversity, or anonymity claim.

## Candidate View contract

The R-027 experimental Epoch envelope and signer/freshness rules are retained,
but its `candidate_fixture` field is replaced for this candidate by one
`view_commitment` with:

- Route Profile identifier for the lifecycle tracer;
- publication cutoff and ordered input-log root;
- accepted-view root and length;
- deterministic-rejection root and length;
- Role Domain assignment seed and algorithm identifier;
- eligible record count and declared finite capacity by Role Domain;
- declared-family count/capacity/concentration summaries;
- exact schema and proof version.

The laboratory input log contains at most `64` Node Records. Every record is
Node-signed and contains network identity, Node Identity, record generation,
validity interval, declared family, supported role-probe capability, static lab
transport endpoint, finite advertised probe capacity, key identifiers, and
signature. It contains no User, Service Name, Service Target, route history, or
Application Data.

Records are evaluated in input sequence at the precommitted cutoff. Every input
has exactly one outcome: accepted or one deterministic rejection code. Accepted
records are canonically ordered by raw Node Identity bytes. Duplicate identity,
generation, key, endpoint handle, invalid signature, invalid time, unsupported
profile, malformed capacity, or source/Node identity collision fails or rejects
according to the frozen matrix; it is never resolved by source order.

Both input and accepted-view roots use one domain-separated binary Merkle tree:

```text
leaf = SHA-256(0x00 || uint32_be(byte_length) || canonical_record_bytes)
node = SHA-256(0x01 || left_hash || right_hash)
```

Tree construction splits at the largest power of two smaller than the current
leaf count, so odd trees have one canonical root without duplicate-last rules.
Empty roots and deterministic-rejection leaves use distinct domain tags frozen
in the Stage 1 golden vectors. This is a lab encoding, not a selected public wire
format.

A Candidate Materialization contains the exact Epoch digest, selected View
index, canonical Node Record bytes, and ordered proof siblings. The requester
precommits its indices before contacting a distributor. Withholding retries the
same index at the other finite source or fails; it never resamples. E verifies
the proof locally. H receives the frozen full input corpus out of band and
recomputes the complete View and summaries. H is not an independent auditor.

## Role Domain assignment

The controlled candidate implements deterministic assignment mechanics without
claiming independent families:

1. validate and group accepted records by declared family;
2. for each family and each permitted domain compute
   `SHA-256("ardents-h3-role-domain-v1\0" || network_id || epoch_number ||
   assignment_seed || family_id || domain_id)`;
3. choose the lexicographically smallest digest; all records in that family
   receive the same domain for the assignment lifetime;
4. ties are impossible unless digest bytes are equal, which fails the candidate;
5. a record is active only when its record, Epoch, profile, assignment, local
   resources, and maximum duty lifetime are simultaneously valid.

The algorithm is a controlled anti-manual-placement candidate, not a final
capacity-balancing algorithm. Its inability to preserve usable capacity across
the fixture is a `redesign`, not permission to hand-place Nodes.

## Node lifecycle contract

Each Node process owns one Node Identity and exactly one active Role Domain. Its
externally observable state is:

```text
ABSENT -> PREPARED -> READY -> DRAINING -> WITHDRAWN
                     |           |
                     +-> FAILED <-+
```

- `ABSENT`: no accepted current local record/assignment; no role listener.
- `PREPARED`: state and keys verify, but activation time/resource checks are not
  yet satisfied; no new work.
- `READY`: exact Epoch/profile/assignment is current and the process may admit
  bounded role-probe work.
- `DRAINING`: new work is refused within `1 s`; accepted work has at most `15 s`
  to finish or cancel.
- `WITHDRAWN`: listener, sockets, timers, queues, ephemeral keys, and temporary
  files are gone; retained rollback floors remain bounded.
- `FAILED`: local integrity, persistence, resource-placement, or evidence
  failure removes readiness and triggers bounded exit.

Withdrawal starts on record removal/revocation, assignment expiry, incompatible
Epoch/profile, unresolved fork/conflict, unsafe Time Confidence, resource DRAIN,
or explicit shutdown. A new-domain duty cannot enter READY until every old duty
is terminal and the quarantine condition in the new Epoch is satisfied. Restart
never restores a live duty; it re-verifies current persistent state and keys.

The role probe uses TLS 1.3 over a static lab TCP Adapter with installed roots
and leaf-key pins. It accepts a nonce-bearing fixed-size request bound to network,
Epoch, Node Identity, assignment, and profile, then returns a bounded response
with the same binding. Early data, resumption tickets, DNS, discovery, relay,
NAT traversal, and fallback are disabled. This transport proves process work and
drain only; Stage 2 may delete it without changing the Node lifecycle Interface.

## Deep Modules and seams

Stage 1 creates only two maintained product Modules:

| Module | Owned behavior | Small Interface | Must not own |
|---|---|---|---|
| Network State | source plan, Epoch/View verification, crash-safe current generation, materialization proof, freshness/conflict/exposure state | open state; refresh through a supplied distribution Adapter; return immutable current Snapshot; close | Node process control, Route, naming, update, Application transport |
| Node Lifecycle | local identity/record match, assignment and lease state machine, readiness, admission/drain/withdrawal, bounded role execution | run one local Node against immutable state updates and one role implementation; return terminal result | Epoch authority, source fetching, route selection, public capacity policy |

The first static TLS distribution Adapter is private to Network State. The
bounded role-probe implementation is private to Node Lifecycle and may be
deleted by Stage 2 without changing its Interface. Qualification owns only the
black-box workload, faults, observers, independent verifier, and verdict; it
does not implement missing product behavior. Product Modules do not import
qualification or `internal/lab`. The resource governor begins unexported inside
each Module; it is not promoted to a generic package until a second real
maintained caller proves a shared seam.

The proposed factual paths, created only with implementation and callers, are:

- `internal/networkstate`;
- `internal/nodelifecycle`;
- `internal/qualification`;
- `cmd/ardents`;
- `cmd/ardents-node`;
- `cmd/ardents-qualify`.

No `util`, `types`, `interfaces`, generic transport, SDK, daemon framework, or
future-role package is permitted.

## Technology selection for Stage 1

| Need | Stage 1 decision | Replaceability limit |
|---|---|---|
| Language/runtime | repository-patched Go 1.26.x | accepted ADR-0009 |
| Hash/signatures | standard `crypto/sha256`, `crypto/ed25519` | no custom primitive |
| Source/probe authentication | standard `crypto/tls`, `crypto/x509`, TLS 1.3, static IP and pins | replaceable H3 candidate; not Route transport |
| Key agreement/AEAD | TLS 1.3 standard-library suite policy | no independent Noise/public suite decision |
| Encoding | strict bounded lab canonical bytes plus golden vectors | no public wire compatibility claim |
| Persistence | immutable generations, fsync, atomic current pointer | no database; real query/transaction need reopens choice |
| Concurrency | `context`, bounded channels, owned counters/semaphores | no external dependency without review |
| Logging | `log/slog`, fixed low-cardinality events | no remote telemetry listener |
| Containment | dedicated Ubuntu Docker Engine host, cgroup v2, namespaces, firewall | selected only as the H3 Stage 1 qualification fixture; not production orchestration |
| Reproduction | the same Compose/topology contract may run locally or remotely | only a preflighted dedicated Ubuntu host produces the official Stage 1 verdict |

Stage 1 adds no runtime module dependency to `go.mod`. Existing Gate C
dependencies remain confined to `internal/lab/namedsite` and cannot be imported.

## Resource hypothesis

R-028 H3-S remains the Endpoint/source process contract. The additional role
probe uses profile `H3-NP1`, deliberately limited to this lifecycle workload:

| Resource | H3-NP1 limit |
|---|---|
| CPU | `1.0` core quota; normal mean `<=0.65`, p95 one-second `<=0.80` |
| Memory | `512 MiB memory.max`; `GOMEMLIMIT=320MiB`; no OOM/oom_kill |
| Go runtime | `GOMAXPROCS=1`, `GOGC=100`, max `512` owned goroutines |
| Process/FD/socket | `pids.max=256`, `512` FDs, `256` sockets |
| Queues/timers | `8 MiB` total mutable queue bytes, `512` timers, no unbounded queue |
| Probe work | `16` open sessions, `4` active; each active client sends one request/s |
| Persistent/evidence | `16 MiB` candidate state/evidence; external evidence separately budgeted |

The Docker host preflight proves that the sum of all candidate CPU quotas is at
most `75%` of effective host CPU and that candidate `memory.max` totals plus the
frozen H/OS reserve fit without host swap or overcommit. Individual H3-S and
H3-NP1 cgroup limits and pass thresholds do not change with a stronger host.
H3-NP1 is not public Node capacity and must not be copied into Stage 2.

All work reserves goroutine, socket, timer, and byte credit before allocation.
Pressure follows `NORMAL -> PROTECT -> DRAIN -> EXIT`. Quiescence after churn
must show no monotonic process, goroutine, FD, socket, timer, queue, or mutable
state growth.

## Evidence and failure matrix

### Evidence plan

#### Primary sources

Accessed 2026-08-11:

- [RFC 9162, Certificate Transparency Version 2.0, Merkle Tree definition](https://www.rfc-editor.org/rfc/rfc9162.html#section-2.1);
- [RFC 8446, The Transport Layer Security Protocol Version 1.3](https://www.rfc-editor.org/rfc/rfc8446.html);
- [Go `crypto/ed25519` package](https://pkg.go.dev/crypto/ed25519);
- [Go `crypto/sha256` package](https://pkg.go.dev/crypto/sha256);
- the dated primary-source lists and bounded experiment contracts in accepted
  R-027 and R-028 for source selection, persistence, cgroup v2, Go runtime,
  resource accounting, statistics, and evidence behavior.

#### Experiment

Freeze the canonical corpus, golden bytes, manifest, source/order seeds, fault
plan, resource profiles, event/sample schemas, calculator, and expected result
before candidate behavior. Execute S1-0 through S1-4 on the exact controlled
topology. H receives the complete frozen input corpus out of band, drives product
executables only as black boxes, and independently recomputes the View,
rejections, proofs, assignments, lifecycle evidence, and terminal verdict.
Generated keys, state, samples, captures, and evidence remain outside Git.

#### Failure scenarios

The mandatory matrix below covers malformed and conflicting state, omission and
withholding, crash boundaries, clock uncertainty, assignment overlap, hostile
resource pressure, evidence failure, process isolation, shutdown, and cleanup.
The falsification section converts every unbounded, collapsed, or
non-recomputable outcome into `redesign` or `stop`; a selected success cannot
compensate for a failed conjunct.

Before feature behavior, freeze a manifest, canonical golden bytes, fault plan,
sample schema, calculator, and expected terminal result. H owns one-second OS
samples and recomputes candidate events against persistent state. Candidate
self-report is diagnostic only.

The short matrix includes at least:

- installed genesis, network refresh, same-Epoch refresh, and restart;
- malformed, oversized, wrong-network, bad-signature, stale, future, rollback,
  signer-transition, equivocation, fork, and source-withholding cases from R-027;
- valid/invalid Node Record, duplicate identity/key/endpoint, cutoff edge,
  deterministic rejection, bad View root/proof/index/summaries;
- admission before/at/after activation and expiry;
- withdrawal by removal, revocation, expiry, profile incompatibility, conflict,
  clock uncertainty, and resource drain;
- crash at each persistence boundary and restart from every durable generation;
- reassignment while work is active, proving no old/new domain overlap;
- source/Node identity and family collision plus Direct Source Exposure behavior;
- slow peer, partial frame, TLS failure, connection flood, forced `EMFILE`, disk
  full, memory pressure, CPU pressure, parent-cgroup change, evidence failure,
  shutdown, and harness impairment failure;
- cleanup and quiescence after repeated churn.

The official candidate runs the complete short matrix, a `2 h` deterministic
churn/resource campaign, then one independent `24 h` unattended campaign from a
virgin state root. A candidate or contract change restarts affected evidence.
The R-027/R-028 `72 h` plus `7 day` pair is not required for this first
mechanical slice; multi-day integrated soak remains a later H3 completion gate.

Machine result is:

- `pass`: every applicable functional, security, resource, cleanup, and evidence
  gate passes;
- `fail`: the campaign is valid but candidate behavior violates any gate;
- `invalid`: frozen conditions or independent evidence are insufficient to
  judge the candidate.

The Product Owner separately records `advance`, `redesign`, or `stop`.

## Falsification and stop conditions

Choose `redesign` or `stop` if any of the following is necessary:

- authenticated state is accepted but no real separate Node process consumes it;
- Node lifecycle requires importing or extending an H2 laboratory Module;
- the role probe leaks into the later Route or Application Interface;
- source, signer, Node, management, and evidence keys or process zones collapse;
- View completeness can pass without full external recomputation;
- a distributor can tailor a valid subset or force silent index resampling;
- stale/conflicting state or failed persistence leaves Node readiness positive;
- reassignment overlaps old and new Role Domain duty;
- queues, retries, state, evidence, or cleanup are unbounded;
- Stage 1 requires libp2p, Waku, a DHT, database, Kubernetes, DNS, public
  consensus, blockchain, or custom cryptography to produce the tracer;
- the one-to-one project cannot maintain the product Modules and controlled
  Docker qualification path.

## Recommendation

Choose H1: implement authenticated state and real Node lifecycle as one Stage 1
vertical outcome. Use R-027 and R-028 as appendices, replacing only the synthetic
fixture with the bounded canonical Candidate View and adding H3-NP1 Node
lifecycle evidence.

Confidence is medium-high. The strongest counterargument is that the role probe
is not useful Route work. That is intentional: it tests the reusable lifecycle
seam without selecting Stage 2 transport. It fails this decision if the probe
cannot later be deleted while preserving the Network State and Node Lifecycle
Interfaces.

## Disposition

- Question state: `decided` by explicit Product Owner acceptance on 2026-08-11.
- Exactly Stage 1 is promoted; Stages 2-8 remain sequential research and
  decision gates.
- R-027 and R-028 are accepted only as the named bootstrap and resource/evidence
  appendices to this decision. Their standalone bootstrap-first work order and
  long-soak schedule are not inherited.
- The standalone H3-A implementation brief is withdrawn in favor of the Stage 1
  brief.
- No ADR is required: Go and repository structure are already accepted, while
  every protocol-shaped choice here is a replaceable controlled-test Adapter.
- No code or generated evidence belongs in this research change.
