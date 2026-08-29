---
id: R-093
title: Voluntary Endpoint contribution without role-collapse claims
status: deferred; no co-resident experiment selected
owner: Product Owner and Codex
started: 2026-08-24
reviewed: 2026-08-29
---

# R-093 — Under what constraints, if any, can an opt-in Client or Publisher Endpoint contribute Node capacity without weakening Ardents role, location, or independent-capacity claims?

## Decision this unlocks

Decide whether a future Ardents alpha or public-network profile may offer an
explicit voluntary contribution role to a Client or Publisher Endpoint, and if
so which Node duties, host/network conditions, resource limits, user-facing
warnings, and evidence requirements apply. The research may instead retain the
current rule that every public qualified Contributor runs on a dedicated host
and Endpoint.

This question does not authorize automatic contribution, a token, staking,
payments, a reward market, or a change to Route qualification.

## Current contract

- A default deployment creates a local Endpoint, not a Contributor Node.
  Publishing and contribution are separate explicit actions with separate
  Local Grants and resource limits.
- A public qualified Contributor currently uses a dedicated host/Endpoint with
  no User connection or Service-publication role. A co-resident development
  Node is unqualified and supplies no public capacity or independence evidence.
- Client and Publisher roles may coexist only under their own documented
  profile; this does not establish same-host unlinkability.
- Independent operator/source capacity, permissionless Contributor admission,
  bounded abuse controls, and external evidence remain Public Beta promotion
  gates, not current implementation work.
- Ardents assumes Sybil actors, correlated control, endpoint observation,
  operator seizure/loss, abuse, and traffic analysis. An installation count is
  not evidence of independent capacity.

Relevant owners: [Product scope](../../product/scope.md),
[Operating model](../../product/operating-model.md),
[Functional map](../../product/functional-map.md), and
[Threat model](../../security/threat-model.md).

## Hypotheses

- **H1:** One or more narrowly defined, opt-in, non-default co-resident
  contribution profiles can supply bounded alpha utility while making no public
  capacity, independence, or Application-level location-privacy claim.
- **H2:** A co-resident contribution profile can satisfy a future qualified
  public-network claim only with demonstrable role separation, endpoint
  exposure bounds, public reachability, resource controls, and post-exclusion
  independent-capacity evidence.
- **H0:** No co-resident Client/Publisher contribution profile meets the
  product and threat-model contract; the dedicated-host Contributor rule must
  remain the sole qualified profile.

## Evaluation criteria

- **Exact outcome:** an Owner can explicitly enable, inspect, limit, drain,
  and withdraw a contribution role without changing ordinary Client or
  Publisher behavior by default.
- **Protected information and adversary:** evaluate whether the added duty
  exposes endpoint location, links Client/Publisher activity to Node identity,
  or gives one malicious/correlated actor forbidden Route or Resolution views.
- **Independence:** installations, IP addresses, and nominal Node identities do
  not count as independent families. Any public capacity result must preserve
  the existing post-exclusion family and concentration rules.
- **Operational fit:** the role must have declared CPU, memory, bandwidth,
  storage, public-reachability, update, drain, abuse, and withdrawal bounds.
  It must be supportable by the current one-to-one project team; a model that
  needs hidden 24/7 operations, moderation, or payment administration fails.
- **Failure:** NAT loss, sleep, laptop mobility, power loss, abuse complaints,
  capture of the endpoint, malicious local applications, Sybil enrollment,
  correlated ownership, and forced or accidental opt-in must have explicit
  outcomes.
- **Product truthfulness:** an alpha may measure voluntary contribution but
  cannot label it public independent capacity. A qualified claim requires the
  applicable separate evidence and review gates.

## Evidence plan

### Primary sources

- Current Ardents product, operating, functional, and threat-model contracts,
  accessed 2026-08-24.
- Tor Project specifications on relay roles, guards, relay requirements, and
  operator risks, accessed during the research.
- I2P official documentation on router participation and bandwidth sharing,
  accessed during the research.
- Supported-platform system-service, resource-control, and local-networking
  documentation for any evaluated operating profile, accessed during the
  research.

### Experiment

Before implementing a maintained profile, define an isolated disposable
experiment for each candidate duty. Its README must fix topology, role
placement, opt-in/withdrawal sequence, resource caps, reachability conditions,
measured endpoint exposure, failure injections, and the exact claim that is
tested. No experiment may count co-resident Nodes as independent public
capacity.

### Failure scenarios

- An ordinary Client accidentally enables contribution or cannot stop it.
- A Contributor duty makes a mobile/NATed endpoint unavailable, expensive, or
  identifiable, then retries or silently changes duties.
- A malicious peer uses the role for resource exhaustion, abuse relay, route
  tagging, or endpoint confirmation.
