# Closed privacy successor: parameters, integration and acceptance

Status: **design and acceptance contract**, not a task ledger or passing
qualification. Owners are the [workload](../product/protected-service-workload.md),
[protocol](../technical/protected-route-protocol.md),
[admission](../technical/private-admission.md) and
[confinement](../technical/application-confinement.md).
The product has one adopted generation; test cases are not user modes.

## Candidate and workload declaration

Bind every run to source commit plus dirty-tree digest, Go/compiler and full
module/tool closure, executable/manifest hashes, Ubuntu/kernel/systemd package
versions, exact State/profile/role inputs, seed, role placement and traffic
conditions. Retain all raw results outside Git with hashes and a deterministic
verdict. Seeds control harness scheduling and network impairments only;
product key, token, nonce and TLS randomness continues to use its normal CSPRNG.
There is no seeded cryptography switch in the installed candidate.
No generated key or traffic trace belongs in the repository.

The first real-host profile is Ubuntu 24.04 LTS x86-64, systemd 255, cgroup v2,
security-updated packages and the existing NET-14 reference hardware/resource
conditions. WSL/Docker component observations are labelled separately.
A missing selected prerequisite invalidates that run; it is never skipped.

Use one Publisher and at least two Endpoint processes. Positive functional
fixtures use distinct synthetic role/family values to exercise selection;
the receipt explicitly states the common real operator/control. They are not
attestations of independent operators. Also run all-known-families-equal
fixtures, which must refuse conflicting selection, and combined full-control
attacks. Public autonomy and public anonymity cannot be inferred from these
closed authority fixtures.

The supported closed directory has at most 32 Nodes and a complete signed
State/profile set of at most 64 KiB. It has enough disjoint eligible Entry,
Interior, Introduction, Rendezvous, Gateway and issuer duties for the selected
roles and their already selected alternatives. Role facts/addresses/keys are
verified by real current validators; do not replace them with trusted booleans.
The selected job uses Target Links. Name input must return not-selected before
network work. The later canonical naming stage needs its own real authenticated
producer and proof path; a local alias or fabricated close cannot qualify it.

The normal latency reference has 20 ms RTT per physical link, 100 Mbit/s
usable bandwidth per direction and no injected loss. The full User–Publisher
path has six physical links. Also measure 40/80/120 ms per-link RTT, asymmetric
links, 0.1%/1% loss and bandwidth ceilings of 10/20/50 Mbit/s. Those additional
measurements expose the cost/availability envelope; they cannot be hidden by
reporting only the reference case. A closed reference pass does not promise
one-second access from every Internet location. Separately run the retained
NET-14AD normal cell: 80 ms total base User-to-Publisher RTT, 100/20 Mbit/s
client inbound/outbound, 100 Mbit/s Publisher/Node, independent 0.1% loss and
p95 <=10 ms directional jitter. The loss-free six-link cell cannot replace it.

## Numeric design parameters

These limits select a bounded engineering profile; they are not measured
production outcomes or authorization to exceed existing product budgets.

| Quantity | Selected bound and accounting |
|---|---|
| Reference request / response | 512 application bytes and 65,536 total response bytes; the text response includes its 13-byte header and 65,523 content bytes |
| Cold response | p95 <=3 s from authorized request to complete validated response; no private destination cache, token stock or prepared prefix |
| Warm response | p95 <=1 s for a new Connection 1–30 s after a prior read in the same context; valid scoped cache/token stock, one target-free prefix and finite actual-work Node Carrier reuse allowed; retained local Rendezvous Node choice, no reserved join or open Service Connection |
| Latency samples | 1,000 cold and 1,000 warm attempts per run, 3 declared seeds; report all outcomes and nearest-rank p50/p95/p99; a non-success in the normal reference counts as infinite completion time |
| First job | UTF-8 document <=4 MiB; one request/Connection, at most 600 s logical lifetime |
| Whole open deadline | 10 s; caller can cancel earlier; timeout is a terminal failure, not a successful response |
| Recovery | Existing immutable Connection authority; retain NET-14 recovery episodes: p95 <=5 s single/sequential and <=8 s overlapping, terminal within 15 s, inside immutable Work Safety; bounded authorized alternatives, no Application request replay |
| Framing/queues | 16 KiB data frames; 64 KiB lane receive window; 4 MiB/prefix and 64 MiB/Node aggregate queued ciphertext; separately reserved control/termination space |
| TLS setup | 5 s and 16 KiB peer handshake input/channel; at most 4 concurrent handshake/verification operations per receiving duty before admission |
| Unused prefix | One/context, 120 s idle expiry, no autonomous refill |
| Introduction | 600 s slot/key, refresh at 300 s, predecessor overlap <=60 s and never beyond its signed expiry; 16 pending capsules/slot, 4 deliveries/s |
| Live publication liveness | At most one request/reply keepalive per 30 s without useful traffic; one designated initiator, real traffic resets the timer; no idle User circuit farm |
| User idle gate | NET-32: <=1,000,000,000 aggregate tx+rx bytes / 24 h; report actual control/transport categories on the declared <=64 KiB State profile |
| Publisher idle | Measure one available publication separately, with its finite readiness/refresh and declared owner hosting allowance; never charge it to the User-only idle figure |
| Private control | 4 KiB Name/Descriptor requests; 16 KiB responses and issuer outcomes; all framing, TLS, refresh, rejection and retry bytes count |
| Normal bulk | Existing NET-14J/K/L/M/N jobs and budgets remain: 16 client / 64 Publisher active Connections, aggregate 10 / 40 Mbit/s, normal carrier ratio <=1.5 |
| Impairment/recovery | Existing NET-14V ratios, per-episode 8 MiB addition and per-direction bitrate bounds remain; no relaxed replacement is selected |
| Local resources | Existing whole-client 512 MiB and Publisher 1 GiB RSS gates, CPU and progress requirements; confinement helpers and ordinary network control are included in their applicable owner totals |

