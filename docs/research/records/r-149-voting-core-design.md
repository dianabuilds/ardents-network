---
parent_question: R-149
title: Participant selection and voting core - composition assessment
status: open
owner: Product Owner and Codex
started: 2026-09-07
reviewed: 2026-09-07
---

# R-149 companion - participant selection and voting core

This focused assessment belongs to [R-149](r-149-autonomy-transition.md).
It is a design candidate and evidence record, not a second active question,
selected runtime specification, implementation authorization or live task ledger.
The [work-package map](r-149-voting-work-packages.md) derives conditional later
development and verification work; its implementation gate is currently closed.

## Decision this unlocks

Determine whether one coherent reusable selection/voting core can meet Ardents'
authority, privacy, resource and recovery constraints before implementing parts.
Application scenarios attach later; a Node restriction does not define this core.
Produce a dependency-complete specification and implementation/test tasks only
when the critical choices below are actually resolved.

## Current contract

Read [scope](../../product/scope.md), the
[public authority boundary](../../product/operating-model.md#public-autonomy-target),
[participation requirements](../../product/operating-model.md#participant-selection-and-public-voting),
[threat model](../../security/threat-model.md#hostile-environment-premise-and-residual-assumptions)
and [handoff gate](../../development/documentation.md#research-to-implementation-handoff).
[Network State](../../technical/network-route-node.md) retains its current owner;
this design cannot write its root or replace its proof through an unchecked result.

**Product Owner decisions, 2026-09-07:** no cryptoasset or cryptoasset bond for
participation in this system. Investigate removal of the advantage obtained by
creating Sybil identities. Publicly checkable votes under separate pseudonyms
are acceptable; real-world identity must not be disclosed. Ordinary network
participation does not automatically confer general voting rights or require
voting work. These choices do not select PoW, a public electorate or an algorithm.

**Inference:** absence of monetary rewards does not establish absence of attack
value. Censorship, obstruction and favorable outcomes can motivate spending.
The achievable anti-splitting requirement is narrower than one-person-one-vote:
relabeling the same accounted resource must not increase total influence.
The actual resource and verification of its scarcity remain open.

## Hypotheses

- H1: a bounded evidence-producing core can serve multiple declared task types
  without acquiring their effect authority or a general network command.
- H2: admitting and sampling indivisible, verifiably non-duplicated resource
  units could make identity splitting neutral at a fixed accounted allocation.
  This neither proves fair hardware access nor independent humans; a concrete
  resource/proof and its common admission boundary must substantiate the claim.
- H3: a fixed verifiable draw and immutable signed choices can give compatible
  same-instance certificates, but require an additional close/continuity rule
  before retries or replacement committees can safely affect the same conflict.
- H4: public pseudonymous voting simplifies ballot verification compared with
  secret tallying, but leaves membership exposure, coercion and correlation risks.
- H0: no examined complete composition yet meets all requirements. It must stay
  unready rather than relying on a trusted placeholder or changed product claim.

## Evaluation criteria

The comparison requires no indispensable appointed authority, no cryptoasset,
no personal-data/owner-secret acquisition, explicit surviving assumptions,
identity-splitting analysis, compatible current result proofs, finite offered
work and recovery, and maintenance for one Product Owner plus Codex. An API or
library label cannot satisfy a missing admission, close, freshness or privacy proof.

The first candidate should support bounded binary choices and explicit no-result
observations. Ranked ballots, delegation, reputation, rewards, automatic reserves,
multiple review juries and binding application powers are not default features.
This is a scope recommendation, not a prohibition on researching later extensions.

## Evidence plan

### Primary sources

All sources accessed on 2026-09-07. Repository READMEs and mutable documentation
establish candidate responsibilities only; no version, audit status or measured
performance is inferred. A later selection needs pinned-source assessment.

| Source | Sourced fact used here | Ardents inference / limit |
|---|---|---|
| [Douceur, The Sybil Attack (2002)](https://www.microsoft.com/en-us/research/wp-content/uploads/2002/01/IPTPS2002.pdf) | The paper examines identity multiplicity and strong assumptions needed to establish distinct entities without a trusted certification authority. | Do not equate distinct keys with independent operators or infer complete Sybil elimination from this design. Its model is not a proof against every resource-based influence protocol. |
| [CometBFT](https://github.com/cometbft/cometbft) | BFT state-machine replication, application separation through ABCI, Go foundation and Apache-2.0 licensing; the project warns against production use of its main branch and describes upgrade compatibility limits. | Candidate for common admission/order/close. It does not select our eligible population, jury, fairness or effect authority. No release or performance claim selected. |
| [SmartBFT](https://github.com/hyperledger-labs/SmartBFT) | Go BFT state-machine replication library, Apache-2.0. | A smaller integration candidate to assess beside CometBFT; application transport/storage/membership work and release support need concrete review. No claim that it is cheaper for Ardents yet. |
| [drand security model](https://docs.drand.love/docs/security-model/) | DKG/group formation, threshold/key assumptions, timing and selective withholding affect beacon operation; the documented setup includes a coordinator trust assumption. | A beacon does not fix eligible membership or delivery. An appointed group cannot silently become the sole indispensable public selection source. External use adds dependencies; self-hosting still needs qualified open membership/setup. |
| [RFC 9381](https://www.rfc-editor.org/rfc/rfc9381.html) | VRF outputs have publicly verifiable proofs under specified keys/inputs; security depends on key-generation assumptions and proofs do not hide their inputs. | Useful selection primitive to evaluate, not proof of a complete selected set, Sybil resistance or anonymous participation. No implementation selected. |
| [Kleros whitepaper](https://kleros.io/whitepaper.pdf) and [PNK documentation](https://docs.kleros.io/pnk-token) | The described court uses stake-weighted selection and economic incentives/fees. The PNK page was available in the search index; direct access returned 404. | Whole-system adoption fails the no-cryptoasset boundary. Removing the stake mechanism requires a new argument rather than inheriting the original guarantees. No live contract audit performed. |
| [Belenios role instructions](https://www.belenios.org/instructions.html) | Verifiable elections use voter credentials, election administration and decryption trustees; the documented setup distributes credentials and permits revoting. | Useful comparison for ballot verification, but the documented whole deployment does not supply our open autonomous admission or no-secret-custody boundary. Secret-tally machinery is not required by the selected public-ballot target. |
| [RFC 8949, section 4.2](https://www.rfc-editor.org/rfc/rfc8949.html#section-4.2) | CBOR defines deterministic encoding rules and application-specific representation decisions. | Candidate grammar only; signing needs one exact encoding and bounds. Existing Ardents binary contracts and a restricted CBOR profile must be compared before selection. |
| [RFC 9106](https://www.rfc-editor.org/rfc/rfc9106.html) | Argon2 is a memory-hard function with specified parameters and test vectors. | A possible resource-cost building block, not ready admission proof, cheap verification, operator fairness or a reason to add a dependency. |

### Experiment

The [predeclared envelope experiment](../../../experiments/r-149-voting-envelope/README.md)
checks exact finite sampling, response capacity, volunteer filtering, workload
and cross-jury conflict. It implements no cryptography, peer delivery or people.
The earlier [committee experiment](../../../experiments/r-149-task-committees/README.md)
remains evidence for proposal grinding, reserves and unsafe deadline examples.

## Findings

### Identity splitting and resource accounting

**Proposed requirement:** for a fixed set of valid resource evidence in one
admission context, partitioning it among more keys must not improve the total
selection distribution, voting weight, proposal allowance, retries or priority.
The full distribution matters, not only expected seats. One-free-attempt-per-key,
per-key caps, entry bonuses and concave per-key weighting all require scrutiny.

**Inference:** for illustration, a per-key influence cap of 10 gives one key
with 20 accounted units only 10, but two keys holding 10 each get 20. Removing
that cap alone does not prove safety; all other admission and selection paths
must also ignore how the same allocation is partitioned.

Compare these resource candidates before choosing:

| Candidate | Possible benefit | Unresolved or rejecting condition |
|---|---|---|
| Verifiable computational work bound to a purpose and admission context | Account valid distinct work instead of counting keys; asymmetric proof checking is a useful target. | Hardware/rental concentration, energy and endpoint cost; fresh challenge, copying, precomputation and aggregate verification DoS. Does not require a blockchain, but is still a form of PoW if selected. |
| Memory/storage commitment and challenge | Investigate a different scarce resource and cost distribution. | Duplication/outsourcing, challenge authority, proof/verification cost and available reviewed implementations. Owning storage or reporting capacity is not proof of unique ongoing allocation. |
| Useful relay contribution | Align admission with provision of something the network needs. | Colluding peers can fabricate receipts or circulate their own traffic; no accounting of private user traffic or relationship graph is allowed. Uptime and reported bytes cannot be imported as voting weight. |
| Key age, activity and willingness only | Low explicit participation cost. | Does not meet the anti-splitting requirement: keys can age together and attacker-controlled identities can acknowledge each other. Useful local operational signals only. |
| Real-world identity or appointed admission | Could impose a chosen one-entry policy. | Does not fit the selected personal-data/autonomy boundary as a default. Not adopted as the missing resource oracle. |

**Recommendation:** investigate purpose-bound resource admission first, comparing
actual computational and memory/storage proofs. Do not yet choose a resource,
weight formula or monetary value. A committee of sampled resource units must not
be advertised as a committee of distinct people; grouping multiple selected
units under one pseudonym cannot erase their weight or increase it by splitting.
Additional verified allocation may increase influence if that policy is selected;
that consequential trade-off is still a decision, not implied Product Owner assent.
Key binding also cannot guarantee that an owner never lends keys or outsources work.

### Selection and willingness

**Recommendation to evaluate:** commit a finite admitted proposal and eligible
snapshot before a predetermined future randomness round; select once; retain
the original denominator. Bind any standing willingness/workload commitment
before the relevant draw. Start the reference comparison without per-task
replacement, reliability penalties or reserve promotion. These features need
their own selection-bias and closure argument before addition.

| Selection composition | Obligation / assessment |
|---|---|
| Public deterministic draw from a complete authenticated snapshot | Makes exact membership reproducible; exposes selected pseudonyms to targeting. Canonical ordering, unbiased sampling, ties, snapshot size, seed binding and anti-grinding all need exact rules. |
| Private self-selection by VRF | May delay exposure of selected participants. A threshold gives a random number of selected units; taking the first n revealed proofs or dropping silent selections cannot silently stand in for a fixed complete jury. Requires its actual protocol and reviewed implementation. |
| Invitation/confirmation then draw | May improve expected response, but hostile filtering changes the eligible fraction. A second random draw does not fix a censored confirmation set. |

No recommendation promises useful turnout yet. Declared willingness is consent
to be selected for a bounded workload, not evidence of independence or a future
human response. Owner-local withdrawal affects future eligible contexts under
the selected close rule; it does not silently rewrite an already fixed jury.

### Same-instance finality versus safe continuation

**Inference from the analytical model:** with n fixed equal-weight signers,
at most f equivocators and correct exclusive signing, opposite thresholds need
`A + R > n + f`; conflicting accepted values need `2A > n + f` where applicable.
Weighted/resource-unit selection needs a matching weight-based proof and
cannot reuse a key-count threshold. The arithmetic alone supplies no network
protocol, persistent lock, membership proof or live result.

Keep three things distinct:

1. collected evidence and a verifiable certificate for one immutable instance;
2. a local observation such as incomplete, unavailable or resource-budget spent;
3. a protocol-proved close/successor that permits a new attempt or effect.

Later contrary votes cannot invalidate a valid monotone certificate within its
proved model. If valid conflicting certificates do appear, record a fault-model
violation/conflict; do not retrospectively delete signers to manufacture one winner.
Expiry of usability is a separate consumer/currentness decision.

| Complete lifecycle option | Consequence |
|---|---|
| Certificate collection without global close | Small logical core, but lack of a certificate remains unresolved. No safe automatic replacement for the same conflict follows. Different readers can observe different progress. |
| Common ordered admission and protocol close | Can define which ballots/attempts belong to a closed context and authorize a successor. Requires an actual shared-state proof, availability, time/order policy and membership continuity; a local timer is insufficient. |
| Fresh jury after local timeout | Rejected: two disjoint juries can certify opposite outcomes with all signers behaving correctly inside their own instance. |

**Recommendation:** design certificate collection as the evidence responsibility
and make close/continuation an explicit required dependency wherever retries,
finite final rejection or conflicting effects are supported. Compare reusing
qualified Network State evidence with a separately justified ordered service.
Do not start a new permanent validator organisation or assume a real proof exists.
If neither dependency fits, the finite-retry core remains unready.

The maintained Network State owner currently verifies appointed C0 evidence;
it is not an existing autonomous pool/close service. Reusing its verification
and ownership discipline does not supply a qualified public successor.

### Quantitative envelope

**Measurement, 2026-09-07:** Python 3.12.12, analyzer SHA-256
`b858b9144631783ca9b6e8ae8b0e3c4b868301f4501a49b881770b7c0bfb00c6`.
All normalization/expectation checks passed; direct enumeration covered 56
small samples and 441 same-jury certificate pairs. Outputs are reproducible
from the retained script; generated JSON was captured outside the repository.

Fixed uniform population N=1000 with K=200 hostile labels:

| n / f / A=R | P(selected hostile > f) | P(at least A agreeing honest replies), exactly f withholding and response p=0.8 | Mean minutes/day at 100 tasks/day, 1000 willing labels, 10 min/task |
|---|---|---|---|
| 15 / 4 / 10 | 16.266930% | 32.212255% | 15 |
| 31 / 10 / 21 | 3.051112% | 0.922337% | 31 |
| 61 / 20 / 41 | 0.464383% | 0.010634% | 61 |

The response column is conditional on the maximum modeled faults, with all
remaining responders agreeing; it is not averaged over sampled populations.
It exposes limited availability margin rather than forecasting real turnout.
The last column is an illustrative mean, not a workload or fairness guarantee.

For n=61, 1000 independent admitted draws yield a 99.048274% chance of at least
one draw outside the illustrative fault bound. This is model-envelope exposure,
not a prediction of a successful network attack. Fixed population, independence,
uniform selection and no adaptive corruption are explicit assumptions.
For n=31, reducing honest volunteers from 800 to 160 while all 200 hostile
labels volunteer changes the eligible hostile fraction from 20% to 55.555556%
and the outside-f10 tail from 3.051112% to 99.447698%.

The disjoint-jury example uses signers 0..6 and 7..13, five signatures each on
opposing outcomes, with zero equivocators. Same-jury intersection checks pass;
they do not supply cross-attempt continuity. No number in this section is a
selected jury size, acceptable risk, time window or public resource budget.

## Options

| Composition | Current assessment |
|---|---|
| Adopt Kleros-style court wholesale | Reject for the selected no-cryptoasset boundary; removing its economic foundation creates a different system. |
| Adopt a conventional verifiable election deployment wholesale | Documented Belenios admission/administration/credential custody does not fit the target; public ballots also do not require its secret tally machinery. |
| Reusable Ardents rules plus maintained proof/ordering/randomness components | Preferred research composition, conditional on real resource admission and safe close/bootstrap. No engine or beacon selected by this assessment. |
| Invent the whole stack, including consensus and cryptography | Not the default; first-party cryptographic primitives remain forbidden. A missing suitable component is an explicit feasibility problem. |

## Recommendation

Keep the core focused on bounded instances, scoped participation, verifiable
selection, public pseudonymous choices and result evidence. Reuse maintained
components after matching their assumptions. Avoid speculative delegation,
reputation, reward economies and automatic rerolls in the initial contract.

Confidence is high in the identified separations and counterexamples, moderate
in this being a useful reusable boundary, and insufficient to claim a complete
public composition. The strongest objection is the joint dependency on common
admission/close, resource-based influence and participation: separating interfaces
does not make these dependencies inexpensive, independent or available.

### Blocking design decisions

| Gate | Concrete decision/evidence still needed |
|---|---|
| B1 - admitted influence | Actual resource proof, splitting/distribution invariance, re-use domain, weighting policy, concentrated/rented attacker allocation and legitimate participation cost. |
| B2 - common context and close | Qualified pool/admission/close proof, genesis and fresh/restarted verification, old/new continuity, clock/order and data availability. No voting-pool bootstrap cycle. |
| B3 - draw and entropy | One exact fixed or variable-size selection protocol, entropy source/round, verification and seed/key/input grinding bounds, outage behavior and workload/willingness policy. |
| B4 - operating envelope | Supported population/load, acceptable lifetime risk, honest resource/turnout assumptions, thresholds, finite timing/retry/storage/CPU/RSS/carrier budgets and combined measurements. |
| B5 - grammar and integration | Exact bounded encoding, suites and signatures, methods, persistence and compatibility, selected maintained dependencies, pseudonym/transport exposure evidence and consumer boundary. |

Resolve B1 and B2 together first: the resource admission evidence must have one
qualified non-duplicated context, while that context's producers also need a
justified admission/continuity rule. B3 depends on their fixed commitments.
B4 can falsify any candidate and force a return to B1-B3; B5 must reflect the
chosen complete composition rather than hide unknown proofs in byte arrays.

## Disposition

This assessment and its work-package map are prepared for review. The two
Product Owner choices are promoted to the operating model; all mechanism
recommendations remain proposed. R-149 remains the same open research question.
No accepted technology ADR, cryptoasset, maintained runtime, package, RPC,
implementation-ready issue or public guarantee is created. The finite analyzer
is retained evidence, not a verifier or proof of people/operator independence.