- A Sybil actor creates many nominal co-resident contributors and attempts to
  satisfy a capacity threshold.
- One operator's Client, Publisher, and Contributor identities are selected
  into forbidden role combinations.
- Update, drain, restart, crash, or loss of local authority leaves a public
  duty active or destroys unrelated Client/Publisher work.

## Options

1. **Retain dedicated-host Contributors only.** Strongest alignment with the
   current contract; it does not solve voluntary operator participation by
   itself.
2. **Add a non-default, visibly unqualified co-resident alpha duty.** May test
   operational willingness and bounded utility, but cannot add public capacity
   or a stronger privacy claim.
3. **Qualify narrowly separated co-resident duties.** Requires evidence that
   exact duties, local/network exposure, and family exclusions preserve the
   public contract; no such evidence is assumed.
4. **Make every Client/Publisher an automatic Node.** Rejected unless a later
   research result overturns the opt-in, resource, Sybil, availability, and
   role-separation objections. It is not a default candidate.

## Findings

- **Sourced fact (analogy only):** Tor documents relay operation as an explicit
  operator commitment with public reachability, bandwidth/traffic, resource,
  uptime, update, and abuse/operations consequences; its thresholds and roles
  are specific to Tor and are not Ardents capacity requirements. [Tor relay
  requirements](https://community.torproject.org/relay/relays-requirements/)
  and [operator expectations](https://community.torproject.org/policies/relays/expectations-for-relay-operators/)
  (accessed 2026-08-24).
- **Sourced fact (analogy only):** I2P exposes participation bandwidth and
  tunnel limits as configuration and recognises mobility through a laptop mode;
  that does not establish Ardents route safety, identity separation, or a safe
  default. [I2P control documentation](https://geti2p.net/uk/docs/api/i2pcontrol)
  (accessed 2026-08-24).
- **Inference:** existing volunteer-network practice reinforces the current
  contract rather than weakening it: a useful traffic-carrying duty needs an
  explicit resource and exposure contract, a visible stop path, and operational
  maintenance. It supplies no evidence that every Ardents Client/Publisher
  should participate automatically or that co-resident installations are
  independent capacity.
- **Current-contract fact (updated 2026-08-29):** R-092 and H4-5A/B selected
  and qualified one dedicated Rendezvous Functional Alpha profile. That result
  supplies a possible duty boundary but does not authorize co-residence.
- **Current-contract fact:** H4-5 deliberately makes the dedicated-host
  operating profile and measured alpha utility (H4-5A/B) predecessors of the
  optional co-resident research slice (H4-5C). It says that a Client and
  Publisher must work without contributing and that H4-5C cannot block the
  dedicated profile. [H4-5 contributor viability](../../product/horizon-4/05-contributor-viability-admission.md)
  (inspected 2026-08-24).
- **Inference:** the minimal safe present-day contribution contract is not a
  new binary switch or a simulated relay: ordinary Endpoint operation has no
  Contributor duty. Any future co-resident experiment must begin *after* one
  dedicated duty has demonstrated its exact resource/exposure/withdrawal
  behavior. H4-5A/B now meet that prerequisite; the Product Owner has deferred
  the separate co-resident experiment.
- **Decision gate:** R-093 may name a candidate only after all four inputs
  exist: (1) R-092 selects and measures one native duty on the declared host;
  (2) H4-5A demonstrates ordinary startup, bounded operation, drain,
  withdrawal, and recovery of that exact duty on a dedicated host; (3) H4-5B
  measures its useful completion effect and operator burden; and (4) the
  Product Owner explicitly decides that an unqualified co-resident alpha result
  is worth its added endpoint-location and correlation exposure. Inputs 1-3
  are now satisfied; input 4 is explicitly not selected. The result remains
  `not offered`, not a background retry, a project-operated exception, or an
  incentive proposal.

## Recommendation

Do not choose an implementation. The current Endpoint installation remains
non-contributing. R-092/H4-5A/B now establish the dedicated Rendezvous duty,
but the Product Owner has not selected the fourth gate: a co-resident alpha
experiment is not currently worth its added endpoint-location and correlation
exposure. A future choice must reopen this record explicitly.

**Confidence:** high that automatic participation is not justified by the
current contract. **Strongest argument against this recommendation:** a
dedicated-host-only model may fail to attract enough independent operators, so
a narrowly bounded voluntary alpha profile could be worth evaluating.

## Disposition

Deferred by explicit Product Owner choice after the first three named inputs
were satisfied. No co-resident experiment, implementation, product-contract
change, or ADR is authorized by this record. The dedicated-host Contributor
rule remains authoritative unless a future Product Owner decision reopens the
question.