Pace latency trials at no more than 300 cold/warm pairs per hour per User
installation, and lower if its actual token accounting requires it. Use the
same context for each pair and preserve every durable authority, quota and
spend floor between pairs; clear only permitted volatile cold-test state.
Provision hour-scoped permissions before their windows and keep issuance
within the real 4,096/16,384/65,536 limits. Refresh public State/profile normally
across long runs. No test-only issuer, unlimited quota, clock rollback or
ledger reset may manufacture enough successful samples. Failures count even
when a later retry succeeds. Run each seed on both selected Carriers.

User-idle measurement starts after explicit enrollment/bootstrap, with no
Application job or publication active, and includes required State/time refresh,
cleanup, failed refresh and any bounded preparation. Report startup separately.
Retain the current State/time refresh and acceptance rules; profile validity
is at most six hours and is shortened by those authorities. Reuse immutable
public bytes only while their existing validators permit it. No new thirty-
minute polling rule or longer validity is selected to make an idle test pass.
The earlier 8/32 MiB proposals had no whole-system evidence and are retired;
they are not extra product gates. NET-32 and actual local hosting budgets remain
binding. No test should generate traffic merely to spend an allowance.

For NET-14's broader network workload, use a fixed test stream Application in
the separately pinned qualification artifact inventory, through the same
verified installed-unit, descriptor, Principal/Grant and bounded-stream boundary.
It generates the predeclared useful byte schedule and canaries instead of the
text grammar; it has no arbitrary egress or authority. The qualification runner
is its sole caller. A normal text-job inventory cannot admit its executable,
and there is no runtime flag, caller bool or arbitrary-program launch bypass.
Register its exact test package/caller/profile with the implementing change.
The real Endpoint, Node, Route and Service implementations remain unchanged
between the text journey and these network measurements. Report Application
work separately where NET-14 explicitly excludes it.

## Counting useful work and hosting cost

For the same complete workload record U=delivered application bytes, W=all
Ardents network bytes, W0=the declared reference implementation's network bytes.
Report W-U and W-W0 separately. Setup, control, retries, fixed shaping and
background remain separate categories with their sum reconciling to W.

Measure tx and rx independently at User, Publisher and every forwarding Node.
A forwarded byte is received and sent; do not present Node totals as endpoint
overhead or count multi-hop replication as useful application delivery.
Record per-interface capture/sampling loss and checksum reconciliation.
No favorable direction, quiet interval or successful retry offsets a failure.

The resource owner configures actual provider period, byte units, counted
directions and allowance locally. Reserve live admitted work plus termination
capacity. At the low watermark stop new work; drain only within existing
deadlines/reserves and explicitly close before required protection is lost.
Restart does not reset period accounting. Unsolicited incoming traffic and
other host processes can affect an invoice outside Ardents's control.

## Before-task evidence versus implementation evidence

Source and dependency review, token interoperability checks, the confinement
mechanism probe and a causal latency/cost model decide this design.
The arithmetic is not a network measurement. Full end-to-end timing, hostile
load and installed-Application behavior are acceptance tests on the eventual
implementation; requiring those to exist before coding would be circular.

The planned critical path opens fresh Rendezvous and Introduction terminal
channels concurrently, uses bounded optimistic TLS-handshake forwarding,
and keeps Service TLS, Service authentication and application completion in
the clock. Every extra mandatory exchange found in implementation updates the
model; it cannot disappear into a cache or an excluded readiness phase.

## Acceptance matrix

