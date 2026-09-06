---
id: R-146
title: Non-administrative network consensus and validator weighting
status: open
owner: Product Owner and Codex
started: 2026-09-06
reviewed: 2026-09-06
---

# R-146 — Can open validators replace appointed network control?

## Decision this unlocks

Recommend a theoretical public-network direction for the Product Owner's
requirement that nobody holds discretionary administrative power over Ardents.
Determine whether relay contribution, a scarce consensus resource, an external
settlement network, or less shared state best supports that requirement.

The comparison is complete; a public mechanism is not selected. The strongest
architectural recommendation is to separate relay capacity from consensus
influence and minimize the state that needs common ordering. Under the current
no-token/no-mandatory-payment boundary, a native proof-of-work control log is
the reference candidate for further falsification, not an approved protocol or
an established feasible launch. No evaluated complete solution currently clears
all product, security, and actual maintenance constraints.

This assessment does not authorize implementation or supersede an accepted ADR.
It is not the C0 delivery ledger.

## Current contract

- [Scope](../../product/scope.md): bounded project-controlled C0; public
  consensus, staking, payment, and an incentive market are not selected.
- [Operating model](../../product/operating-model.md#control-plane-roots):
  appointed public thresholds and emergency authority are the existing design.
- [Threat model](../../security/threat-model.md): malicious peers, Sybil control,
  partitions, traffic analysis, supply-chain compromise, and governance capture.
- [Network owner](../../technical/network-route-node.md),
  [naming owner](../../technical/naming.md), and
  [release owner](../../technical/release-update-custody.md).
- [ADR-0004](../../adr/0004-authenticated-epochs-and-separated-control-roots.md)
  and [ADR-0006](../../adr/0006-separate-release-safety-from-protocol-transition.md)
  would require explicit reconsideration for a non-administrative public design.
- [ADR-0005](../../adr/0005-route-domains-and-bounded-entry-exposure.md)
  preserves role-local knowledge separation. Resource weight does not establish
  independent operators. Product terms remain in [CONTEXT.md](../../../CONTEXT.md).
- The current common Candidate View, finite freshness, role assignment,
  concentration evidence, and Name lifecycle are substantive requirements.
  Removing appointed signers does not retire these requirements by implication.

**Assumption:** absent another Product Owner choice, preserve the existing
no-own-token and no-mandatory-payment product direction. Consider external
settlement as a conditional comparison only. The user authorized theoretical
comparison, not changing those boundaries. The actual team is one Product Owner
and Codex; no miners, independent auditors, or external operators are assumed
available to staff the design.

## Hypotheses

- **H1:** Minimal shared state, a separately accounted scarce consensus resource,
  and separately measured relay capacity have a clearer security argument than
  consensus influence derived from useful relay traffic.
- **H2:** Distributed bandwidth tests can produce non-reusable consensus weight
  without trusted measurers, an already Sybil-resistant measurement population,
  or real-user traffic receipts, at an acceptable operating cost.
- **H3:** An existing permissionless settlement network avoids bootstrapping a
  new honest consensus resource budget at an acceptable dependency, privacy,
  fee, verification, and freshness cost.
- **H0:** No evaluated complete option meets the autonomy, privacy, no-token,
  no-mandatory-payment, and one-human-plus-Codex maintenance boundaries together.

## Evaluation criteria

These criteria were fixed before the source comparison. Reject a claimed
solution if it requires appointed admission or measurement authorities, lets
signatures override ownership or fixed validity rules, grants extra aggregate
influence solely by dividing the same resource among keys, publicly accounts
real-user connections, or silently restores founder control after failure.

Evaluate open entry; inclusion and censorship; resource rental and reuse;
validator turnover; partitions and rejoin; fresh-node bootstrap; data
availability; randomness; privacy; compatibility; honest resource supply; and
maintenance. A committee algorithm alone does not establish open membership.

**Assumption:** control ordering is outside ordinary per-connection setup.
This does not make control-state freshness optional: new work still needs
sufficient authenticated facts and every accepted lease remains finite unless
an explicit replacement contract is selected. No numeric startup, bandwidth,
storage, control-latency, or attack-cost result is assumed. The existing public
performance requirements remain gates, not inherited measurements.

A dependency's maintenance status, license, vulnerability history, and exact
release would need review before selection. None is evaluated as a ready-to-adopt
runtime in this record; protocol papers are not maintained components.

## Evidence plan

### Primary sources

All sources accessed **2026-09-06**. Dates on papers describe their original
publication, not current implementation qualification. Claims below refer only
to the identified source or specification version.

| ID | Source | Relevant evidence |
|---|---|---|
| S1 | Douceur, [The Sybil Attack](https://www.microsoft.com/en-us/research/publication/the-sybil-attack/), 2002 | Multiple identities do not establish independent entities. |
| S2 | Bitcoin, [original paper](https://bitcoin.org/bitcoin.pdf), sections 4-6; [developer guide](https://developer.bitcoin.org/devguide/block_chain.html) | Verifiable computational work, valid-chain selection, and reorganization risk. |
| S3 | Sheng et al., [Proof of Backhaul](https://www.ndss-symposium.org/wp-content/uploads/2024-764-paper.pdf), NDSS 2024, sections III-V and VIII | A bounded Byzantine challenger assumption is part of the measurement guarantee; it is not derived from open key creation. |
| S4 | Tor, [FlashFlow design proposal 316](https://spec.torproject.org/proposals/316-flashflow.html) | A coordinator and measurement nodes; a proposal, not an authority-free consensus mechanism. |
| S5 | Kokoris-Kogias et al., [ByzCoin](https://www.usenix.org/system/files/conference/usenixsecurity16/sec16_paper_kokoris-kogias.pdf), USENIX Security 2016, sections 3.3, 3.7, 5, 7 | Work-derived rolling membership combined with BFT; incentives, transition details, and failure limits. |
| S6 | CometBFT, [consensus specification](https://raw.githubusercontent.com/cometbft/cometbft/main/spec/consensus/consensus.md), proof and censorship sections; [using CometBFT](https://docs.cosmos.network/cometbft/latest/docs/core/Using-CometBFT) | Weighted quorum, initial validator input, and halting/censorship boundary. Mutable documentation, not a pinned dependency recommendation. |
| S7 | Ethereum, [proof of stake](https://ethereum.org/developers/docs/consensus-mechanisms/pos/); [weak subjectivity](https://ethereum.org/developers/docs/consensus-mechanisms/pos/weak-subjectivity/) | Capital-backed consensus and a fresh/offline client's trusted-checkpoint issue. |
| S8 | Namecoin, [FAQ](https://www.namecoin.org/docs/faq/) | An existing consensus-backed naming example with asset-denominated registration/transaction costs and its own lifecycle. |
| S9 | I2P, [network database](https://www.i2p.net/en/docs/overview/network-database/), documented profile accurate for 0.9.65 | Owner-signed distributed records; explicit bootstrap, Sybil, and query-observation limits. |

### Experiment

None selected or run. Algebraic examples below are illustrations, not empirical
measurements. A future experiment must state the adversary budget, honest
resource supply, network and clock assumptions, error thresholds, workloads,
and external evidence root before execution. It must not become a production
consensus implementation by accumulating prototype code.

### Failure scenarios

Theoretical comparison covers identity splitting, fabricated useful traffic,
colluding measurers, measurement-only resource rental, repeated use of a shared
bottleneck, old validator keys, entry censorship, withheld state data, isolated
fresh clients, founder disappearance, release-publisher disappearance, honest
resource collapse, partitions, reorganization, and key compromise.

## Findings

1. **Sourced fact — S1:** identities are not independent entities. **Inference:**
   one-key-one-vote, age per key, endorsements among unknown peers, and a maximum
   weight per key cannot supply Ardents' missing Sybil boundary.
2. **Sourced fact — S3:** the main Proof of Backhaul result assumes a Byzantine
   challenger fraction below one third; it includes rushing and withholding
   attacks in its model. **Inference:** using bandwidth measurements to create
   the very population whose honesty the measurement assumes is an unresolved
   circular dependency. The paper does not establish an Ardents voting rule.
   This does not reject bandwidth testing as a capacity tool.
3. **Sourced fact — S4:** FlashFlow's design uses an authority coordinator and
   measurers. **Inference:** adopting that measurement architecture would retain
   a privileged measurement role under the user's autonomy requirement.
4. **Inference:** two colluding endpoints can manufacture transfer receipts and
   demand. Proving byte movement does not prove independent demand, useful
   service, exclusive resource reservation, or expensive external transit.
   Measuring a channel briefly does not establish long-term availability.
5. **Sourced fact — S5:** ByzCoin derives membership shares from recently mined
   blocks and combines them with BFT. Its design uses mining rewards and fees;
   membership changes and quorum rules require special handling. **Inference:**
   replacing work with a capacity score, or removing its economic assumptions,
   does not inherit that paper's result. A native work-plus-BFT combination
   would be a substantial protocol responsibility for this team.
6. **Sourced fact — S6:** CometBFT's consensus uses more than two thirds of voting
   power for commitment, and its specification describes halting/censorship by
   a coalition holding at least one third. **Inference:** an available engine
   can execute an agreed validator set; it does not establish that set's
   independent ownership or solve autonomous recovery beyond its fault model.
7. **Sourced fact — S2:** valid-chain work supplies a verifiable ordering signal;
   competing branches and reorganizations are part of the model. **Inference:**
   a native work-only control log is a simpler theoretical reference than
   inventing bandwidth voting plus committee rotation. It gives probabilistic,
   not absolute, finality and does not establish fresh-source availability.
8. **Sourced fact — S7:** stake provides a scarce capital basis, while weak
   subjectivity makes fresh/long-offline bootstrap a separate trust concern.
   **Inference:** stake is not a solution to all bootstrap or governance
   dependencies, and a valueless internally issued reputation unit is not an
   economically meaningful stake merely because it can be confiscated.
9. **Sourced fact — S8:** Namecoin demonstrates consensus-backed names with
   registration/transaction costs. **Inference:** an existing chain is useful
   comparative evidence but is not a compatible replacement for Ardents' Name
   semantics or no-payment contract. Fee sponsorship moves cost and can create
   availability dependencies; it does not eliminate them.
10. **Sourced fact — S9:** I2P distributes signed records and documents Sybil and
    bootstrap attacks. **Inference:** owner-authenticated discovery can reduce
    shared ordering, but a DHT alone does not preserve Ardents' common complete
    Candidate View, admission completeness, or route-selection privacy.
11. **Assumption:** honest volunteers may supply bounded relay and consensus
    resources. **Inference:** their future number, independence, and sustained
    resource budget cannot be inferred from the actual current team. A small
    transaction workload does not imply a cheap-to-secure consensus network.

### Illustrative weight accounting

**Inference; not a selected formula:** suppose H and A are honest and hostile
amounts of the *same correctly accounted resource over the same interval*, with
linear influence. The hostile fraction is A / (H + A). In a weighted BFT design
where one third can halt, A = H / 2 already reaches that fraction. This is not a
universal attack threshold or an estimate of money needed; it illustrates why
an honest resource budget matters more than a count of validator keys.

The minimum identity-splitting requirement is that splitting one fixed resource
allocation must not increase aggregate expected weight. Linear accounting helps
only after resource reuse has been excluded. Concave per-key bonuses and per-key
caps can reward splitting. Linear weight still permits a wealthy operator to
buy influence, and random committee selection adds sampling risk. No arbitrary
committee size, weight cap, or confirmation count is selected here.

## Options

| Option | Product/security fit | Operational and governance dependencies | Disposition |
|---|---|---|---|
| Appointed threshold committee | Fits the current contract; fails the new absence-of-administration objective. | Named custodians, continuing signed authority and emergency powers. | Comparison control only. |
| Bandwidth/traffic/uptime voting | Useful relay capacity is relevant; secure aggregate consensus weight is unproven. Traffic receipts can leak relationships. | Challenger-population security, scheduling, correlated bottleneck accounting, durable anti-reuse, resource-rental economics. | Do not make this the consensus foundation. |
| Native stake and BFT | Known families of scarce-capital consensus; no direct link to relay independence. | Valuable asset, initial distribution, incentives, slashing, withdrawals, offline bootstrap and safety recovery. | Requires a separate economic/product decision; not a fit under the assumed boundary. |
| Native work-only control log | Open resource-based ordering without a native currency is conceptually possible; probabilistic finality conflicts need explicit treatment. | Sustained honest work, bounded state growth, difficulty/time rules, censorship economics, reorg-safe Name lifecycle. | First independent reference candidate for falsification; not release-ready or proven feasible. |
| Native work-derived BFT membership | Faster commitment may help names; adds membership and finality composition obligations. | All work-budget problems plus safe rotation, committee capture, long-range/fresh-client reasoning, and quorum-loss behavior. | Defer unless work-only ordering fails a concrete required latency/finality condition. |
| Existing permissionless settlement | Avoids creating a fresh underlying resource-security population. | External chain, fees, censorship/finality assumptions, independently verified data, privacy and software maintenance. | Strongest maintenance candidate if the Product Owner admits these tradeoffs; no chain selected. |
| Local signed discovery with no common log | Attractive for owner-controlled facts; no global transaction delay. | Discovery capture and inconsistent views; cannot decide globally competing Name claims by itself. | Use only where equivalent safety is demonstrated; not a complete replacement. |

Proof of storage is not shortlisted: retained user content is outside the
network core, and adding a storage-proof system would introduce another scarce
resource and proof runtime without solving the relay-specific product problem.
This is a scope/maintenance inference, not a claim that storage consensus is
impossible. Federated trust chosen by users can avoid a single appointed list,
but adds trust-graph configuration and canonical-fork questions; it does not
meet an automatic globally agreed membership requirement by default.

## Recommendation

**Choose none of the complete mechanisms for implementation yet.** Recommend
one architectural direction and a ranked next decision: minimal control state,
owner verification, and separate accounting of relay capacity and consensus
influence. Under the current assumed independent/no-token/no-payment direction,
use native work-only ordering as the first reference model to falsify. If an
external settlement dependency and funding cost are acceptable, compare that
before investing in a custom native consensus protocol.

Confidence is **high** in rejecting raw useful-traffic or per-key reputation as
the initial voting foundation; **moderate** in the architectural separation;
and **low** that a standalone token-free work network can sustain enough honest
resources under the declared adversary. The strongest objection is that useful
relay volunteers have no demonstrated reason or budget to perform sufficient
additional work, and a large adversary can outspend them. Splitting layers can
also require more independent infrastructure, rather than reducing it.

### Proposed architecture for evaluation

This section is a proposal, not a maintained contract or new domain glossary.

| Fact or operation | Proposed decision boundary | Required qualification or unresolved condition |
|---|---|---|
| Instance authentication and ordinary Application bytes | Service/Endpoint-owned keys and finite credentials; no transaction for each connection or packet. | Preserve Route and Application boundaries; no consensus receipt of traffic or destinations. |
| Node advertisements and source delivery | Owner signatures, bounded dissemination, endpoint verification. | Signed claims do not prove truthful capacity, independent control, or complete discovery. |
| Common candidate membership and non-overlapping role assignment | Deterministic transitions under agreed, available input where the existing global contract requires it. | Keep the common View until equivalent omission, sampling, and separation protection is demonstrated; do not replace it silently with local peer gossip. |
| Name collision, ordering, lease/reclaim and authority succession | Narrow shared ordering plus owner signatures and explicit lifecycle rules. | Finality, private submission, front-running, retained floors, and current-proof freshness require a new complete design. |
| Relay load choice | Finite capacity evidence and bounded Endpoint-local observations within role/exposure constraints. | No public per-User receipt or destination-specific score; measuring must not bypass direct-source exclusions. |
| Software authenticity | Verifiable author signatures, reproducible artifacts, and local activation policy. | Remove mandatory publisher permission-to-run only via a replacement safety contract; a signature proves authorship, not correctness. |
| Protocol evolution | Explicit opt-in compatible implementation; incompatible rules identify an explicit fork. | No consensus transaction that grants administrators arbitrary upgrades, package installation, or emergency shutdown. |

### Native reference flow

1. A fixed initial network identity and rule set establish what is valid. Their
   initial provenance must be independently checkable; they do not include a
   continuing founder key capable of authorizing network state or software use.
2. Anyone can verify records. Any participant may attempt to produce a control
   block by doing work bound to this network, parent, and exact block contents.
   Influence comes from work in the selected valid history, not declared CPUs,
   relay traffic, a paid account, a hardware serial number, or key count.
3. Blocks carry bounded data needed to reconstruct state, or a separately
   justified availability protocol. A root hash with withheld inputs is not
   adequate. All full verifiers execute the same validity rules; accumulated
   work never makes an unauthorized Service or Name signature valid.
4. Shared inputs contain only necessary infrastructure facts and naming
   submissions. Do not log a User's routes, queries, sessions, or traffic.
   An opaque commitment is not automatically private: low-entropy Name hashes
   are dictionary-testable and timing can remain linkable.
5. A work-only rule orders valid histories by accumulated work. Name operations
   need explicit provisional/accepted/conflicting states and a researched
   confirmation policy. A reorganization must not silently revive a superseded
   authority, retract a displayed final Name claim, or lower a retained safety
   floor. If that cannot be reconciled with the product contract, reject this
   candidate; adding an ad hoc BFT checkpoint does not resolve it automatically.
6. Endpoints consume an authenticated current snapshot with bounded proofs;
   opening a connection does not wait for a new block. A header chain alone
   does not prove state validity, global completeness, freshness, availability,
   or absence of eclipse. Full-node and fresh/light-client costs are separate
   unsolved profiles, including bounded initial synchronization and storage.
7. Founder disappearance should not stop progression if sufficient independent
   honest producers and distributors remain. If *all* producers stop, there is
   no new common state. Existing work continues only within accepted safety and
   credential bounds; this proposal does not grant indefinite stale operation.
8. New producers must not need a manual admission signature. Submission gossip,
   inclusion under honest production, resource scarcity, and spam budgets need
   explicit models. Work admission and fair access for anonymous Names are
   different requirements; a large miner must not bypass Name claim costs.
9. With no token or fees, production is voluntary or funded outside protocol.
   No honest work supply is assumed. Difficulty adjustment, time manipulation,
   rental spikes, profitability-driven exits, recovery after a prolonged outage,
   and founder-heavy genesis are gating questions, not configuration details.

This is deliberately a work-only reference. A hybrid committee would need a
proof for membership admission, work withholding, unbiased selection, expiry,
turnover quorum intersection, entry censorship, and behavior after loss of the
old quorum. Installing CometBFT would not supply that proof. No homemade
cryptographic primitive or production consensus implementation is proposed.

### Conditional external-settlement flow

Use a separately reviewed existing permissionless network only for necessary
ordered records, with Ardents clients independently deriving/validating their
meaning. An external transaction order cannot authorize invalid Ardents state.
The design must have no privileged proxy, contract-upgrade key, trusted RPC,
admin-selected checkpoint, or single funded batching operator as an indispensable
participant. A mutable administrative contract would fail the user objective.

This variant needs a funding source for submissions; allowing many sponsors
reduces a single-sponsor dependency but does not make guaranteed submission free.
Embedding only a hash does not inherit off-chain state validity or availability.
Embedding records may expose permanent metadata and requires explicit privacy
analysis. Client verification and private query transport also require work.
Existing chains do not automatically implement Ardents Name ordering/recovery.

This is external security dependence, not necessarily direct administration by
Ardents founders. If network self-sufficiency also forbids an external chain,
reject the option. Even when accepted, a chain halt eventually affects fresh
control-dependent operations; being outside per-packet traffic does not remove
that eventual dependence.

### What autonomy can and cannot promise

**Proposed claim:** protect Service/Name owner authorization and the absence of
privileged network-wide commands against founder disappearance or abuse, under
fixed validated rules, adequate honest consensus/relay resources, available
state, and the selected network/freshness assumptions. Measure with the failure
matrix below. Limitation: resource concentration can still cause censorship,
stalling, or permitted-history manipulation within the consensus fault model.
It is not proof of anonymity, independent operators, or universal availability.

There are two separately attacked populations: those ordering control state and
those carrying traffic. Securing one does not establish independent control of
the other. A large valid relay population can still be malicious; consensus
cannot cryptographically certify real-world operator independence. Existing
public anonymity and concentration gates must be retained or explicitly
replaced by researched, narrower claims.

Removing emergency administrative keys also removes their fast common response
to a newly discovered flaw. Compatible local rules can automatically reject
objectively invalid behavior. Unknown vulnerabilities, disputed policy, and
incompatible software changes require owner choices; universal automatic repair
is not promised.

### Falsification and decision gates

These are evaluation conditions, not scheduled work or a second task ledger.

| Case | Required observation before promotion | Reject or reopen when |
|---|---|---|
| Identity splitting and shared resources | Fixed resource cost bounds aggregate expected influence across keys. | Repeated counting or per-key bonuses cheaply multiply influence. |
| Measurement collusion | Any proposed bandwidth weight has a non-circular challenger security argument and topology/anti-reuse model. | Its honest-challenger assumption is obtained only from its own unvalidated scores. |
| Honest resource supply | A declared independent resource population and adversary-cost envelope support the selected fault margin. | Security relies on hypothetical volunteers, key count, or cheap work mistaken for high attack cost. |
| Founder and release-publisher disappearance | Ordinary participant admission and state progression continue without their keys, while local code authentication remains possible. | A hidden issuer, timestamp publisher, upgrade key, or rescue committee is indispensable. |
| Reorg or conflicting Name history | No silent authority resurrection or rollback of a claimed final result; explicit bounded unavailability where needed. | A finality promise depends on an unmodeled reorganization or checkpoint administrator. |
| Partition and rejoin | Outcomes follow the exact selected assumptions; safety is not traded for silent acceptance of incompatible states. | Both isolated sides invent final conflicting truth or reconnection needs an unacknowledged authority. |
| State withholding and fresh-client eclipse | Invalid, unavailable, or insufficiently fresh evidence is refused; honest alternative retrieval fits finite budgets. | A signed root/header is treated as proof of complete, available, latest state. |
| Privacy | Role-local observations contain no new origin-to-destination binding or public user-traffic accounting. | Scoring/consensus introduces a relationship graph or bypasses source/Route separation. |
| Capacity and maintenance | Existing product budgets and bounded sync/storage hold for actual roles with available maintained components. | Required operation assumes an unstaffed economics, consensus, or infrastructure organization. |

## Disposition

- The authorized theoretical comparison is complete. R-146 remains **open for
  product/feasibility selection**; there is no running experiment or selected
  C0 research execution after this assessment.
- H1 has architectural support, not a protocol proof. H2 is not established by
  the examined measurement work. H3 is conditional on product changes. H0
  remains live because no complete option clears every current constraint.
- Changed only this research record and the question index. No product/security
  contract, glossary, accepted ADR, dependency, package, or runtime was changed.
- No performance or security experiment was run. No generated artifact or
  private material was stored in the repository. Documentation validation does
  not qualify the proposed network.
