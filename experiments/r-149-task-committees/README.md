# R-149 temporary task committee assessment

Status: predeclared analytical inputs, 2026-09-07. Results are appended after
execution; this section is retained as the frozen plan.

Question from [R-149](../../docs/research/records/r-149-autonomy-transition.md):
can a randomly selected committee for one decision preserve the public authority
boundary while excluding nonresponses from the tally?

The Product Owner proposed 100 active participants, a random temporary subset,
and a provisional 24-hour voting window. The supplied comparison additionally
suggests 15 members, 10-member turnout, fresh draws after failed turnout,
variable sizes and two disjoint committees. These are candidates, not accepted
requirements. Human deliberation versus automatic protocol votes is unselected.
The [accepted authority boundary](../../docs/adr/0074-target-non-administrative-public-operation.md)
still excludes general rule-changing, forced installation and Name seizure.

## Hypotheses and falsification criteria before execution

- H1: finite uniform sampling can reduce the probability of selecting a hostile
  majority when the fixed eligible population has a sufficiently small hostile
  fraction. Reject any unconditional capture-resistance claim if pool admission,
  randomness or adaptive compromise is outside these assumptions.
- H2: deleting nonresponses from the denominator permits a minority of selected
  members to prevail; turnout alone can also produce contradictory local results.
  Exhibit exact vote sets or reject this hypothesis for the frozen examples.
- H3: a fixed certificate threshold can have correct-member intersection under
  an explicit fault bound. This is a structural condition, not a consensus
  protocol, guaranteed availability or proof of a complete deadline tally.
- H4: fresh repeated draws change lifetime risk; disjoint committees are not
  independent samples. Compare exact conditional sampling against squaring the
  one-committee probability.
- H0: no complete public protocol is selected by these arithmetic tests.

## Frozen inputs and method

Use Python standard-library exact integer combinations and rational arithmetic.
For N eligible labels, K hostile labels and n distinct sampled labels:

`P(X = k) = C(K,k) C(N-K,n-k) / C(N,n)`.

Report all nine combinations of N=100, K in {10,20,30}, n in {9,15,21}.
For each, report strict hostile-majority probability and probability of
exceeding `f=floor((n-1)/3)`. The latter is loss of the illustrative Byzantine
fault envelope, not the probability of an actual attack succeeding. Uniform
selection is without replacement within a committee. Labels, hostile counts,
unbiased randomness and authenticated votes are assumptions, not measurements.

Independently cross-check the distribution by enumerating all subsets of size
3 from 8 labels, three hostile; check normalization and expected hostile count
for every distribution used. No Monte Carlo simulation is necessary.

Additional frozen cases:

1. n=15, two YES, one NO, twelve unavailable: responder majority passes 2/3.
2. n=15, six hostile YES, four honest NO, five honest unavailable: turnout 10
   plus responder majority passes 6/10, despite six being a selected minority.
3. Fifteen non-equivocating votes, YES labels 0..6 and NO labels 7..14. Reader A
   sees seven YES and three NO; reader B sees two YES and eight NO. Both see
   turnout 10 at their local deadline. No proof of a common complete close is
   assumed. Reject contradictory finality based solely on these views.
4. n=15, fixed threshold t=10, assumed f=4: check `2t-n > f` and `t <= n-f`.
   With five equivocators construct conflicting 10-signature certificates.
   With four withholding members and two additional unavailable members,
   nine remaining votes cannot meet t. Missing is not a signed NO.
5. N=100,K=20,n=15: majority tails for 1,100,500,1000 independent fresh draws from
   the unchanged pool; report chance of at least one hostile majority. This is
   exposure if redraws are actually obtainable, not proof an attacker can force
   every retry. Conditional retries need a separate protocol model.
6. Same population: exact probability of hostile majorities in BOTH sequential
   disjoint committees of size 15; compare with the independent-draw square.
7. N=100,K=20,n=15: count expected sampled hostile labels. Adding identities,
   suppressing heartbeats, post-selection bribery/corruption or a controllable
   selection seed invalidates the fixed-pool calculation, rather than being
   silently priced into it.

## Nonclaims and design checks

