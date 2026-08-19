---
id: R-037
title: Which controlled evidence contract qualifies Horizon 3 Stage 5?
status: decided
owner: product research
started: 2026-08-16
reviewed: 2026-08-16
---

# R-037 — Horizon 3 blocked-entry evidence

## Decision this unlocks

Freeze the controlled topology, censor profiles, clocks, resource budgets,
sample floors, evidence split, and independent `pass|fail|invalid` rules for the
smallest Horizon 3 Stage 5 Bridge slice. The accepted decision authorizes an
implementation brief mapping R-035 Bridge state and the R-036 WebTunnel Adapter
onto maintained packages and one integrated tracer.

The Product Owner accepted option O1 and `h3-s5-b1-v1` on 2026-08-16. This
authorizes preparation and review of the implementation brief. It does not by
itself authorize maintained Stage 5 code, a runtime dependency, candidate
packaging, a public wire protocol, or a censorship-resistance claim.

On 2026-08-19 the Product Owner completed maintained Stage 5 development while
moving the complete qualification campaign to S9.6. This is a scheduling and
promotion decision, not a qualification result. The Product Owner clarified on 2026-08-19 that the evidence
contract is one 564-cell candidate campaign plus six independent five-episode
evidence-integrity campaigns. All 594 episodes,
numeric thresholds, long sustained runs, supply identity, independent verdict,
and cleanup requirements remain mandatory for the final cleaned H3 candidate.
No Stage 5 qualification pass is recorded.

The Stage 5 input records only the reviewed host reservation. At S9.6 a
stand-side collector must derive the actual per-allocation CPU set, cgroup path
and memory limit, and network-namespace inode from the running process trees.
The independent verifier requires that process-derived attestation and rejects
missing or overlapping runtime ownership; a manifest-authored pseudo-namespace
cannot satisfy the contract.

## Current contract

The applicable product and development contracts are:

- [Stage 5 — Bridge and Blocked Entry](../../development/horizon-3-technical-design.md#stage-5--bridge-and-blocked-entry),
  the bounded H3 product journey;
- [Bridge entry](../../security/threat-model.md#bridge-entry) and the
  [threat/response matrix](../../security/threat-model.md#threat-and-response-matrix);
- [Entry exposure and Isolation Contexts](../../product/operating-model.md#entry-exposure-and-isolation-contexts);
- glossary terms [Bridge](../../../CONTEXT.md#bridge),
  [Bridge Invite](../../../CONTEXT.md#bridge-invite),
  [Entry Set](../../../CONTEXT.md#entry-set),
  [Role Domain](../../../CONTEXT.md#role-domain), and
  [Direct Source Exposure](../../../CONTEXT.md#direct-source-exposure);
- [ADR-0005](../../adr/0005-route-domains-and-bounded-entry-exposure.md),
  [ADR-0009](../../adr/0009-go-project-foundation.md), and
  [ADR-0012](../../adr/0012-select-webtunnel-for-h3-camouflage.md);
- [R-023](r-023-interactive-route-performance-budget.md),
  [R-028](r-028-h3-runtime-resource-contract.md),
  [R-032](r-032-h3-same-connection-recovery.md),
  [R-033](r-033-h3-stage-5-research-map.md),
  [R-034](r-034-stage-4-bridge-capacity-sequencing.md),
  [R-035](r-035-h3-bridge-state.md), and
  [R-036](r-036-h3-camouflage-adapter.md).

R-033 through R-036 are decided. R-035 fixes the two-slot Initiator Bridge
Entry Set, one replacement per slot/Epoch, complete four-ordinal contact order,
one non-resetting attempt, and retained exposure. R-036 selects standalone
WebTunnel `v0.0.6` at commit
`d729fde1f38357dcefa2a751eb4752e9ca78f910` behind a candidate-neutral seam and
fixes `5 s` Adapter startup plus `6 s` Adapter cleanup limits. R-037 owns the
remaining numeric parent clocks, integrated hostile cells, Bridge useful-work
capacity, observers, and verdict.

The exact user outcome is a fresh or already-installed Endpoint importing one
authenticated local-file Invite, changing from the ordinary Initiator entry
regime only by the predeclared Owner/policy action, and reaching the same
authenticated network and exact Target through the unchanged Stage 4 Route and
Service Connection when the ordinary entry is externally blocked. Failure must
be explicit and finite; success through DNS, direct access, an ordinary entry,
another Adapter, a shorter Route, cached work, or weaker authentication fails.

The protected information is the exact Target, Service/Instance binding,
opposite endpoint, complete Route, other Role Domain state, Application bytes,
Invite secret, and unselected Bridge material. Adversaries are the censor/DPI,
an uninformed or Invite-informed probe, malicious or stalled Bridge, replaying
distributor, conflicting Node/family, same-host hostile process, and candidate
or harness process that crashes, withholds, exhausts resources, or leaves
residue. The Bridge and adjacent observer still learn Endpoint address, timing,
volume, retry, WebTunnel TLS name/path when informed, and likely Ardents use.
This stage makes no anonymity, address-unblockability, indistinguishability,
operator-independence, or real censored-region claim.

## Hypotheses

- **H1 — finite integrated profile:** one predeclared Ubuntu blocked-entry
  campaign can exercise the exact R-035 state and selected R-036 Adapter,
  preserve the existing Route/Target/Application semantics, provide four
  concurrent Bridge useful-work units on the reference Bridge class, and fail
  every hostile cell without fallback or residue.
- **H2 — functional but unqualified:** the Bridge path carries the exact bytes,
  but one or more clock, pressure, probe, replay, role-conflict, evidence, or
  cleanup gate cannot be made finite and independently verifiable.
- **H0 — redesign/stop:** the selected Adapter or Bridge state requires DNS,
  hidden retry, deadline reset, multi-role reuse, an unbounded resource, or a
  weaker/direct path to complete the controlled outcome.

## Evaluation criteria

The comparison is conjunctive. A security, authentication, role, fallback,
deadline, cleanup, or evidence-integrity failure cannot be offset by good
latency, bandwidth, or availability.

### Exact profile and topology

The profile identifier is `h3-s5-b1-v1`. Its immutable manifest names the
repository commit, source identity, Linux image/kernel, candidate binaries,
configuration hashes, topology, cgroups, workload seeds, cell order, clocks,
budgets, observers, and verifier identity before any measured process starts.

| Boundary | Exact controlled role |
|---|---|
| `E` | Ubuntu LTS `x86-64` User/client Endpoint on the R-023 reference floor: `4` dedicated vCPU threads, `8 GiB` RAM, SSD-backed storage, and a `100 Mbit/s` toward-client / `20 Mbit/s` from-client access link. It owns one Initiator Bridge state store and one network-isolated client Application. |
| `O` | Ordinary Initiator entry fixture from the accepted Stage 4 Route. Its address is reachable only in baseline cells and is blocked externally in Bridge cells. |
| `B` | Separate Ubuntu LTS `x86-64` Bridge on the R-023 infrastructure floor: `2 vCPU`, `2 GiB` RAM, and a symmetric `100 Mbit/s` link. One unprivileged WebTunnel server and one standard-library TLS/HTTP front carry only the endpoint-adjacent leg. |
| `R` | The unchanged real Stage 4 Introduction/Rendezvous/Responder role processes and exact-Target Service fixture. No Bridge state or Adapter configuration enters these roles. |
| `P` | Ubuntu LTS `x86-64` Developer/publisher Endpoint on the same `4`-vCPU, `8 GiB`, SSD-backed floor as `E`, with a symmetric `100 Mbit/s` access link. It owns the unchanged responder Application and exact Target. |
| `H` | Harness/observer host with `4 vCPU`, `8 GiB`, SSD-backed temporary evidence, and `1 Gbit/s`; it owns blockers, probes, clocks, collectors, workload generators, verifier invocation, and cleanup. It is not a Route role. |

`E`, `O`, `B`, `P`, and each existing Route role have distinct process and network
namespaces. The management network is removed before a cell becomes ready.
`E` can reach `O` only through the ordinary-entry boundary and `B` only through
the Bridge boundary. `B` alone reaches the next authenticated Initiator leg.
The Application process has only its accepted local IPC and no network device.

The harness preloads exact signed Invites and fixture credentials through the
management plane, records their hashes, then removes write/runtime distribution
access. During a measured cell there is no Internet, package manager, module
download, DNS service, proxy, alternate candidate, or undisclosed address.

All normal, capacity, and sustained cells inherit R-023's complete normal
reference-network envelope: the `E` access link is `100 Mbit/s` toward `E` and
`20 Mbit/s` from `E`; the `P`, `O`, `B`, and ordinary infrastructure links are
symmetric `100 Mbit/s`; network-only base RTT between the `E` and `P` operating-
system boundaries is `80 ms`; carrier-packet loss is independently `0.1%` in
each direction; additional per-direction delay variation has `p95 <= 10 ms`;
and H injects no complete interruption or packet reordering. C1-C6 change only
the packet acceptance, stall, truncation, or loss explicitly named in their
rows. The immutable manifest binds both the base envelope and each exception;
an undeclared cap, delay, loss, jitter, interruption, or reorder invalidates the
cell.

### Controlled censor and probe profiles

| Profile | Harness action | Required observation | Sample floor |
|---|---|---|---|
| `C0-ordinary-control` | Permit `E → O`; do not authorize Bridge regime. | The existing exact Target and workload pass; no Bridge contact or Adapter process exists. | `20/20` complete attempts. |
| `C1-address-block` | Drop every `E ↔ O` packet before the prearmed transition signal; permit only the manifest-bound numeric `E → B` address/port. | Ordinary useful work is zero; one bounded transition occurs; Bridge useful work passes. | `20/20` complete attempts. |
| `C2-protocol-allow-list` | Apply the C1 address block and permit the Bridge boundary only as the declared TLS/HTTP WebTunnel profile. Reject raw ordinary/PT alternatives. | Selected WebTunnel passes with no DNS or alternate transport; all rejected traffic is retained. | `20/20` complete attempts. |
| `C3-bridge-address-block` | Block both `O` and every configured Bridge address. | Attempt exhausts by its original deadline with no direct, DNS, proxy, ordinary, shorter-Route, or cached fallback. | `5/5` complete attempts. |
| `C4-withhold-stall` | For successive ordinals: refuse, accept then stall before Adapter readiness, stall after readiness, and truncate carriage. | Ledger order and exposure remain exact, cleanup precedes the next ordinal, and final result is bounded exhaustion. | `5/5` complete four-ordinal sequences. |
| `C5-uninformed-probe` | In each episode send one valid-TLS missing-path request, one valid-TLS wrong-path request, one malformed TLS request, and one malformed HTTP request after valid TLS from outside `E`. | Missing/wrong paths receive the manifest-bound ordinary web-front response. Malformed TLS receives the manifest-bound TLS alert or close within `5 s`; malformed HTTP receives the manifest-bound `4xx` or close within `5 s`. None exposes an Ardents identifier, PT readiness, internal address, Invite fact, or success oracle. | `20/20` four-request episodes. |
| `C6-informed-probe` | Use the disclosed numeric endpoint, TLS name, and secret path. | Detection/success is retained as the accepted limitation; it cannot be mislabeled as probe resistance or omitted. | `20/20` disclosed-path episodes must record detection/success. |

These are controlled falsification profiles, not a model of every censor or a
claim that HTTPS-shaped traffic is indistinguishable from an arbitrary site.
The allow-list profile proves only that this fixture permits the chosen shape
while refusing the declared alternatives.

### Numeric clocks and availability

All clocks are harness-bound monotonic offsets from one manifest start. Wall
time is retained only for provenance. A restart does not recreate an offset.

| Clock | Exact value and rule |
|---|---|
| Ordinary blocked observation | `3 s` from the ordinary contact start. Only H may emit the precommitted blocked condition; runtime silence never diagnoses censorship. |
| Policy transition | At most `2 s` after that authenticated harness condition; exactly one `ORDINARY → BRIDGE` record is durable before contact. |
| Bridge attempt | One absolute `64 s` deadline from durable Bridge-regime publication. It never pauses or resets. |
| Contact | `15 s` absolute maximum clipped by the attempt deadline, including Adapter startup, useful work or failure, shutdown, cleanup, and ledger terminal publication. |
| Adapter startup | R-036 hard maximum `5 s`, inside the contact. |
| Inter-contact interval | Exactly `1 s` after terminal cleanup; it creates no extra ordinal and is clipped by the attempt deadline. |
| Adapter cleanup | R-036 hard maximum `6 s`, inside contact and attempt deadlines. |
| Whole-cell cleanup | At most `15 s` after the terminal result to remove all harness-owned containers, namespaces, networks, mounts, files, processes, sockets, queues, and timers. |

The four possible starts remain slot `0` initial, slot `0` retry, slot `1`
initial, slot `1` retry. Missing/ineligible members are skipped, not replaced.
A successful contact skips remaining ordinals. Failed short-cell latency is
infinity and failed goodput is zero. Percentiles use nearest rank; no failure,
timeout, or crash is dropped.

When Bridge acquisition participates in an already-live Service Connection's
R-032 recovery episode, the original episode clock remains authoritative. The
effective Bridge deadline is `min(Bridge attempt deadline, recovery terminal
deadline)` and is therefore no later than `15 s` after the last pre-failure
Application byte. Contact, startup, inter-contact spacing, cleanup, and ledger
publication are clipped to the remaining parent time; no Bridge event resets or
extends it. A successful authenticated continuation before that deadline keeps
the same Service Connection. Otherwise the Application receives the supported
`connection loss` Connection Result by the original deadline, and every
unstarted ordinal remains unstarted. Five recovery-parent-clipping episodes
stall the first Bridge contact; H terminates the stall at recovery offset `8 s`,
after which Adapter cleanup must finish by offset `14 s` and the ledger plus
`connection loss` result must publish by offset `15 s`. Each episode produces
exactly that result, zero later starts, the unchanged attempt ID/deadline, and
zero residue. They are recovery misses, never positive Bridge attempts.

Positive deterministic short cells require `20/20` successful attempts per
applicable profile. Each hostile state/replay/substitution case runs five
manifest-seeded episodes unless a larger floor is stated below and must produce
the exact expected terminal class in `5/5`. An observed hard security violation
fails immediately regardless of sample count. Scheduled repetitions are never
post-hoc reruns.

### Useful work, bandwidth, and Bridge capacity

Every short positive attempt carries fresh `32-byte` connection canaries, the
exact nonce-bearing `512-byte` HTTP request, and exact `64 KiB` verified
incompressible response through the same authenticated Target, Route shape,
Service Connection, and Application framing used before Stage 5. Bridge startup
to verified response has `p95 <= 8 s`; the `15 s` contact and `64 s` attempt
deadlines remain hard even if that percentile passes.

One **Bridge useful-work unit** is one authenticated endpoint-adjacent Carrier
Channel that carries that complete request/response and reaches terminal
cleanup without giving `B` the Target, other Route positions, Application
plaintext, or another Role Domain.

The reference `B` class must complete four concurrent units from four isolated
fixture Endpoints in five batches, with all twenty units successful. A fifth
concurrent authenticated admission is refused within `1 s` without evicting or
slowing an established unit past its deadline. The stronger functional profile
`h3-s5-b1-v1-strong` uses the identical binary, configuration semantics,
security/failure behavior, and impairment profile, but freezes `B` at `8` vCPU,
`8 GiB` RAM, SSD-backed storage, and a symmetric `400 Mbit/s` link. Its cgroup
gates are exactly four times the reference candidate gates: `cpu.max=6.4`
cores, mean CPU `<= 4.48`, p95 one-second CPU `<= 5.12`,
`memory.max=5120 MiB`, p95 `memory.current <= 3584 MiB`, helper p95 RSS
`<= 512 MiB`, `256` helper FDs, and `128` helper sockets, with at least `20%`
CPU/memory/link reserve and zero swap/OOM. H uses a separately charged
`16`-vCPU, `32 GiB`, `1 Gbit/s` collector class. Sixteen isolated reference
fixture Endpoints drive 16 concurrent units in five batches (`80/80`) with one
bounded seventeenth refusal per batch; no normal or hostile oracle changes. It grants no production
capacity, trust, selection weight, or role.

Sustained evidence uses one established Bridge unit for five `10 minute` runs
in each separate Application Data direction. Each run contributes exactly ten
non-overlapping `60 s` windows, producing fifty ordered values per cell. The
nearest-rank `p05` must be `>= min(10 Mbit/s, 50% of the paired direct
baseline)`; failed windows are zero. One verified `60 s` direct transfer
immediately before and one immediately after the complete batch use the same
incompressible stream, direction, endpoint pair, caps, and impairment manifest.
Both results must be positive, complete, digest-valid, and correctly paired,
with `max(B_before, B_after) / min(B_before, B_after) <= 1.10`; the arithmetic
mean `(B_before + B_after) / 2` is the baseline. A zero, incomplete, corrupt,
mismatched, or over-drift direct result invalidates the complete batch. Only a
confirmed candidate-independent harness/environment fault permits a full rerun;
candidate-caused drift fails the candidate and all original evidence remains.

The active endpoint carrier ratio is computed separately at the `E` and `P`
operating-system network boundaries and separately for each Application Data
direction as `(all Ardents-attributable bytes sent + received) / delivered
Application Data`. The numerator covers every interface carrying the measured
Ardents path and includes framing visible at that boundary, encrypted carrier
payload, control, acknowledgements, keepalives, retransmissions, padding, cover
traffic, and background traffic from Ardents and its helpers; unrelated OS and
Application traffic is excluded by the frozen attribution manifest. The
denominator is only verified incompressible bytes delivered to the receiving
Application in the tested direction. Each ratio must be `<= 1.5`. Short-cell
handshake traffic is reported but has no ratio gate because the fixed payload
would make setup cost dominate.

### Resource, storage, and pressure budgets

Every process, descendant, TLS front, listener, socket, pipe, state file, page,
queue, timer, packet, and cleanup action is charged to its declared candidate,
Bridge, Endpoint, or harness parent. Moving work to a helper never excludes it.

| Boundary | Normal gate |
|---|---|
| Complete Endpoint tree | Existing active-client gate: mean CPU `<= 0.50` core, p95 one-second CPU `<= 1.00` core, p95 RSS `<= 512 MiB`, and `256 KiB` logical Application queue per direction. |
| Endpoint Adapter subtree | One unprivileged WebTunnel client, zero descendants/capabilities, p95 RSS `<= 64 MiB`, at most `16` FDs and `4` live sockets, one `<= 1 MiB`/`32`-entry state directory, and no writes elsewhere. |
| Complete Bridge candidate tree | R-028 reference containment: `cpu.max=1.6` cores, mean `<= 1.12`, p95 one-second `<= 1.28`, `memory.max=1280 MiB`, p95 `memory.current <= 896 MiB`, no swap/OOM event, and at least `20%` CPU/memory/link reserve. |
| WebTunnel Bridge helpers | Exactly one unprivileged server and one unprivileged standard-library TLS front, zero descendants/capabilities, combined p95 RSS `<= 128 MiB`, at most `64` FDs and `32` sockets at four-unit load. |
| Durable Bridge state | One exclusive state root, at most four member records, four contact records, one attempt, one regime record, and `<= 256 KiB` including current/previous/temp generations; no secret address/config survives terminal Epoch cleanup. |
| Evidence | Candidate-local bounded events `<= 16 MiB`; external per-role and harness streams `<= 2 GiB` each, preflight projection `<= 80%`, outside Git. No required verdict event may be dropped. |

Collectors sample monotonic time, process tree, CPU, RSS/cgroup memory, FDs,
sockets, PIDs/threads, goroutines, timers, queues, state bytes/entries, traffic,
DNS packets, progress, and cgroup events once per second. Short cells additionally
retain pre-start, transition, readiness, useful-work, terminal, and post-cleanup
snapshots. Missing or coalesced mandatory samples invalidate the affected cell.

At projected resource exhaustion, admission is rejected before allocation and
established work continues. At R-028 PROTECT high water, no new Bridge contact
starts; an established contact retains its original deadline. At DRAIN, no new
work starts and every owned resource exits within `60 s` or the earlier Work
Safety/attempt bound. A dedicated pressure cell must return from PROTECT only
after all low watermarks hold for `120 s`; entering DRAIN fails that recoverable
cell but is the expected result in the separately declared emergency cell.

The resource cells use this exact precommitted pressure schedule and corpus:

| Cell | Offered input and schedule | Required result |
|---|---|---|
| `P0-four-unit-hold` | Establish four Bridge useful-work units, each carrying a distinct manifest-seeded `10 Mbit/s` stream for `30 s`. | All four progress under the normal gates for the full hold. |
| `P1-projected-admission` | While P0 is held, offer one fully authenticated fifth unit every `100 ms` for `10 s` (`100` offers). Every offer carries a distinct `32-byte` canary and the exact `512-byte`/`64 KiB` corpus if admitted. | All `100` refuse before allocation and within `1 s`; the four established units retain progress and their original deadlines. |
| `P2-recoverable-socket` | With one established unit progressing, preflight requires exactly six charged helper sockets: two listeners, the front's accepted and outbound sockets, and the server's accepted and next-leg sockets. H opens exactly `20` manifest-bound `128-byte` partial TLS handshakes to the Bridge front at `2/s`, producing exactly `26/32` total charged helper sockets, then holds them through three complete one-second samples and closes them all. | PROTECT begins after the third high sample; no new contact starts; the established unit progresses. After close, every low watermark, including six total sockets (`<= 19`), holds for `120` consecutive samples and the same process returns to NORMAL. DRAIN fails this cell. Any baseline other than six or any injected connection that does not add exactly one charged socket invalidates the cell. |
| `P3-emergency-socket` | From a fresh fixture with the same exact six-socket baseline, H opens exactly `23` partial handshakes at `2/s`, producing exactly `29/32` total charged helper sockets, and holds them. | Crossing the `90%` threshold enters DRAIN immediately; no new work starts and the complete owned tree exits within `60 s`. Returning to NORMAL, exceeding the 32-socket hard cap, or an OOM fails. A baseline/count mismatch invalidates. |
| `P4-refusal-churn` | Repeat P1 in ten fresh batches (`1,000` total offers) with a new manifest-seeded corpus and state root per batch. | Admission allocations, open FD/socket/goroutine/timer deltas, state bytes, and evidence counts reconcile exactly; no residue or upward trend may be hidden by a restart. |

The manifest binds every corpus byte/digest, source address, cadence, connection
count, expected accepted/refused count, pressure threshold, and sample ordinal.
A different ramp, rate, duration, allocation input, or threshold is not evidence
for these cells.

### Mandatory state, attack, and recovery cells

In addition to C0–C6 and useful-work/resource cells, the manifest contains the
following five-episode groups:

1. malformed, non-canonical, oversized, duplicate/trailing-field, wrong-signature,
   wrong-network, wrong-Epoch, wrong-profile, expired/not-yet-valid, and
   insufficient-Time-Confidence Invite;
2. Responder/Introduction/Rendezvous/Resolution/unknown domain, conflicting
   retained family, Direct Source Exposure, Interior/live Route, drain, and
   quarantine collisions;
3. exact active reimport, retired replay, same-generation different bytes,
   skipped generation, wrong replacement ID, third generation, full set, and
   cross-slot replacement;
4. restart after import, regime publication, each exposure publication,
   Adapter start, readiness, useful-work prefix, terminal record, and cleanup;
5. slow/partial handshake, malformed PT control, wrong SOCKS listener/method,
   child exit, `SIGTERM`, `SIGKILL`, Bridge accept-then-stall, malformed
   carriage, and evidence write exhaustion;
6. Target, Instance generation, Network, Route Profile, Isolation Context,
   Route generation, attachment, and Application canary substitution;
7. attempts to use DNS, environment proxy, ordinary entry, direct Target,
   alternate address/candidate, shorter Route, cached success, or deadline and
   exposure reset; and
8. cancellation, expiry/revocation during contact, Endpoint restart, Bridge
   process restart, collector loss, blocker loss, clock discontinuity, and
   residual injection; and
9. unknown Invite fields, BRIDGE/ORDINARY regime oscillation, out-of-order or
   duplicate contact/retry commands, attempted ledger reset after restart or a
   new Application operation, candidate leakage of injected Invite/address/path/
   certificate canaries to ordinary diagnostics, and separate harness-pipeline
   contamination of publishable evidence with those canaries.

Unknown fields are rejected as `invalid` before atomic import with zero durable
mutation. A second regime transition in the same Epoch/attempt, slot `1` before
slot `0`, retry before its initial contact, duplicate ordinal, or any ledger
reset is rejected without a network contact; the original attempt, exposure,
ordinal, and deadline records remain byte-identical. Each secret variant uses
four distinct manifest-seeded `32-byte` canaries bound respectively to the
Invite, numeric address, WebTunnel path, and fixture certificate. Candidate
arguments, PT control stdout, and WebTunnel stderr remain permitted raw secret
evidence under R-036 and must be drained only to the external secret root.
Trustworthy candidate emission of any canary to an ordinary-diagnostic event or
other publication-eligible field is a candidate security `fail`. The separate
harness-contamination variant injects each canary directly into a
publishable artifact and must return `invalid` because the evidence pipeline is
untrustworthy. A valid/pass artifact contains only canary/artifact hashes and
zero raw canary bytes. Raw canaries, candidate logs, and contaminated artifacts
remain only in the external secret root. Each named variant runs five episodes
and must produce its exact result in `5/5`.

Every failure before atomic import changes no Bridge state. Every uncertain
post-exposure crash consumes that ordinal. Restart resumes cleanup only and
returns `bridge-interrupted`; it never resumes a live stream or starts the next
ordinal. Expiry, revocation, role conflict, or Time Confidence loss stops new
work and terminates by the earliest accepted bound.

### Resolver, fallback, and probe observation

Every egress interface in the `E` Endpoint/Adapter namespace, the `B` TLS-front
and WebTunnel-server namespaces, and the `B → R` next-leg boundary is observed
for IPv4/IPv6 TCP/UDP port `53`, common IPv6 extension headers, proxy variables/
configuration, connect targets, and all boundary packets. The `P` endpoint and
ordinary Route-role boundaries retain the accepted Stage 4 observers. Each
candidate/helper namespace must first produce and drain IPv4 UDP, IPv6 UDP, and
IPv4 TCP positive-control flows; its control exemption closes before candidate
startup. Non-initial fragments, ESP, overlong extension chains, observer loss,
an unobserved interface, or ambiguous process/cgroup attribution invalidates the
cell instead of supporting zero.

For every Bridge or negative cell:

- candidate/helper DNS packets at every observed egress are exactly zero;
- `E` connects are a subset of the manifest numeric Bridge target; `B` outbound
  connects are a subset of the single authenticated next-Initiator-leg target;
  every other candidate/helper connect target is forbidden;
- ordinary, direct Target, alternate candidate/address, proxy, shorter-Route,
  and Application-network packets are exactly zero; and
- the uninformed probe response contains no Ardents-specific identifier,
  Invite/path fact, PT line, internal address, Target, or readiness result.

The informed C6 observation is published as a limitation and never contributes
to a positive probe-resistance claim.

### Evidence and independent verdict

Raw Invites, addresses, TLS/path secrets, arguments, keys, packet captures,
workload seeds, and candidate logs remain in a secret directory outside Git.
Publishable evidence contains schema/profile IDs, pseudonymous manifest-scoped
identities, source/binary/harness/verifier hashes, exact cell/run IDs, clocks,
event commitments, workload digests, aggregate packet/resource/traffic series,
terminal classes, cleanup facts, and hashes of every secret artifact.

The candidate cannot emit `pass`. After a cell is frozen and candidate
processes are terminal, a separately built verifier receives the immutable
manifest; canonical publishable events/samples; cleanup inventory; a read-only
external secret-artifact tree whose hashes match the publishable commitments;
and the private four-canary corpus whose hash is manifest-bound. It scans the
predeclared ordinary-diagnostic/publication-eligible fields and every
publishable file for raw canaries without copying them to its output. Allowed
secret configuration, candidate arguments, PT stdout, and secret-only stderr
are hash-checked but are not ordinary-diagnostic leakage surfaces. The verifier
then emits one canonical result:

- `pass`: every mandatory cell/sample is present and all applicable conjunctive
  behavior, security, timing, resource, traffic, and cleanup gates pass;
- `fail`: trustworthy evidence shows candidate/state/Route behavior missed a
  gate, including timeout, crash, resource exhaustion, forbidden packet/path,
  wrong terminal result, or security violation; or
- `invalid`: the harness, observer, clock, environment, input commitment,
  attribution, evidence completeness, or cleanup inventory cannot support the
  judgment independently of candidate behavior.

Invalid evidence is retained with the reason and replacement-run link. A
candidate failure is never renamed invalid. A raw canary in a publishable
artifact is `fail` when trustworthy attribution proves candidate emission into
an ordinary-diagnostic/publication field; it is `invalid` when independently
injected by the harness/pipeline or when its cause cannot be attributed
trustworthily. Likewise, a trustworthy candidate-owned residual is `fail`,
whereas an incomplete/ambiguous harness cleanup inventory is `invalid`. Every
residual process, listener, socket, namespace, mount, file, queue, timer,
cgroup, or publishable secret blocks the next cell until the external harness
has removed the isolated fixture, regardless of verdict class.

### Operator, governance, maintenance, license, and accessibility

The campaign uses only the Product Owner/Codex project, local fixture roots,
the accepted Network/Bridge keys, the pinned Ubuntu/Go supply, and the selected
WebTunnel/goptlib licenses and source identity from R-036. It assumes no public
Bridge distributor, external operator, auditor, user panel, account, CAPTCHA,
token, domain broker, or real censor. The Bridge TLS name/certificate and
numeric address are preprovisioned secret fixture inputs, not public DNS.

No candidate module enters root `go.mod`; exact candidate binaries and notices
are supplied as separately hashed offline artifacts. A changed candidate,
dependency, toolchain, image, license, advisory state, or helper topology
reopens the applicable R-036/R-037 gate. The development surface is one strict
local Invite file, one bounded campaign manifest, one command, and one canonical
verifier result. Windows, installer/public UX, public Invite acquisition, and
real censored-region accessibility remain later-horizon work.

## Evidence plan

### Primary sources

Accessed 2026-08-16:

- the accepted product, threat, operating-model, ADR, and research records
  linked in Current contract;
- R-023 P3-D3b3, P3-D6a, P3-D6b2a, payload, direct-baseline, sampling,
  evidence-bundle, and requalification decisions;
- R-028 cgroup/runtime ownership, pressure, evidence, cleanup, and reference
  H3-S profile;
- R-032 and the accepted Stage 4 brief for non-resetting deadlines, Route/
  Service Connection continuity, fault ownership, and independent verdict;
- R-035 for Invite/state/contact/exposure/restart semantics; and
- R-036 plus its retained experiment for selected source identity, Adapter
  process contract, DNS observer limits, shutdown rungs, resource measurements,
  licenses, maintenance, and known probe limitations.

### Experiment

No integrated experiment is authorized while this record is active. After
Product Owner acceptance, the Stage 5 implementation brief must freeze exact
encoding, package/command ownership, TDD seams, Docker topology, manifest and
evidence schemas, verifier inputs, build supply, and vertical order before code.

The later maintained campaign reproduces `h3-s5-b1-v1` with no runtime fetch,
retains all generated material outside Git, runs deterministic unit/command E2E
cells during development, and runs the full Ubuntu Docker suite at S9.6 against
the cleaned integrated H3 candidate. The record may be falsified before implementation by
showing that any numeric budget cannot fit the fixed R-035/R-036 parent
mechanics or contradicts an accepted higher-authority contract.

### Failure scenarios

The C0–C6 profiles and nine mandatory hostile groups above cover malicious,
degraded, recovery, governance, and evidence failure. In particular, they do
not treat an honest Bridge as the only failure mode: malicious state import,
informed probing, selective withholding, candidate compromise, same-family role
capture, pressure, observer failure, supply drift, clock rollback, cleanup
residue, and the absence of independent/public operators remain visible.

### Falsification criteria

Choose H0 or redesign before maintained code if the accepted brief would need:

- more than two slots, one replacement per slot, four starts, or one attempt;
- public DNS, proxy, another candidate/address, automatic family cycling, or
  an ordinary/direct/shorter/weaker fallback;
- a reset/extension of the `64 s` attempt or any R-035 exposure history;
- Bridge reuse in another Role Domain or conflicting live/retained family;
- Route, Service Connection, Target, or Application semantics changed to fit
  WebTunnel;
- unbounded process, state, queue, timer, evidence, traffic, or cleanup cost;
- omission of TLS front, candidate, observer, or failed-attempt resources;
- candidate-authored verdict or a missing/ambiguous security observation; or
- a stronger privacy, availability, independence, or censorship claim than the
  exact controlled evidence supports.

## Findings

**Sourced fact:** R-035 already makes four contact starts, four member-history
records, one transition, and one attempt the maximum finite state. R-037 can set
clocks and repetitions without adding state-machine branches.

**Measurement:** the final R-036 campaign completed all six candidate/shutdown
cells with zero observed candidate DNS packets, zero ambiguity, and complete
cleanup in at most `306 ms`. Selected WebTunnel used at most `6,016 KiB` client
RSS, `4,480 KiB` server RSS excluding the harness front, two state entries, and
`80` state bytes in those single-contact fixtures.

**Measurement:** the accepted Stage 4 implementation already carries the exact
authenticated Target and Application workload through real role processes,
recovery, pressure, and cleanup. Stage 5 needs to replace only the
endpoint-adjacent Carrier acquisition, not reconstruct Route or continuity.

**Assumption:** four concurrent Bridge useful-work units on the reference class
are sufficient for the one-owner H3 development topology. This is a small
falsifiable capacity floor, not evidence for public Bridge availability.

**Inference:** a `64 s` attempt with `15 s` contacts and `1 s` terminal-cleanup
spacing contains all four ordinals (`63 s` maximum) without hiding the fixed
`5 s` startup or `6 s` cleanup bounds. Clipping remains authoritative if an
earlier operation overruns rather than extending the attempt.

**Inference:** 20 deterministic positive attempts and five episodes per
hostile variant provide useful local regression evidence while remaining below
R-023 public qualification floors. They cannot be presented as V1 release or
real-censor statistics.

## Options

| Option | Product/security fit | Latency, bandwidth, storage, and availability | Operations and governance | Maturity, license, distribution, and DX |
|---|---|---|---|---|
| O1 — exact `h3-s5-b1-v1` matrix | Preserves R-035/R-036 seams, hard-fails forbidden paths, and records informed-probe/Bridge visibility limitations. | Finite 64-second attempts, exact goodput/traffic/state/evidence bounds, four-unit capacity, pressure and cleanup gates, explicit `pass|fail|invalid`. | One-owner offline fixtures and existing project roots; no public operator, broker, DNS, account, or new governance authority. | Uses the accepted pinned WebTunnel/goptlib licenses and supply; largest implementation/review cost, but one deterministic Linux command and verifier result limit misuse. |
| O2 — happy-path smoke plus qualitative negatives | Demonstrates bytes but cannot support replay, probe, role, fallback, restart, or exact-Target claims. | Cheap and fast, but has no capacity, resource, deadline, sample, storage, or cleanup verdict and therefore no availability result. | Same fixture roots and no new governance, but failures remain operator interpretation rather than a mechanical verdict. | Same accepted candidate/license; lowest implementation effort and highest misuse/maintenance ambiguity because a smoke result is easy to overstate. |
| O0 — stop before Stage 5 code | Makes no unsupported Bridge claim and preserves all existing stages. | No latency, bandwidth, storage, blocked-entry availability, or Bridge-capacity result. | No Bridge operator/distributor or new governance dependency. | No candidate distribution or new maintenance/DX cost; Stage 5 remains documentation-only until its contract changes. |

No option changes public distribution, governance, accessibility, or real-world
censorship support. O1 and O2 use the same selected candidate and licenses;
their difference is whether the evidence can falsify the product contract.

## Recommendation

Choose O1 with medium-high confidence and accept `h3-s5-b1-v1` as the exact
Stage 5 development/evidence contract. After acceptance, prepare one explicit
implementation brief. Do not begin maintained code until that brief is itself
accepted.

The strongest argument against O1 is campaign cost: sustained directions,
pressure, hostile state groups, and cleanup make it much larger than a simple
Bridge demo. The cost is justified because Stage 5 combines hostile local
state, an external transport process, censorship inputs, and security-negative
claims; omitting those cells would create progress-looking code without a
trustworthy terminal verdict.

## Disposition

- State: `decided`; the Product Owner accepted option O1 and profile
  `h3-s5-b1-v1` on 2026-08-16.
- Authorized follow-up: the Product Owner explicitly accepted the Stage 5
  implementation brief on 2026-08-16, authorizing S5.1 through S5.5 in order.
- Documents changed by acceptance: this R-037 record, the R-037 row in
  [questions.md](../questions.md), and the accepted
  [Stage 5 implementation brief](../../development/horizon-3-stage-5-brief.md).
- No ADR is created: this record operationalizes accepted ADR-0005/0009/0012
  and does not select another hard-to-reverse technology.
- That acceptance authorizes only the brief's maintained code and integrated
  local experiment; it does not authorize public distribution, a release
  claim, a new module dependency, or unpinned runtime supply.
- The prior R-036 disposable harness remains retained source evidence; all
  generated candidates, keys, captures, state, and run evidence stay outside
  Git.
