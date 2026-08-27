# Horizon 5 — Security and Privacy Model Review

Status: **future product intent; Horizon 5 is closed and authorizes no current
research or implementation work.**

[Product scope](../scope.md) remains authoritative. This document records why
Horizon 5 may exist, what question it is expected to answer, and what evidence
would be required once it opens. It is not a committed roadmap, a promise of a
Shielded Route Profile, or permission to add speculative protocol, package, or
runtime seams during Horizon 4.

## Intent

Horizon 5 will use evidence from the first public Ardents product to decide
whether the security and privacy model remains honest, useful, and sufficient
under real operation. The review starts from observed traffic, failures,
concentration, abuse, operator behavior, and Qualification results rather than
from a desire to add anonymity mechanisms.

The intended result is a bounded, evidence-backed decision about three things:

1. which Interactive Route claims and limitations remain supportable;
2. whether one named Application job needs a distinct, stronger Route Profile;
   and
3. what public wording, implementation boundary, and independent Qualification
   are required for the selected outcome.

Horizon 5 is therefore a **claim and model review first**. A new Route Profile
is only one possible result.

## Entry conditions

Horizon 5 opens only when all of the following are true:

1. Horizon 4 has produced the first public product and the exact release has
   earned its stated Public Beta claims.
2. Public Beta has produced a usable body of operational and Qualification
   evidence covering real traffic shapes, failures, concentration, abuse, and
   operator behavior. The observation period, populations, exclusions, and
   missing data must be declared when the review opens; elapsed time alone is
   not evidence.
3. The current build, Route Profile, user-visible claims, known limitations,
   topology conditions, and effective operator families are identifiable from
   retained evidence.
4. A privacy-preserving evidence plan defines which observations may be
   collected, why they are necessary, how they are minimized, who may inspect
   them, and when they are deleted. Horizon 5 may not create a new metadata
   hazard merely to measure an old one.
5. The Product Owner explicitly opens a decision-relevant research question
   and its falsification criteria. R-005 may be reopened or superseded at that
   point; it does not enter the active queue in advance.
6. Any result that requires external users, independent operators, or
   independent security or privacy review waits until those participants
   actually exist in a declared role. The current one-to-one team does not
   stand in for them.

Stable Network operation may provide useful evidence, but Stable Network is a
promotion state for a running beta, not Horizon 5 and not an automatic entry
condition.

## Review question

The central question is:

> Does a named Ardents Application job need protection beyond the Interactive
> Route's Route Knowledge Separation, and can a distinct Route Profile provide
> a reproducible advantage against an exact adversary within acceptable user,
> operator, and network costs?

The review must also answer:

- Where did Public Beta evidence support, weaken, or contradict the current
  threat model and public wording?
- Which concrete user harm remains after the Interactive Route's qualified
  protections, and for which Application job does reducing it matter?
- Is the relevant adversary a Broad Traffic Observer, correlated infrastructure,
  active endpoint confirmation, a malicious Application, infrastructure
  seizure, or another precisely bounded actor?
- What observation or experiment could falsify the proposed protection?
- What latency, traffic, storage, CPU, memory, energy, concurrency, reliability,
  and operator-capacity costs are acceptable for that job?
- Can the product expose profile selection, unavailability, incompatibility,
  and failure without silent downgrade or misleading anonymity language?
- Does the proposed change reduce one risk by increasing fingerprinting,
  centralization, Sybil leverage, denial of service, metadata retention, or
  supply-chain risk elsewhere?

## Claim frame

Every retained or proposed privacy claim must state all five fields below.

| Field | Required Horizon 5 answer |
|---|---|
| Protected information | The exact origin, destination, relationship, timing, volume, activity, or other information whose exposure is being reduced. |
| Adversary | The exact observations, active powers, controlled roles, collusion, duration, and auxiliary knowledge available to the attacker. |
| Conditions | The selected Route Profile, endpoint and Application boundaries, operator-family assumptions, traffic population, platform, and failure conditions under which the claim applies. |
| Measurement | A precommitted experiment, baseline, metric, denominator, uncertainty treatment, and failure threshold that can distinguish an advantage from noise. |
| Honest limitation | What the adversary can still infer or force, which environments are excluded, what cost is paid, and which failures cannot be distinguished from attack. |

Payload encryption, multiple hops, multiple Node IDs, decentralized storage, or
added traffic are not substitutes for this frame.

## Review sequence

### 1. Evidence intake and claim audit

Bind the review to an exact Public Beta candidate and inventory the evidence
that exists, the evidence that is missing, and any observations that cannot be
used safely. Compare actual behavior with the accepted
[threat model](../../security/threat-model.md), public product wording, Route
Qualification, and effective operator-family assumptions.

The audit may reaffirm a claim, narrow it, withdraw it, or identify a concrete
unresolved harm. A deployed claim is not protected from correction by
compatibility or marketing cost.

### 2. Application job and adversary selection

Select one complete Application job before comparing stronger mechanisms. The
job must identify the User, Application behavior, protected information,
adversary, consequence of exposure, expected traffic pattern, acceptable delay,
and reason the Interactive Route is insufficient.