No real signatures, VRF, beacon, clock, network, voting protocol or deadline
engine is implemented. No latency, availability or independence is measured.
Twenty-four hours is a discussion input, not a global authenticated clock or
permitted Name-renewal delay. Local absence does not establish non-submission;
late old certificates cannot be discarded merely because a fresh draw started.
Owner authorization, opaque Name ordering, common admitted View, freshness,
bounded queues and explicit refusal remain separate obligations.

For a candidate design require exact decision/conflict identity, predecessor,
rules and class, pool snapshot and admission evidence fixed before randomness;
verifiable unbiased selection without requester-controlled rerolls; fixed
certificate/locking rules; bounded evidence acquisition; a proved transition
between attempts; and an explicit no-decision result when evidence is inadequate.
Classifying a hard task as routine must not lower its checks. Publicly announcing
the committee for 24 hours exposes a target; hidden eligibility only mitigates
this if its actual protocol and anonymity assumptions hold.

## Sources and reproduction

Accessed 2026-09-07:

- [Algorand paper](https://eprint.iacr.org/2017/454.pdf), introduction and
  sections 5.1-5.3: private verifiable sortition, weighted eligibility, seed/key
  timing and targeted attacks. Its stake assumptions, variable sampled weight
  and full protocol differ from our fixed equal-label arithmetic.
- [drand security model](https://docs.drand.love/docs/security-model/): a
  randomness beacon has threshold, compromise, withholding and availability
  assumptions. It is not selected as a dependency or an admission mechanism.
- [Current naming contract](../../docs/technical/naming.md#claim-and-epoch-boundary)
  and [threat model](../../docs/security/threat-model.md#hostile-environment-premise-and-residual-assumptions).

Run `python -B experiments/r-149-task-committees/analyze.py` from the root.
Capture JSON outside the repository. Retain this disposable calculator and the
human-written result; generated receipts are not maintained product code.

## Additional predeclared clarification

The Product Owner subsequently proposed an immutable proposal, future beacon,
fixed committee, absolute YES threshold, no same-proposal reroll and a new
version/cooldown for retry. Evaluate this as a refinement, not an adopted ADR.
The 500-draw row also models 500 precommitted distinct inputs if each obtains a
uniform independently derived selection. Removing a nonce or committing before
the beacon does not itself bound how many valid commitments are admitted.

Check the distinction between proposal identity and the identity of a mutually
exclusive decision over one predecessor; natural-language rephrasing cannot be
assumed to produce the same digest. A missing certificate at local timeout is
not a proof that no certificate exists elsewhere. No automatic final rejection,
unsafe replacement or expiry of owner rights follows from local timeout. A
second turnout threshold equal to or below the absolute acceptance threshold is
redundant for approval; signed ABSTAIN and missing remain distinct observations.

## Follow-up predeclaration: acceptance versus rejection

After the first arithmetic run, the Product Owner proposed separate monotone
acceptance and rejection certificates, including YES >= 2/3 and NO >= 1/3.
Before evaluating this refinement, freeze these additional cases:

- n=15,A=10,R=5: exhibit simultaneous certificates without any double voter.
- n=15,A=10,R=6: test one equivocator and otherwise disjoint correct voters.
- n=15,A=10,R=10,f=4 and n=21,A=14,R=14,f=6: check minimum intersection.
- n=21,A=14,R=7: test whether disjoint honest votes can certify both outcomes.
- Enumerate all nonempty acceptance/rejection subset pairs for n=6 and verify
  that the minimum intersection for each pair of sizes is max(0,A+R-n).

For the modeled certificate-only rule, correct voters sign at most one outcome
for one fixed conflict instance, whereas up to f faulty voters may sign both.
Opposite certificate safety requires A+R > n+f. This says nothing about whether
correct voters eventually agree, sign, or deliver. A protocol with different
locking, external close or one-way certificates needs its own argument.
Local timeout/inconclusive must not be reported as final rejection or as proof
that no valid certificate has already formed elsewhere.

## Follow-up predeclaration: willingness and turnout

The Product Owner then raised honest nonparticipation: operating a Node does
not imply a human will review or vote. Evaluate availability without treating
nonparticipation as dishonesty or introducing governance powers.

Before execution freeze n=15, approval threshold A=10, and independent response
probabilities p in {0.2,0.5,0.8,0.95}. For each p calculate P(at least 10 responses)
from 15 honest selected members and from 11 honest members when four selected
faulty members withhold. Assume EVERY received honest response supports the same
valid proposal. These probabilities are upper bounds for obtaining approval
under this simplified response model, not real turnout measurements or a full
agreement liveness theorem. Correlated outages and preference disagreement are
excluded, so the figures must not be used as a qualification or a prediction.
Check exact binomial normalization and mean. Compare a finite n=4,p=0.5 case
against all 16 equiprobable response patterns.

Also evaluate deterministic self-selection from an initial 100 labels with
20 hostile: all 20 hostile volunteer, and 80,40,16 honest labels volunteer.
Report resulting hostile fractions. This is a counterexample to assuming that
an opt-in subset inherits the whole population's honest fraction, not an
empirical model of volunteer behavior. No proof of independent humans exists.
Growing the original population alone does not change the response probability
for a fixed-size committee with unchanged p. Expected per-person assignments
are D*n/M for D admitted tasks and M equally sampled eligible members; whether
those variables or p improve or worsen at scale is unknown.

## Follow-up predeclaration: availability filter and fixed alternates

The Product Owner supplied a two-phase candidate/availability/final-jury design,
standing voluntary availability, participation-reliability filtering and ordered
alternates before voting. Freeze these counterexamples before calculating:

- Forty initial candidates: eight hostile and 32 honest. All eight hostile and
  only eight honest confirm readiness. Select 15 uniformly from the 16 replies.
  Compute hostile-majority probability and loss of the f=4 fault envelope. This
  is a constructed asymmetric-response case, not a claim about typical turnout.
- Primary labels 0..14, of which 0..2 are hostile. Ordered alternates 15..24,
  of which 15..19 are hostile. Suppress readiness from honest primary labels
  3..7. Apply the fixed first-five alternate rule and count hostile final seats.
  No reroll or forged signature is available to the attacker in this example.

These cases test the claims that confirming availability or fixing alternates
makes selection immune to manipulation. They do not test whether future hidden
selection reduces targeting under a specified communication adversary. The
availability-close and second seed must have independently justified common
admission/currentness; a signed acknowledgement proves a statement, not its
completeness, future response or an independent human. Reliability exclusion
based on missing messages can exclude censored honest members. Equal weight per
selected vote does not make unequal eligibility frequency politically neutral.

## Results and disposition

Executed 2026-09-07 with Python 3.12.12 on Windows 11 build 26200, root HEAD
`26c95e12448d527ef3971e384b70d0cde31df9f3` plus existing uncommitted work.
These are exact combinatorial calculations and constructed counterexamples,
not empirical human-turnout estimates, network simulations or qualification.

### Random selection sensitivity

Percentages below assume the declared immutable, uniformly sampled population.
The second tail is loss of the illustrative fault assumption, not observed
attack success. A low majority probability cannot replace the fault-tail bound.

| Hostile of 100 | Committee | Assumed f | P(hostile strict majority) | P(hostile > f) |
|---|---:|---:|---:|---:|
| 10 | 9 | 2 | 0.035173% | 4.448047% |
| 10 | 15 | 4 | 0.000135% | 0.631392% |
| 10 | 21 | 6 | 0% | 0.056878% |
| 20 | 9 | 2 | 1.469881% | 25.601751% |
| 20 | 15 | 4 | 0.179744% | 14.691075% |
| 20 | 21 | 6 | 0.015091% | 8.275503% |
| 30 | 9 | 2 | 8.858371% | 54.279439% |
| 30 | 15 | 4 | 3.675317% | 48.819161% |
| 30 | 21 | 6 | 1.401822% | 44.914746% |

At N=100,K=20,n=15, the chance of at least one hostile majority over 100,500,
1000 independent admissible draws is respectively 16.465106%,59.323871%,
83.454525%. This is NOT the chance of collecting ten malicious YES signatures,
nor proof that all retries are available. Many precommitted proposal variants
can create this exposure if their independently derived draws are all admitted.
Two sequential disjoint juries both having hostile majorities has probability
0.000007771%; squaring the one-jury probability gives 0.000323079%. Disjoint
finite-population samples are dependent. Neither formula covers adaptive
corruption, suppression, preference changes or a two-stage finality protocol.

### Certificates and closing

The 2 YES/1 NO/12 missing and 6 hostile YES/4 honest NO/5 missing cases both
pass responder majority. The latter also meets turnout ten. Two local readers
of non-equivocating votes see 7:3 and 2:8 with turnout ten each and disagree.
A common closed tally cannot be inferred from their local 24-hour deadlines.

For n=15,A=10,R=5, ten honest YES and five honest NO certify BOTH outcomes.
Raising R to six still permits two certificates with one double voter.
Under the declared certificate-only rule with f possible equivocators,
opposite-outcome safety requires `A+R > n+f`; two conflicting approved values
similarly require `2A > n+f`, plus correct voters' exclusive signing across the
same conflict instance. n=15,A=R=10,f=4 and n=21,A=R=14,f=6 pass the opposite
intersection condition. They are conditional examples, not chosen settings.
With five equivocators, conflicting ten-signature certificates are constructed
for n=15. With four withholding members and two additional unavailable members,
only nine votes remain: safety does not supply progress.

Local timeout is a pending/unavailable observation. It cannot revoke a remotely
formed certificate or establish final expired/rejected status. Any such closure
and any subsequent attempt must be justified by a protocol that preserves old
locks/certificates and current state. Receipt time is not an authenticated
submission cutoff. A turnout threshold <= A adds nothing to approval.

### Willingness and selection bias

These capacity probabilities assume independent response AND every responding
honest voter supports the same proposal; real approval can be lower.

| Honest response probability | At least 10 of 15 honest respond | At least 10 of 11 honest respond; four selected faulty withhold |
|---|---:|---:|
| 20% | 0.011323% | 0.00009216% |
| 50% | 15.087891% | 0.585938% |
| 80% | 93.894857% | 32.212255% |
| 95% | 99.994719% | 89.810541% |

Honest willingness is not measured here. Independent response is an explicit
simplification; shared outages, timing and common incentives can correlate it.
Population growth alone does not lower the fixed-committee probability at fixed
response rate. Node operation cannot be treated as a human's voting commitment.

From 100 initial labels including 20 hostile, all 20 hostile volunteering while
80,40,16 honest volunteer produces hostile shares 20%,33.333333%,55.555556%.
A larger willing population is not by itself a safer population.

The two-phase example starts with 8 hostile of 40 candidates. Eight hostile
and eight honest acknowledge availability. A uniform final 15 of those 16 has
seven or eight hostile members: 50% probability of hostile majority and 100%
probability of exceeding f=4. Readiness can improve expected response while
worsening the committee's fault distribution. The ordered-alternate example
moves from three hostile of 15 to eight hostile of 15 by suppressing five honest
acknowledgements and admitting five prelisted hostile alternates, without any
reroll. It is an existence counterexample, not its frequency under random order.

Missing-message reliability penalties can remove censored honest participants;
signed readiness proves neither future conduct nor why a later reply is missing.
Equal per-vote weight does not remove influence from eligibility frequency.

### Validation and evidence retention

All assertions passed. Independent small oracles checked 56 single subsets,
420 ordered disjoint pairs, 3969 opposite-certificate pairs and 16 response
patterns. No real signatures, beacon, committee selection implementation,
networking, cryptographic primitives or human experiment was executed.

Receipts remain outside the repository under
`C:/Users/vitek/AppData/Local/Temp/ardents-r149-committees-20260907/`.
Each follow-up was predeclared before its execution; earlier source/plan/JSON
receipts remain there. The retained source is the final combined calculator.

| Final evidence | SHA-256 |
|---|---|
| analyze.py | 58e7fe19a2d8219d4e9085d99a9fb28f4e3837e9b75207fdb169ed019fdd57c8 |
| predeclared-plan-v4.md | 6633108008f1400422964791b4cb71a15149164778299c16e5253021a0b94469 |
| result-v4.json | f555d2927a0a162c3351486e51fb59c0da6ae38cb105e8dc581b7ea35fd5bcf3 |

Retain the idea of scoped random committees as a conditional candidate. Reject
responder denominators, the 2/3 YES versus 1/3 NO finality rule, automatic reroll,
and assuming availability filters/alternates preserve the original fault share.
Do not select the composed public protocol, human governance powers, a beacon,
identity system, punishment policy or runtime implementation from this evidence.
