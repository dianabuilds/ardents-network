---
id: R-148
title: Shared-resource benefits for useful relay contribution
status: open
owner: Product Owner and Codex
started: 2026-09-06
reviewed: 2026-09-06
---

# R-148 — Which shared-resource benefit could motivate useful relay contribution?

## Decision this unlocks

Choose a future-product benefit worth investigating after the resource-control
comparison in [R-147](r-147-contributor-resource-controls.md): encrypted backup
storage, replicated application data, or additional service capacity. The
Product Owner explicitly permits proposals that require changing current
contracts. This is a theoretical comparison, not adoption evidence or a
technology selection.

## Current contract

The [product scope](../../product/scope.md#outside-the-network-core) places
content persistence, replication, incentive markets, and general compute
outside the network core. They can be proposed as Applications, optional
Overlay Services, or separate future products. The
[threat model](../../security/threat-model.md) does not qualify retained content
or a User identity system. [Network Route and Node](../../technical/network-route-node.md)
owns current transport and bounded resource behavior; it does not implement a
storage pool or contributor reward exchange. [Documentation ownership](../../development/documentation.md)
requires accepted behavior to be promoted with its owning change.

The [glossary](../../../CONTEXT.md) keeps Contributor, Publisher, Node, Person,
Device, identity, Credential, and Capability distinct. This record resolves no
new domain term. Service allowances must not silently become route-selection
trust, governance rights, or evidence of independent operators. Co-residence,
resource controls, storage privacy, and incentive accounting need their own
qualification; none follows automatically from the others.

## Hypotheses

- **H1:** A recoverable encrypted backup allowance is a comprehensible personal
  benefit that can motivate sustained useful relay contribution.
- **H2:** Additional transfer allowance under congestion is a sufficient benefit
  for existing heavy users and avoids building a storage product first.
- **H3:** Replicated application data motivates a narrower developer/Publisher
  audience more effectively than personal backup.
- **H0:** Costs, accounting trust, or resource commitments outweigh these
  benefits; none justifies a public resource exchange.

## Evaluation criteria

- A person can explain what they contribute, receive, and lose on withdrawal.
- Useful relay service increases; extra disk capacity or identity count alone
  does not count as success. No adoption lift or target percentage is assumed.
- Total physical storage, redundancy, repair, retrieval, metadata, and operating
  costs fit funded capacity. No positive credit balance substitutes for bytes.
- Owner CPU, RAM, disk, upload/download rates, period quotas, and schedule remain
  enforceable. Compare actual restore time and interactive service under load;
  no latency, availability, or conversion rate is promised before measurement.
- Evaluate malicious storage peers, collusion, self-traffic, duplicate credit
  claims, churn, partitions, resource exhaustion, and issuer disappearance.
- State who places data, audits availability, repairs losses, issues allowances,
  accounts for spending, and supplies startup capacity; do not hide these jobs
  behind the word decentralized.
- Fit one Product Owner plus Codex. Existing components are references for a
  later maintenance, license, audit, and integration review, not dependencies.

## Evidence plan

### Primary sources

All sources accessed 2026-09-06:

1. [Tahoe-LAFS: operation and trust boundaries](https://tahoe-lafs.readthedocs.io/en/latest/about-tahoe.html).
2. [Storj: peer classes and data flow](https://storj.dev/learn).
3. [Storj: resources actually compensated](https://storj.dev/node/payouts).
4. [OrbitDB: data models and consistency](https://github.com/orbitdb/orbitdb).
5. [OrbitDB: encryption and replicator separation](https://github.com/orbitdb/orbitdb/blob/main/docs/ENCRYPTION.md).

### Experiment

None executed or selected. Before a later bounded experiment, select a corpus,
resource ceilings, redundancy policy, independent failure domains, churn trace,
restore deadline, repair budget, and tolerated baseline-service degradation.
Then test upload, owner-device loss and recovery, shard loss, partition,
withdrawal, and quota reduction. Fail a candidate if it acknowledges protected
backup before its selected durability condition, cannot restore within the
selected fault envelope, or violates a host limit to preserve a remote promise.

A separate accounting experiment must try self-generated traffic, colluding
receipts, replay, multiple identities, restart, and issuer failure. Falsify any
claim that raw byte counters alone establish useful contribution. A Product
Owner walkthrough checks the product contract only. Adoption and retention
validation require actual future users; none are assumed available.

### Failure scenarios

- Many apparent storage peers fail together because one operator controls them.
- Owners reduce disk or traffic limits while previously accepted data still
  needs repair. Admission stops and repair attempts stay within hard ceilings;
  degradation must be visible when capacity is insufficient.
- A laptop disappears after receiving credit; other participants bear repair
  traffic and cannot immediately reclaim equivalent usable storage.
- Colluding peers manufacture transfers or reuse the same evidence to claim
  rewards. Even storage possession evidence does not prove operator diversity.
- A member stops contributing, loses the original device, or cannot reach the
  allowance issuer. Existing recovery must have a stated retention/expiry and
  funding policy; indefinite free retention is not an assumed solution.
- A malicious peer returns corruption or an older valid snapshot. Integrity,
  version freshness, and recoverable key/catalog state need separate checks.
- Service credits link an infrastructure identity to sensitive application use.
  Encrypting files does not prevent this metadata leak.

## Findings

- **Sourced fact (1):** Tahoe-LAFS encrypts and encodes files at a client/gateway,
  distributes shares, and reconstructs from enough valid shares. Storage peers
  remain an availability dependency. A remotely operated gateway moves the
  plaintext trust boundary to that gateway; a local gateway keeps it local.
- **Sourced fact (2, 3):** Storj separates storage nodes, Uplink clients, and
  Satellites. Satellites coordinate metadata, node reputation, audits, repair,
  and billing. Its operator policy compensates used storage and specified
  bandwidth, not merely a declared disk limit. This is evidence of an operating
  model with coordinating services, not proof of an authority-free exchange.
- **Sourced fact (4, 5):** OrbitDB offers replicated event, document, and key/value
  models with eventual consistency. Its encryption interface can separate
  payload readers from replicators. The documented SimpleEncryption module is
  explicitly unaudited; these docs establish a pattern, not Ardents security
  qualification or a recommendation to adopt that module.
- **Measurement:** No benchmarks, restore trials, cost measurements, security
  tests, or user research were performed for this comparison.
- **Assumption:** Some prospective users value recoverable personal files enough
  to keep a host available. Some have spare bandwidth but little spare disk.
- **Inference:** Encrypted backup has the clearest general-user outcome of the
  three ideas. Replicated data is more directly useful to application builders.
- **Inference:** A storage reward increases relay supply only if useful relay
  service can earn that reward. Disk-for-disk exchange alone targets a different
  bottleneck. Cross-resource conversion requires funded storage and accounting.
- **Inference:** Remote storage of encrypted records is feasible without giving
  peers plaintext. Arbitrary remote queries over confidential fields need an
  explicit trust or cryptographic computation design. Local querying over an
  authorized replica offers a narrower starting point, with client costs and
  conflict/consistency semantics still to define.

## Options

| Option | Concrete benefit and fit | Dependencies, risks, and rejection reason |
|---|---|---|
| Encrypted versioned backup | Recover selected folders on a replacement device, including while the old device is offline. First consumer hypothesis. | Storage placement, repair, keys/catalog recovery, capacity funding, retention and exit behavior. Reject if useful allowance costs more than contribution or cannot survive selected failures. |
| Encrypted replicated application data | Keep notes, settings, or a small application's records across authorized devices; remote peers retain encrypted state. Developer/Publisher hypothesis. | Writer authorization, local indices, schema/version handling, conflict rules, metadata, eventual versus strong consistency. Reject a universal database promise without an exact application job. |
| Additional transfer/service allowance | More bulk transfer, larger service quotas, or a greater scheduling share when capacity is contested. Directly fits heavy users. | Capacity admission, auditable contribution, fair baseline and class privacy. Reject fixed speed guarantees without reserved end-to-end capacity, or rewards that require degrading ordinary use. |
| Family or community backup pool | Known participants exchange off-device backup capacity with explicit limits. Optional narrower trust model. | Simpler membership but correlated failures and social/operator dependencies. Useful technical test context, not evidence of open-network independence or adoption. |

All options require separate authorization for each contributed resource. A
relay-only participant must be able to opt out of retaining third-party data.
CPU/GPU execution is a separate application and attack surface, not an implied
extension of encrypted storage.

## Recommendation

**Choose no implementation or accounting mechanism yet.** Prefer encrypted
versioned backup as the first future consumer benefit to validate; keep
additional bulk-service allowance as the simpler competing hypothesis. Treat
replicated application data as a later focused developer feature.

Proposed product logic: measured, demanded contribution earns bounded service
allowance. A participant can contribute relay bandwidth, storage, or both,
within independent limits. Storage can reward relay work only from an actual
funded pool; exact conversion rates remain unselected. Empty reserved capacity
can have value only under a separately specified reservation contract. Raw
traffic, uptime, declared capacity, or node count is not sufficient evidence.

Usable storage must account for redundancy and spare repair capacity. For
illustration only, a three-full-copy policy turns 300 GB of raw occupied space
into at most 100 GB of logical data before metadata and reserves. Other coding
policies differ. Quotas also need a time dimension: a burst of relay activity
cannot finance permanent storage. Preview the allowance, its validity, and
what happens when contribution declines. Propose stopping new uploads first,
with a funded, declared recovery interval; no sudden deletion as a reward rule.

Additional priority should mean a bounded share of an optional service queue,
more transfer allowance, or more admitted bulk work under contention. It cannot
promise a faster destination or provider link. Preserve a baseline share and
bound starvation; never shorten a privacy route or grant trust because someone
contributes. A visible service class may itself leak metadata and needs review.

Encryption claims must name the protected file/record contents and metadata,
malicious storage operators, trusted local clients and keys, and their limits.
Planned evidence includes corruption/rollback cases, hostile-peer observation,
and restore trials; none exists here. Availability, access-pattern privacy,
endpoint compromise, and key recovery remain separate claims. Revocation cannot
withdraw plaintext or keys already copied by an authorized reader.

A blockchain or tradable token is not inherently needed for an allowance
service. A bounded service operator can issue and redeem allowances, but is
then a trust, privacy, availability, and governance dependency. An open exchange
without such an operator requires its own verification and settlement research.
Anonymous vouchers alone do not prove useful work or prevent colluding earners;
no global stable User identity or anonymity claim is selected here.

Confidence is moderate in the product ordering, low in adoption and economics.
The strongest objection is that backup introduces a second substantial product
and repair burden before the relay network has established user demand. A
mature component or a simpler transfer allowance may prove the better fit for
the actual team capacity. Nothing here requires moving storage into the core.

## Product Owner catalogue direction

On 2026-09-06 the Product Owner requested a broad catalogue of concrete
products and usage scenarios, independent of current architecture and delivery
capacity. The [future product catalogue](../../product/future-product-catalog.md)
owns that exploratory inventory: 18 product concepts, each with an audience,
usage scenario, shared resources, a possible contributor benefit, and a
material condition on the proposed promise. The earlier preference for backup
remains one comparison hypothesis, not an accepted priority or exclusion of
other products. The catalogue also records primary-source examples for
synchronization, encrypted media, and volunteer computation; none is a
selected dependency or an evaluated Ardents implementation. No experiment,
adoption measurement, implementation sequence, or additional C0 work selected.

## Disposition

- Open future-product question; theoretical comparison and exploratory catalogue prepared.
- Added this record and the future product catalogue, with links from the
  research queue and product vision.
- No experiment, package, dependency, reward ledger, public product contract,
  ADR, C0 research execution, or implementation slice selected.
- No existing current contract changed. No generated artifacts or experiment
  code produced. Unrelated workspace changes remain outside this record.