| ID | Receiving boundary / experiment | Required verdict |
|---|---|---|
| P1 Job/authority | Publish/read empty, reference and maximum document by Link; Name refused; wrong Target, bad UTF-8, special/changing file, terminal escapes and extra request | Exact intended content/outcome; no redirected Target, arbitrary file read, automatic URL launch or control-sequence execution |
| P2 Wire/flow | All kinds and state transitions, zero/oversize/truncated/unknown records, lane reuse, credit overflow, blocked lane and hostile control flood | Reject before forbidden effects; bounded memory/work; independent lane progress; preserved EOF and joined cleanup |
| P3 Role observations | Capture each role's entire inputs/state, then prescribed combinations; compare startup, lookup, admission, publication, data and cleanup | No forbidden origin–Target field join or cross-context identifier; complete observer traces, not packet-only evidence |
| P4 Cryptography | RFC 9474/9577/9578 vectors and independent verification; wrong SPKI/key/cohort/receiver, changed capsule, replay, wrong Instance and later-key capture | Required authentication/confidentiality/PFS conditions hold; malformed fields rejected; no claim for captured live recipient keys |
| P5 Admission | Issue/commit/restart ambiguity, exhausted voucher/duty, concurrent duplicate spend, stolen issuer key, bootstrap and verification floods | Durable debit/spend, no quota creation by retries, bounded honest receiver work and truthful unavailable result |
| P6 Local escape | Exact installed worker, IPv4/IPv6/UDP/DNS, inherited descriptors, host IPC, namespace/syscall escape, child tree, same-user attack, paths/storage | Working positive controls and zero forbidden network/file/authority effects; missing enforcement is failure |
| P7 Recovery | Interrupt every durable/volatile transition; late completion, closed stream, revoke/withdraw, conflicting State, disk full/corrupt/leased root | No replayed Application request, resurrected permission/publication, alternate generation or orphaned worker |
| P8 Cost/latency | Numeric matrix above, cold/warm distinction, long stream, idle and failed refresh, honest overload and host allowance exhaustion | Required reference budgets and progress pass; full role/byte/resource reconciliation; all degraded results retained |
| P9 Correlation | Both edges, chosen traffic, delay/drop/watermark, sparse and repeated sessions with nonuniform priors | Diagnostic report of the accepted initial limitation; a correlation success is not relabelled a protocol-field pass, and a failed attack is not an anonymity proof |
| P10 Migration | Old/new peer and artifact combinations, interrupted install/adoption/drain, rollback, state conversion and stale credentials | Only the adopted generation accepts new work; floors survive; no generic/unconfined bypass |
| P11 Supply chain | Exact runtime, worker, tool/test and OS inventories; primary advisories; reproducible build inputs and exported diagnostics | Every known finding fixed or scoped non-applicability proved; fresh review and required repository checks; no invented supported package claim |

For any stronger future correlation claim, freeze challenger, observations,
attack budget, population/prior, duration, metric and confidence rule before
candidate results. No such stronger claim is admitted by P9 here.

## Integration and migration contract

| Existing owner | Change / retained contract |
|---|---|
| State/Source/Enrollment | Authenticate the selected closed profile and issuer/permission roots; preserve current authority, time/freshness/conflict checks and public-only Source effects |
| Entry/Route/Node | Implement the one generation-3 forwarding/lane lifecycle behind the existing deep Route owner; retain Endpoint selection and finite resource ownership |
| Naming/Reachability | Replace successor OHTTP transport with terminal TLS; preserve canonical proof/currentness/floors; migrate only the new reachability payload |
| Credential/Duty | Add the selected blind-token and closed-permission contracts; preserve exclusive roots, exact current duty binding and durable one-use semantics |
| Publication/Instance | Own independent Introduction keys/revisions and bounded readiness; retain Authority separation, withdrawal and non-resurrection |
| Service Connection | Preserve its existing ordered stream and continuity; bind immutable Connection and fresh Attachment contexts separately |
| Endpoint/Broker/Application | Admit only verified confined workers for the successor; add typed Name/Link local Interface version 2 and the fixed text Application without arbitrary execution authority |
| Resource/Release/install | Admit one complete candidate inventory with root-owned units and worker; enforce host periods, authenticated adoption and drain; preserve release/authority floors |

Compatibility is explicit: the current C0 generation and local AAI2 remain
their existing contracts before adoption. Prepare successor binaries,
configuration, units and validated state conversion without accepting new
successor traffic. Quiesce new old-generation jobs, drain at most 5 seconds,
terminate remaining work, then atomically adopt one generation and its exact
artifact/profile digest. Mixed or unknown peer generations are unavailable.

Preserve State, Name, Instance, publication, withdrawal, release and budget
floors. Do not migrate unused old Transit Grants, live joins, private cache
entries or volatile keys into the new generation. Keep old spend tombstones
through their former expiry. Initialize new issuer/receiver ledgers under
fresh signed duty/key generations and separate exclusive roots.

No automatic conversion grants a new Service Credential or revives a published
Instance whose private key was erased. The current no-overlap successor rule
remains. An interrupted conversion reads its durable phase and resumes only
the same authenticated conversion; deletion/reset of roots is not recovery.

Rollback of executable bytes requires the existing fresh Release authorization
and a candidate supporting the adopted protection generation. A predecessor
that only implements the retired generation cannot resume network work.
Restoring old state backups cannot lower floors or resurrect old admissions.

The implementation dependency order is: shared encodings and receiving
contracts -> real authority/admission and forwarding owners -> discovery/
publication/Connection composition -> qualified local job/install integration
-> complete migration and system trials. Each layer uses the already selected
contract of its dependencies. This is a design dependency graph, not issues,
assigned work or authorization to start multiple C0 slices.
