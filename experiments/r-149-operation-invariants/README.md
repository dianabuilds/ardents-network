# R-149 operation and agreement counterexamples

Status: **executed finite analytical experiment with predeclared inputs**, 2026-09-06.
Question: which Ardents obligations can use local verification or convergent
fact replication, and which still need a justified common choice?

This is evidence for [R-149](../../docs/research/records/r-149-autonomy-transition.md#task-first-mechanism-comparison).
It is not a new research question, C0 implementation slice, public protocol,
cryptographic implementation, or a qualification of SCP/BEC/Ardents.

## Contract and hypotheses

The [product](../../docs/product/functional-map.md),
[threat model](../../docs/security/threat-model.md#hostile-environment-premise-and-residual-assumptions),
[State](../../docs/technical/network-route-node.md),
[Namespace](../../docs/technical/naming.md),
[private reachability](../../docs/technical/private-reachability.md), and
[ADR-0074](../../docs/adr/0074-target-non-administrative-public-operation.md)
remain authoritative. Go and current package ownership remain unchanged.

- **H1:** exact authenticated immutable facts and some independent operations
  have order-independent accumulation under stated constraints.
- **H2:** extending that accumulation to immediately final canonical claims,
  conflicting successor selection, or proof of latest state breaks a retained
  invariant or withholds progress.
- **H3:** a non-work quorum can meet finite intersection/removal conditions,
  but open participation, honest independence and configuration continuity
  are additional obligations.
- **H0:** none of the explored compositions establishes the complete public
  contract; partial fits are retained only for the obligations actually tested.

## Predeclared input and adversary cases

Symbols stand for already authenticated objects and asserted participants.
There are no signatures, hash primitives, fabricated validity proofs, clocks,
network calls or production interfaces. These models may refute a semantic
shortcut; they cannot certify a real verifier, protocol or operator.

| Case | Frozen comparison / required distinction |
|---|---|
| Duplicate immutable facts | All 24 permutations of four deliveries containing three distinct facts, with one duplicate, must give the same fact set. This says nothing about currentness or authorization of future operations. |
| Independent changes | Two bindings for distinct Names under unchanged, valid authority/lineage must commute. Introduce a shared ancestor change separately; independence cannot be assumed just from different strings. |
| Concurrent exclusive successors | Two distinct successors to revision 7 each pass against that predecessor; installing both as current is forbidden. First-local-arrival yields different outcomes for the two orders. |
| Concurrent root claims | Two initially eligible claims for one unclaimed Name, with ordinal 9 delivered before withheld ordinal 4. Publishing the first observed claim as final and then using the lowest received ordinal must reverse the answer. Existing E/E+1 admission/completeness evidence is deliberately absent. |
| Claimed wall-clock winner | An owner-supplied far-future timestamp wins a naive latest-timestamp rule. It is not authenticated global time or an authorized priority rule. |
| Parent and child | A child bound under parent generation 3 must stop resolving after that parent is released; parent generation 4 cannot reactivate that child. Accumulated evidence may converge while old records remain stored. |
| Local single-use grant | Parent capacity 8; 100 distinct key/request labels cannot produce more than eight allocations. Replaying one local spend identity creates no new allocation. This is not global per-person fairness. |
| Renew versus reclaim | Epoch inclusion, deadline and predecessor decide the permitted transition; union of a held renewal and a later-generation claim cannot independently make both current. A signed author timestamp cannot prove timely admission. |
| Fresh-source indistinguishability | Two worlds give a new reader the same transcript: timely delivery or a stale six-hour-old state plus equally shifted source time. No external currentness assumption is supplied. |
| Uniform quorum | Four participants a,b,c,d; each requires any 3-of-4 including itself. Enumerate every nonempty subset, single Byzantine deletion and crash set. |
| Weak majority | Five participants a,b,c,d,e; each requires any 3-of-5 including itself. Enumerate every single Byzantine deletion; overlap solely in a faulty participant is insufficient. |
| Disconnected choices | Groups a,b,c and d,e,f, each member requiring any 2-of-3 from its group including itself. Enumerate disjoint quorums. |
| Indispensable core | Anchor r has slice {r}; each of a,b,c,d requires {self,r}. Enumerate common intersection and loss of r. |
| Configuration turnover | Old 3-of-4 on a,b,c,d; new 3-of-4 on a,e,f,g. Each separately intersects, but enumerate aggregate old/new slice choices in the same undecided slot. |
| Held evidence and total compromise | Evaluate as explicit non-claims: valid evidence cannot force delivery, and the fully compromised local verifier need not enforce any result. |

## Falsification criteria before execution

- Refute a claim of order-independent current state with one legal permutation
  that changes the selected authority or violates generation/predecessor rules.
- Refute a claim of finalized uniqueness if two locally accepted claims cannot
  merge without revoking an exposed result; merely waiting forever is not
  bounded progress.
- For a candidate quorum configuration, report ordinary intersection, intersection
  after each single-fault deletion, original-quorum availability after removals,
  indispensable members, and explicit counterexample sets. These are structural
  checks, not a protocol safety/liveness proof.
- Never label evidence union, an owner's sequence number, a consistency proof,
  or a quorum of sources as complete fresh State without its separate argument.
- Mark a conditional pass only for the stated obligation. No Node count implies
  operator independence, and no artificial set configuration solves Sybil admission.
- Do not measure or infer runtime latency, CPU, memory, anonymity, maintained
  library suitability, real independence, or public availability from this model.

## Method and reproduction

A standard-library Python program enumerates the frozen small state spaces and
prints JSON. It also checks generated uniform quorum sets against the independent
cardinality condition, and validates each reported counterexample against its
stated predicate. The logical cases are deliberately simpler than any product
implementation. Pending/committed/current cryptographic evidence is not modeled.

Run from the repository root:

```powershell
python -B experiments/r-149-operation-invariants/analyze.py
```

Redirect receipts only outside the repository. Retain the human-written README
and disposable calculator here; no output, caches, keys or binaries belong here.
Current product gates are separate from semantic experiment consistency checks.

## Primary sources

Accessed **2026-09-06**:

- [Kleppmann/Howard, BEC](https://arxiv.org/pdf/2012.00472), sections 2-3:
  invariant-confluent replication under its cryptography/communication assumptions.
- [SCP paper](https://stellar.org/papers/stellar-consensus-protocol.pdf),
  sections 3-4 and configuration changes: slices, quorums, faulty-node deletion
  and quorum availability. The program enumerates these structural objects;
  it does not implement nomination, ballots or consensus.
- [RFC 9162](https://www.rfc-editor.org/rfc/rfc9162.html#section-11.3):
  a log can withhold inclusion and show inconsistent views. Audit evidence alone
  does not assign canonical Name authority.
- [Stellar Core](https://github.com/stellar/stellar-core): the current reference
  implementation describes C++20 and Apache-2.0 licensing; it is not selected
  as an Ardents dependency or a ready Go module.

## Results and disposition

Executed **2026-09-06** using Python **3.12.12** on Windows 11 build **26200**,
at repository HEAD **26c95e12448d527ef3971e384b70d0cde31df9f3** plus existing
uncommitted work. There were no network connections or cryptographic operations
inside the model. All participant and authority labels are synthetic.

Frozen input README SHA-256:
`d78022f1d849e46af942ac2bfdefa2e855b52535e1217dcec16d89845bb5e5bc`.
A copy is retained externally at
`C:/Users/vitek/AppData/Local/Temp/ardents-r149-invariants-20260906/predeclared-plan.md`.

Calculator SHA-256:
`f2825942877610fecc39d69faa2167da57902434093ce2f0df8298ed2ffd619e`.
The full JSON receipt is
`C:/Users/vitek/AppData/Local/Temp/ardents-r149-invariants-20260906/result.json`,
SHA-256 `3aec2ddd12b39d0c9189d775d5189b1e6d1327dbba3fee695f58ed2667641940`.
The retained program reproduces the semantic results if temporary files disappear.

### Semantic results

**Calculation:** all 24 positional permutations (12 distinct delivery sequences)
of three distinct immutable facts and one repeated delivery yield the same fact
set. Two changes to distinct bindings under fixed valid authority/lineage also
commute. This is a positive result for evidence accumulation and the stated
independence case only.

**Calculation:** two different successors to revision 7 each pass that initial
predecessor condition. Applying target-a first selects a; applying target-b first
selects b. Rejecting the second local arrival protects one local chain but does
not establish a common answer. Accumulating both as evidence is legitimate;
presenting both as a single exclusive current successor is not.

**Calculation:** observing root claim ordinal 9 and later the withheld ordinal 4
changes the locally chosen minimum. Independent replicas can each see one
claimant and their union has two. Thus immediate finalization on the received
subset is falsified. This does not manufacture an ordinal proof or replace
ADR-0017's admitted domain, E/E+1 relationship and complete-close evidence.

**Calculation:** a child bound to parent generation 3 stops resolving after
release of generation 3 and remains unavailable after parent reclaim as
generation 4. Stored child evidence may remain; a read must re-evaluate lineage.
This illustrates that fact retention and current authority are different
operations. It does not prove arbitrary parent/child changes are independent.

**Calculation:** a symbolic local parent budget of eight produces eight
allocations under 100 different labels, with no additional allocation on exact
replay. This checks the arithmetic of the local rule, not real concurrent
admission, fairness or Sybil exclusion.

**Counterexample / required evidence:** two distinct branches can each extend
the same known prefix. An append-only-consistency observation does not choose
which branch is authoritative. Similarly, timely and six-hour-stale worlds can
supply the same signed-history/time transcript to an isolated fresh reader.
The model establishes equality of those observations, not cryptographic forgery.
The renewal/reclaim row records the missing admission/generation/time evidence;
it is not an executed Namespace lifecycle implementation.

### Quorum enumeration

**Calculation:** five frozen configurations were enumerated, including every
single participant as the potentially Byzantine member. There were **1,133**
nonempty-subset evaluations in total, including fault reductions and independent
uniform-threshold cross-checks; this is not 1,133 network executions.

A quorum satisfies every member's local slice requirement. For a fault set B,
the structural safety comparison deletes B from nodes and all slices before
checking intersection; availability separately checks quorums in the original
configuration without the absent members. This follows the structural objects
from the SCP paper, not its ballot algorithm.

| Configuration | Original quorums | Ordinary intersection | Single-fault deletion cases breaking intersection | Loss sufficient to remove every original quorum |
|---|---:|---|---:|---:|
| Any 3 of a,b,c,d, including self | 5 | Yes | 0 of 4 | 2 members |
| Any 3 of a,b,c,d,e, including self | 16 | Yes | 5 of 5 | 3 members |
| Separate 2-of-3 groups abc and def | 24 | No | 6 of 6 | 4 members |
| Every participant depends on anchor r | 16 | Yes | 1 of 5, namely r | r alone |
| Aggregate old abcd / new aefg, both 3-of-4 | 27 | No | 7 of 7 | 3 members |

Explicit witnesses:

- **3-of-4:** any two original minimal quorums intersect in at least two
  identities; with at most one faulty identity, at least one common identity
  remains correct. Every single-member crash leaves the other three as a
  quorum, and no one identity belongs to every quorum.
- **3-of-5:** abc and cde intersect only at c. If c equivocates, the two groups
  can obtain conflicting simple threshold certificates without another member
  signing twice. This is a counterexample to that threshold shortcut, not an
  attack against an implementation of SCP.
- **Disconnected choices:** ab and de are disjoint quorums. More signatures
  inside either component cannot connect their decisions.
- **Mandatory anchor:** every original quorum contains r. Its disappearance
  prevents all original quorums; its Byzantine deletion leaves separate local
  quorums. Many peripheral identities do not remove the dependency.
- **Turnover:** the original abcd and new aefg configurations individually
  intersect. In their aggregate for one undecided slot, bcd and efg are disjoint.
  A shared member a and separately valid configurations are insufficient for
  transition safety. No protocol transition or overlapping-mode rule is selected
  by this example.

The removal count is for loss of *all* original quorums, not a promise that
every remaining participant progresses until that count. The receipt separately
reports whole-remaining-set and some-quorum availability after each single crash.
Structural intersection is necessary evidence for this approach, not a proof of
actual consensus correctness, bounded latency, open membership or honest
operator independence.

### Design consequences

**Inference:** separate the following responsibilities in the proposed public
design without adding new packages or changing the current C0 flow:

1. Local owner authorization, finite resource admission, exact Target verification
   and voluntary software adoption.
2. Bounded exchange of independently checked evidence. Safe accumulation does
   not authorize a current decision.
3. Common decision evidence for the remaining conflicts and complete current
   Candidate View: input cutoff/order, exclusive Name generations/successors,
   parent/recovery constraints, currentness, availability and retained floors.

The third responsibility is still conditional on a real mechanism and its
fault/admission/transition argument. The first two have useful partial solutions
without PoW or votes. This separation alone does not remove current State
dependencies from Route or publish a public protocol.

**Recommendation:** retain authenticated fact replication and owner-local
verification as task-specific design directions. Reject plain set union,
first arrival, sender timestamps, and standalone append-only logs as complete
solutions for exclusive current Names. Keep non-work quorum agreement as a
candidate for the narrow shared-decision obligation; do not select a fixed
committee, SCP, a global trust list or an external chain from this enumeration.
Its decisive open test is admission and replacement while retaining the required
intersection/availability and avoiding indispensable appointed participants.
Currentness, Name inclusion under censorship, proof cost and maintenance remain
separate gates. H1 is supported only for its frozen cases; H2 has explicit
counterexamples; H3 has partial structural fits. H0 remains the production
selection result, not a universal impossibility theorem.

The experiment and counterexamples are retained. No runtime dependency,
cryptographic primitive, protocol, new product guarantee or C0 issue is selected.
