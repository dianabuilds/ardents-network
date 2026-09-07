# R-149 coupled feasibility worksheet

Status: **executed analytical experiment with predeclared inputs**, 2026-09-06. This is a disposable
calculation and counterexample model for the Product Owner's selected
[R-149 question](../../docs/research/records/r-149-autonomy-transition.md).
It is outside C0 implementation and does not run an Ardents network, cryptography,
a consensus engine, a clock protocol, or a public qualification.

## Interpretation after Product Owner clarification

The Product Owner clarified on 2026-09-06 that mechanisms follow tasks and that
PoW is optional. This worksheet samples one hypothetical native-work profile;
it neither makes that profile the preferred architecture nor rules out non-PoW
solutions. Its numerical results and captured receipts remain unchanged.
The [task-first comparison](../../docs/research/records/r-149-autonomy-transition.md#task-first-mechanism-comparison)
owns the next research question. Blocks, confirmations, work shares and history
sizes below apply only to this example, not to every autonomous design.

## Question and current contract

Can a resource-backed reference ordering, bounded readers, justified time, and
finite canonical Name Leases satisfy the retained product constraints together?

Authority remains in the [product scope](../../docs/product/scope.md),
[requirements registry](../../docs/product/functional-map.md),
[operating model](../../docs/product/operating-model.md#autonomous-shared-state-contract),
[threat model](../../docs/security/threat-model.md#hostile-environment-premise-and-residual-assumptions),
[State owner](../../docs/technical/network-route-node.md),
[naming owner](../../docs/technical/naming.md), and
[ADR-0074](../../docs/adr/0074-target-non-administrative-public-operation.md).
ADR-0075/0076 remain proposed. Current C0 behavior is unchanged.

**H1:** At least one filled reference profile fits the arithmetic timing and
bandwidth constraints, with explicit surviving assumptions.
**H2:** Finite Name renewal can survive a declared finite censorship interval,
but not arbitrary omission without changing the lease contract.
**H0:** No complete public profile can be qualified from these inputs because
proof, fresh bootstrap, inclusion, or funded honest supply remains unjustified. This is a selection outcome, not a theorem that no future construction can fit.

## Frozen inputs before execution

Every value below is an **assumption for comparison**, unless identified as an
existing product ceiling. It is neither a measurement nor a selected wire.

| Quantity | Reference and sensitivities |
|---|---|
| Infrastructure declarations / Names | 1,000 / 10,000; also 10,000 / 100,000 |
| Node current record / admitted update | 512 / 1,024 bytes, including assumed framing |
| Node renewal and churn | Renewal every 6 hours; 5% additional updates per day; 1-hour renewal sensitivity |
| Name record / update / root-claim pair | 1,920 / 4,096 / (64 + 4,096) bytes; these do not certify composed proof fit |
| Name workload | 100 new Names/day; all Names renewed every 30 days; 1% other updates/day |
| Work reference | Independent fixed-rate Poisson work, total reference interval 60 seconds; fixed difficulty during modeled loss/attack |
| Adversarial work share | 10%, 30%, 40%, 49%, 51%; 30% comparison envelope, not a promised deployment |
| Confirmations | 6 and 80; per-race catch-up comparison threshold 1e-6; not lifetime finality |
| Epoch | 300 seconds; commits in E, reveals in E+1 |
| Admission plus final evidence acquisition/checking / confirmation planning | 600 seconds combined / 10,800 seconds; optimistic planning envelopes, not deterministic PoW guarantees |
| State cutoff maximum age | 1,800 seconds; 14,400-second sensitivity |
| Header / input block | 256 bytes / 64 KiB; at most 32 KiB per block in each of infrastructure and naming classes |
| Time | Initially justified interval +/-30 seconds; surviving elapsed-clock drift <=50 ppm; maximum allowed uncertainty +/-120 seconds |
| Name lifecycle | Active 30 days, Grace 7 days; renewal starts 7 days before Active end; signed Record and parent valid through the tested deadline unless a case overrides it |
| Name censorship | 0, 7, 13, 14, 21 days; cutoff rounding 300 seconds and combined clock margin 240 seconds |
| Client startup | Existing normal clean-start p95 <=15 seconds, routine-restart <=5 seconds; incoming access 100 Mbit/s |
| Client idle | Existing 256 MiB p95 RSS, 1% mean of one core; 25 MiB/day secondary traffic guardrail |
| Infrastructure | Existing 2 vCPU / 2 GiB / symmetric 100 Mbit/s minimum comparison class |
| Modeled traffic allowance | Payload multiplier 1.5 plus 8 MiB/day for other required carrier work; neither is measured sufficiency |
| Hypothetical compact proof | 8 KiB or 64 KiB per 300 seconds; mechanism and producer/verifier cost UNKNOWN |
| Retention | 365 days full replay comparison; no assumed trusted rolling checkpoint |
| Resource supply | Normalized honest H=70, attacker A=30 units; honest loss sensitivities 50%, 90%, 100%; units are work throughput, not Nodes or currency |

The 600-second input is counted once as an optimistic combined reserve for admission and final evidence acquisition/checking. Actual proof checking and delivery are unmeasured; the fit is conditional on their inclusion in that reserve. The example lease durations are new research inputs; current product documents
leave durations unselected. A Name renewal frequency of 30 days is an offered-load
assumption; the explicit day-23 trace evaluates an earlier individual renewal.
Network ordering and Namespace verification remain separate data/role domains.
Only opaque Name inputs may enter the Network domain. Bytes assigned to a Name
body refer to its separate Namespace path, not plaintext replicated to all
Network producers. The shared 64 KiB capacity is an optimistic aggregate
accounting envelope across those paths, not a selected shared block format.

## Falsification criteria fixed before running

1. Reject a proposed arithmetic timing fit if confirmation, cutoff, acquisition
   and clock margin exceed usable State age, or a renewal arrives at/after the
   conservative owner/parent deadline. Timer expiry cannot invent a shared
   Released Record, reclaim winner, or voluntary abandonment.
2. Reject a claimed risk fit above 1e-6 per modeled race at q=0.30. Include q>=0.5,
   loss of honest supply, and partition as failure cases; the formula is not a
   global capture, inclusion, or lifetime-risk bound.
3. Reject a normal readiness claim requiring >15 seconds of payload transfer
   alone. An arithmetic fit cannot qualify readiness without actual verification,
   acquisition, time and Route work. The 25 MiB idle check is an efficiency
   comparison, not a runtime quota or independent release blocker.
4. Reject acceptance from mere signed roots, headers, sample proofs, agreeing
   clocks, source count, or a saved lower time watermark without the missing
   validity/completeness/availability/currentness evidence. No model boolean is
   a substitute for a cryptographic verifier.
5. A finite queue/class cap must bound memory but cannot claim per-person
   fairness or timely renewal under a saturating valid Sybil load.
6. Reject any claim of measured CPU, RSS, cryptographic proof size, independent
   operators, funding, anonymity, maintainability, or real network throughput:
   this experiment supplies none.
7. Preserve retained safety floors in the proposed conflict response. Two
   separately exposed committed branches cannot silently merge; detection is
   itself conditional on obtaining contradictory evidence.

## Procedure and evidence

Run the standard-library Python calculator from any working directory:

```powershell
python -B C:/Users/vitek/code/ardents-network/experiments/r-149-coupled-feasibility/calculate.py
```

The script prints JSON to stdout. Redirect output only to an external temporary
or evidence directory; no generated results, caches or binaries belong here.
The retained script and this predeclared input set permit exact arithmetic
reproduction. Results are analytical counterexamples and conditional bounds,
not measured Ardents behavior. No runtime dependency or maintained package is
introduced. Calculation consistency checks use two formulations of the work
race and known boundary cases; they do not test the product implementation.

## Primary evidence

Accessed **2026-09-06**:

- Grunspan and Perez-Marco,
  [Double Spend Races](https://webusers.imj-prg.fr/~ricardo.perez-marco/publications/articles/doublespend.pdf),
  sections 5-6: exact negative-binomial race analysis. Used only for the stated
  fixed-rate race, not an Ardents consensus or network-delay security theorem.
- [Bitcoin paper](https://bitcoin.org/bitcoin.pdf), sections 6, 8 and 11:
  honest resource assumptions, bounded scope of header/inclusion checking,
  and the distinction between invalid operations and alternative valid history.
- [RFC 8915](https://www.rfc-editor.org/rfc/rfc8915.html#section-8.6):
  authentication does not eliminate asymmetric delay; multiple sources require
  an applicable honesty assumption. We do not select NTS as the bootstrap answer.

## Results and disposition

Executed on **2026-09-06**, Python **3.12.12**, Windows 11 build **26200**,
repository HEAD **26c95e12448d527ef3971e384b70d0cde31df9f3** plus the existing
uncommitted research/contract changes. No existing implementation was modified.
The model compared **18 exact rational race probabilities** using independent
binomial and negative-binomial formulations and checked published rounded and
time/lease boundary values.

Calculator SHA-256:
`6986daa96362d79628ff28ec1a4523602dcc6b221e40ee73b94ca8539669ba17`.

External captured JSON:
`C:/Users/vitek/AppData/Local/Temp/ardents-r149-coupled-20260906/calculation-corrected.json`;
SHA-256 `2f2547a02b771826163e1a4fc98cd457feda22b7ef3705c0e694fbf5b82c12ce`.
The retained calculator reproduces the result if the temporary receipt disappears.
Its environment fields and file-byte hashes are provenance, not performance data.

The first output is retained externally as `calculation.json`
(SHA-256 `20406d208fe87d9164a8f2a748c6df7715fcddc2e17bcb77ff510ed244c47d36`).
Review corrected one model distinction: expiry of a signed Record's validity
can interrupt resolution without ending the Authority's separate Grace renewal
right. That initial classification is superseded, not presented as a product
failure. The predeclared input values and all other rejected profiles remain.

### Work, confirmation and freshness

**Calculation.** For hostile work fraction q<1/2 and z confirmations, the
fixed-rate concurrent-race probability is evaluated as
`2 * sum[k=z..2z-1] C(2z-1,k) q^k (1-q)^(2z-1-k)`.
The independent negative-binomial expression checks the same rational value.
A tie counts as catch-up success. The paper's model excludes adversarial
premining advantages, varying difficulty, partition and dissemination attacks;
therefore this is NOT a bound for every Ardents reorganization or censorship.
For q>=1/2, eventual catch-up is 1 under indefinite continued work.

| Hostile work share | 6 confirmations | 80 confirmations |
|---|---:|---:|
| 10% | 0.00059141216 | 2.50505e-37 |
| 30% | 0.15644958192 | 1.33779e-7 |
| 40% | 0.49300373504 | 0.0107197 |
| 49% | 0.94589545143 | 0.800568 |
| 51% | 1 | 1 |

Values are probabilities, not percentages. At q=30%, 69 is the first tested
confirmation depth meeting the 1e-6 comparison threshold. The frozen
80-confirmation profile passes this one per-race comparison, while six fails.
It is not absolute or lifetime finality: over 1,000 such races the union upper
bound is 0.000133779, and over one million it is 0.133779. These are loose upper
bounds conditional on the same model, not measured cumulative probabilities.
Correlated trials do not require independence for that union bound.

**Calculation.** With q=30% withholding all its blocks, surviving public honest
arrivals have rate 0.7/60 per second. Eighty arrivals take 114.29 minutes on
average, 146.09 minutes at p99 and 157.87 minutes at p99.9. Even the 180-minute
planning envelope is exceeded with probability 4.68754e-6 under ideal arrivals.
A 600-second inclusion allowance has probability 0.000911882 of seeing no honest
block at all, before scheduling or censorship. Neither is a hard upper bound.

**Inference.** The conservative cutoff-to-use allowance is
`180 + 5 + 10 + 4 = 199 minutes` (confirmation, Epoch rounding, acquisition,
combined clock margin). A 30-minute maximum State age fails. The four-hour
sensitivity leaves 41 minutes of arithmetic headroom, conditional on all
inclusion/time/data assumptions. It also tolerates older infrastructure facts
and delayed observed withdrawal; its product acceptability is unselected.
No clock value may reactivate state after its accepted freshness/credential
terminal bounds.

### Commit, reveal and Name lifecycle

**Calculation.** A five-minute E+1 cannot contain the 194-minute envelope for
finishing commitment confirmation, acquisition and clock margin before revealing.
The existing E/E+1 rule does not itself require final commitment before reveal;
there are two distinct candidate policies:

- Reveal against provisional included evidence, then expose no current Name
  until the complete claim/reveal outcome is committed. The 204-minute pipeline
  estimate is conditional; premature revelation/reordering and withheld data
  need an explicit security argument under the actual protocol.
- Require commitment finality before reveal. This requires a substantially
  longer Epoch (four hours is only an arithmetic comparison), and changes
  registration latency and possibly shared-state cadence. It is not selected.

**Calculation.** Renewal starts at day 23. The owner deadline is day 37, giving
14 days including Grace. The conservative fit condition is:

`censorship + inclusion + confirmation + rounding + clock margin < 14 days`.

| Continuous censorship | Conservative completion | Margin before day 37 | Outcome |
|---|---|---:|---|
| None | Day 23 + 3 h 19 min | 332 h 41 min | Fits declared envelope |
| 7 days | Day 30 + 3 h 19 min | 164 h 41 min | Fits; enters Grace first |
| 13 days | Day 36 + 3 h 19 min | 20 h 41 min | Fits narrowly |
| 14 days | Day 37 + 3 h 19 min | -3 h 19 min | Exclusive renewal cannot be promised |
| 21 days | Day 44 + 3 h 19 min | -171 h 19 min | Exclusive renewal cannot be promised |

**Inference.** A 13-day result is not a supported censorship guarantee: the
inclusion allowance, honest supply, time and finality tail still need evidence.
Once Grace ends, the selected shared transition may release/reclaim the Name.
A local timer cannot choose a claimant, but a later otherwise valid claim can
replace the old generation without forging the previous owner's signature.

The earliest applicable **parent Grace end** controls a child. If it is day 20,
a child following its own day-23 schedule is already too late even without an
attacker. A Record valid only to day 30 cannot supply resolution during Grace:
the day-30-plus-199-minute renewal may still be valid, but the old Binding
cannot bridge the 199-minute gap. A complete proof also has its own freshness.
Owner/parent lifecycle, signed Record validity, and proof currentness must not
be collapsed into one expiration check.

### Traffic, replay and proof budget

**Calculation.** Values include the declared traffic multiplier/reserve where
marked wire. Neither assumption is observed carrier overhead.

| Node / Name scale and Node refresh | Common deltas, modeled wire MiB/day | With hypothetical 8 KiB proof/5 min | 1-year common history transfer lower bound |
|---|---:|---:|---:|
| 1,000 / 10,000; 6 h | 14.469 | 17.844 | 132.05 s |
| 10,000 / 100,000; 6 h | 67.863 | 71.238 | 1,221.93 s |
| 1,000 / 10,000; 1 h | 43.766 | 47.141 | 730.07 s |

Current View bytes at 1,000 Nodes are only 0.488 MiB; sending the complete View
every five minutes costs 140.625 MiB/day before transport. A year's common input
history is 1.537 GiB at the smaller scale. Its **payload transfer alone** exceeds
the normal 15-second clean-start budget at 100 Mbit/s. Name bodies/history are
separately accounted, not imposed on an idle client or disclosed to every
Network producer.

At the small scale, a hypothetical 64 KiB validity proof every five minutes
raises modeled wire to 41.469 MiB/day; the nominal proof allowance is at most
25,561 bytes per refresh after the assumed other work. The 8 KiB example fits
the secondary traffic guardrail; **no such complete proof is implemented or
selected**. A Merkle inclusion proof, trusted snapshot, or unverified projection
cannot fill that slot. Privacy, time, publication, Release and Route traffic
cannot be silently omitted if the reserve is insufficient.

CPU/RSS, full verifier/prover costs, indexes, restart durability, data availability
and actual transport are UNKNOWN. The idle 1% CPU ceiling gives at most 3 core
seconds per 300 seconds to *all* charged work, not an entitlement for proof
verification alone. At 10,000 Names a simple 1,920-byte leaf + 14 sibling hashes
+ assumed 256-byte framing totals 2,624 bytes, leaving 1,472 in the current
4,096-byte envelope. This establishes neither lineage nor complete current-state
proof fit. The 127-record retained Namespace tracer is not a qualified
10,000-Name implementation.

### Hostile input and loss of honest supply

**Calculation.** At a nominal block per minute, 64 KiB per block permits 90 MiB/day
of aggregate input and 32.08 GiB/year before indexes/replication. An assumed
half assigned to common infrastructure alone permits 45 MiB/day. The Name
half carries 11,520 maximum-size updates/day nominally, or 8,064/day when only
the reference honest 70% publishes. With valid attacker input at 10 updates/s,
a 4,096-slot/16-MiB Name queue fills in 415.14 seconds even using the more
generous nominal service rate.

**Inference.** Finite caps protect a correctly enforcing host but grant no
individual owner an inclusion bound. Attackers can keep the pre-admission queue
full using many authorized objects; even an honest producer cannot derive
personhood from valid signatures. Separating renewal capacity helps only with
cross-class contention; hostile existing Name holders can fill that class too.
The 600-second assumption is therefore **not established** by these caps.

**Calculation.** Keeping q<=30% requires H/A >= 7/3 in actual scarce work.
With H=70 and A=30 fixed, loss of half the honest supply makes q=46.15%,
80-confirmation catch-up 0.330587 and mean honest confirmation time 228.57 minutes.
With 90% honest loss, q=81.08% and mean honest confirmation time is 1,142.86 minutes;
the honest-majority comparison is lost. With all honest work gone, there is
no honest-progress or freshness guarantee. Difficulty reduction cannot restore
the lost share; an actual adjustment protocol also needs separate validation.

No dollar cost, useful consensus throughput or independently funded honest
deployment has been measured. Work units cannot be equated to VPS count,
relay bandwidth or identity count. Lack of an internal reward does not bound a
censor's external benefit or expenditure.

### Time and combined counterexamples

**Calculation.** The interval half-width grows as
`30 seconds + 50 ppm * surviving monotonic elapsed time`.
It is 60.24 seconds after seven days, 159.6 after 30 days, and crosses the
120-second comparison limit after 20.833 days. This concerns a surviving,
bounded-drift clock; a hostile or reset clock supplies no such promise.

The cases below are **reasoned counterexamples/required outcomes**, not executed
Ardents tests or simulated cryptographic acceptance:

| Scenario | Concrete observation / consequence |
|---|---|
| Fresh client; all State and time sources collude | A six-hour-old valid transcript can be shown with time shifted back six hours. Without an independently justified initial time/currentness basis, the client cannot distinguish that view from timely delivery by examining those bytes alone. Refuse the currentness claim; several agreeing signatures do not fix it. |
| Routine restart after seven days powered off | A saved lower watermark has no finite current upper bound. The elapsed-clock calculation does not survive this loss of continuity. Fresh evidence is required; five-second readiness is unproved. |
| Replay on a running endpoint with surviving clock | A six-hour-old cutoff fails the proposed four-hour age bound. A malformed rival cannot advance/erase its accepted floor or manufacture conflict. |
| Time interval straddles day-37 Grace end | A local upper bound must not authorize work beyond the deadline. Uncertainty can refuse a Binding; it cannot materialize Released or select a new generation. |
| Partition produces two separately exposed committed branches | Rejoin must not silently resurrect an old Authority. A correct endpoint that obtains an actual conflicting committed proof retains floors and refuses the affected transition. An isolated endpoint need not detect the other branch; the race formula is inapplicable to this arbitrary-delay attack. |
| Source hides a Name successor while serving plausible headers | Inclusion of a different leaf proves neither complete projection nor latest binding. The absent bounded verification/availability argument is blocking. |
| Valid renewal acknowledged, withheld before admission | Receipt grants neither priority nor lease extension. The day-37 trace still fails if the owner cannot obtain admitted and committed evidence. |
| Resource loss plus queue flooding plus time-source loss | Confirmation and inclusion envelopes fail together; a wider grace interval does not restore currentness. Finite locally enforceable work can terminate, conditional on correct endpoint execution. |
| Hostile endpoint, key store and verifier | Correct local refusal and secret protection are not guaranteed. No result above presumes this combined case is protected by networking. |

## Recommendation and retained disposition

**Inference:** H1 has conditional arithmetic fits (small-scale deltas and the
four-hour freshness sensitivity), but no complete supported profile. H2 is
supported as a finite-envelope calculation and unbounded-omission counterexample.
H0 holds for production selection: real bounded validity/data/currentness proof,
admission under valid Sybil load, initial/restart time basis, honest resource
funding, and full costs remain missing.

Retain this calculator as evidence about its exact hypothetical profile. The
next comparison starts from product invariants and includes solutions without
PoW; this worksheet does not narrow the search to its proof/bootstrap shape.
Any proposed change to freshness, Epochs, reclaim or outside dependencies is
assessed under the owning contract. No C0 behavior, package, runtime dependency,
protocol, ADR or tracker issue is introduced by these calculations.