A generic wish for "more anonymity" or a mechanism looking for a use case does
not pass this step. Different jobs may need different profiles and must not be
collapsed into a universal privacy mode.

### 3. Falsifiable comparison

Define the baseline, candidate treatments, observer positions, active attacks,
traffic populations, metrics, thresholds, resource budgets, and stop criteria
before running an experiment. Candidate mechanisms may include timing
re-shaping, padding, cover traffic, batching or mixing, multiplexing, multipath,
or combinations of them. None is selected by this document.

Disposable experiments must follow the repository research discipline. Raw
captures or datasets containing sensitive metadata remain outside the
repository; durable records retain minimized, reproducible measurements and
their provenance.

### 4. Product and system decision

Compare measured protection with the complete cost and new attack surface. The
decision must cover profile discovery and authentication, Application-facing
behavior, capability mismatch, overload, recovery, observability, operator
requirements, and explicit failure. The existing Application Interface and
Service Connection contract should remain stable unless evidence justifies a
separate product decision.

Selecting a technology, public wire protocol, runtime dependency, or other
meaningful lock-in requires a completed research record and an accepted ADR.
A decision to explore or select a profile does not by itself qualify an
implementation or strengthen a public claim.

### 5. Independent qualification and publication

A selected implementation candidate receives its own Qualification plan and
Evidence Bundle. It must be tested independently from the Interactive Route,
including downgrade, fingerprinting, tagging, active confirmation, collusion,
resource exhaustion, partial deployment, recovery, and unsafe Application
escape where applicable.

Public product and security documentation then state the strongest supported
claim and its limitation. Horizon 5 does not retroactively strengthen the
claims of an earlier Public Beta build.

## Gate for a Shielded Route Profile

A Shielded Route Profile may enter a bounded implementation brief only if the
review establishes all of the following:

- one named Application job and a demonstrated user harm;
- exact protected information, adversary, conditions, measurement, and honest
  limitation;
- a reproducible improvement over the qualified Interactive Route baseline,
  including uncertainty and negative results;
- finite and accepted latency, traffic, compute, memory, energy, concurrency,
  reliability, and operator-capacity budgets;
- explicit authenticated profile selection, with unsupported or unavailable
  protection failing visibly rather than falling back silently;
- no unsupported claim that one mechanism defeats all traffic analysis,
  collusion, endpoint compromise, or Application identification;
- a maintained implementation boundary and dependency decision justified by
  current evidence; and
- an independently reviewable Qualification plan with actual independent
  participation required before the public claim is promoted.

If any input is absent, the profile remains unselected. Preserving the Route
Profile seam is sufficient preparation during earlier horizons.

## Valid outcomes

Horizon 5 is successful if it produces an honest decision, including any of
these outcomes:

1. **Retain the Interactive Route only.** Evidence does not justify another
   profile, so its qualified claims and visible limitations remain.
2. **Correct the baseline contract.** Evidence requires narrower wording,
   remediation, requalification, or withdrawal of an Interactive Route claim.
3. **Select a bounded Shielded Route Profile.** One job and adversary justify a
   separately budgeted, implemented, and qualified profile; it does not silently
   replace the Interactive Route.
4. **Reject or stop a candidate.** Its advantage is not measurable, its cost or
   new risks are unacceptable, or independent Qualification is unavailable.

A negative result is evidence, not a failed horizon.

## Required durable outputs

Once Horizon 5 opens, its closure should leave:

- a research record created from the current research template, with sourced
  facts, measurements, assumptions, recommendations, and predeclared
  falsification criteria kept distinct;
- the exact minimized evidence inventory and reproducible comparison results;
- updated product claims, threat model, functional requirements, and glossary
  terms for every accepted change;
- an ADR for any consequential selected profile or technology lock-in;
- a bounded implementation and Qualification brief if a new profile is
  selected; and
- independent review provenance and unresolved findings for every claim that
  depends on external Qualification.

## Non-goals

- Scheduling Horizon 5 work before its entry conditions pass.
- Treating Stable Network promotion as a security-model redesign.
- Promising blanket anonymity, unlinkability, unobservability, or protection
  from a global observer.
- Preselecting padding, cover traffic, mixing, multiplexing, multipath, a
  protocol, a dependency, or an operator economy.
- Adding placeholder packages, speculative interfaces, wire fields, or runtime
  cost to Horizon 4 for a possible future profile.
- Treating encryption, raw Node count, or multiple logical positions as proof
  of independent control or traffic-correlation resistance.
- Collecting production metadata without a necessary, minimized, consented,
  access-controlled, and deletion-bounded evidence purpose.
- Hiding negative results, failed cells, unavailable reviewers, or unacceptable
  resource cost behind a stronger product label.

## Decisions deliberately left open

The following decisions belong to the future review and are not fixed now:

- the first Application job, protected information, and adversary;
- metrics, thresholds, traffic populations, observer placements, and experiment
  duration;
- candidate mechanisms and implementation architecture;
- resource budgets and whether a stronger profile could be default, opt-in, or
  unsuitable for release;
- operator-capacity, incentive, and deployment implications;
- external reviewers and independent test environments; and
- whether Ardents should ship a Shielded Route Profile at all.
