---
id: R-003
title: How does a Service Name bind and recover without becoming a directory?
status: active
owner: product research
started: 2026-08-08
reviewed: 2026-08-08
---

# R-003 — Service Name contract

## Decision this unlocks

Define how an exact human-readable Service Name is obtained, authenticated,
resolved, updated, recovered, expired, and governed without becoming a public
service directory or one mandatory administrator. The result must let a stable
name survive accepted Service Target replacement while keeping direct Target
connections pinned and resolution failures explicit.

## Current contract

- [Product vision](../../product/vision.md)
- [Network functional map](../../product/functional-map.md)
- [Threat model](../../security/threat-model.md)
- [R-006: Service Target lifecycle](r-006-service-target-lifecycle.md)
- [R-002: Live Application Interface](r-002-live-application-interface.md)

Already fixed: a Service Name is optional, human-readable, and used for exact
resolution rather than search. An Unlisted Service may be opened by anyone who
already knows its exact name; knowing the name is neither authorization nor a
secrecy guarantee. Successful resolution returns a Service Target, while a
direct Target destination bypasses naming. Routine host migration preserves the
Target and needs no name change; loss or compromise replaces the Target and
requires trustworthy rebinding of the stable name.

R-003 does not select a blockchain, distributed hash table, consensus protocol,
registry implementation, token, payment model, key algorithm, or wire format.

## Decisions

### P4-D1 — Name Authority is separate from Service Authority

**Product Owner decision, accepted 2026-08-08:** every Service Name is controlled
by a distinct Name Authority. It authorizes authenticated Name Record changes
for that name, including binding the name to a replacement Service Target. It is
not a User identity, Service Authority, Service Target, Service Instance, Node
identity, publication permission, or Application authorization.

The Name Authority is not required online for ordinary Service publication,
resolution, or connection establishment. A Publisher proves control of the
Service Target and publishes its reachability without receiving naming authority.
The Developer may keep Name Authority custody separate from the active Service
host; storing both authorities together is permitted only as an explicit custody
choice and does not preserve name recovery if that common boundary is compromised.

This separation is required by the accepted R-006 catastrophe path. If the
Service Authority is lost or compromised, the Developer creates a new Service
Authority and Service Target, then uses the independent Name Authority under the
eventually accepted naming policy to rebind the existing name. The old Target
remains untrusted. Routine migration under the same Service Authority preserves
the Target and therefore does not require Name Authority use.

A Name Authority is powerful: a valid malicious rebind can direct name-based
Users to an attacker-controlled Target, whose authentication would then be
correct for the poisoned binding. Direct connections to an explicitly supplied
Service Target do not follow a changed Name Record and cannot silently fall back
to the name. R-003 must therefore define separate loss, compromise, rotation,
recovery, transfer, expiry, conflict, and transparency behavior for Name
Authority without pretending target authentication repairs a captured name.

The existence of Name Authority does not decide who initially allocates a name,
which Namespace recognizes it, or whether recovery uses successor keys, several
authorities, a time delay, a quorum, or another mechanism. No single project or
network administrator gains implicit authority over every name.

### P4-D2a — One canonical network-wide Namespace

**Product Owner decision, accepted 2026-08-08:** V1 has one canonical
network-wide Namespace for Service Names. The same complete Service Name denotes
the same Name Record for every honest compatible client with the same valid
network state. A Resolver either returns that authenticated record or an explicit
unavailable, stale, conflicting, or invalid result; it does not assign a local or
provider-specific meaning to the canonical name.

Canonical Service Names may be hierarchical. Authority over a parent name may
delegate a bounded subordinate name or subtree without granting Service
Authority, authority over siblings or ancestors, or a network-wide administrator
role. The exact label syntax, depth, normalization, and confusable-name handling
remain P4-D2c decisions.

An endpoint or Application may maintain a local alias for convenience, but the
alias is not a Service Name, has no network-wide meaning, and must be visibly
distinguished before sharing or connecting. Resolution never silently tries an
external namespace, another namespace root, ordinary DNS, a search result, or a
same-looking local alias when the canonical name fails. An explicit gateway may
translate another naming system only by producing a canonical Service Name or
Service Target under a separately visible Application policy.

One canonical Namespace is a consistency and user-experience contract, not a
decision to use one server, registrar, administrator, ledger, operator set, or
project-controlled root. P4-D2c through P4-D6 must further constrain naming and
governance power and make partitions, capture, and forks visible.
If honest clients cannot establish one current binding during a conflict, they
fail explicitly rather than accepting resolver-dependent destinations.

Global uniqueness does not make a Service listed, authorized, or secret. Ardents
still supplies no index, search, recommendation, or public browsing surface for
Unlisted Services. P4-D5 must address enumeration and query linking created by
the naming mechanism itself.

### P4-D2b — Permissionless leased initial claim

**Product Owner decision, accepted 2026-08-08:** no person, project operator,
registrar, account provider, or other central administrator grants a canonical
Service Name. A root-level name enters use through the first valid claim accepted
by the shared Namespace state under its deterministic ordering rule. The claim
binds a time-bounded Name Lease to a Name Authority; it does not bind a host,
Service Authority, Service Target, User identity, legal identity, or publication
permission.

A valid claim must use an available canonical name, authenticate its proposed
Name Authority, satisfy the then-current bounded anonymous anti-abuse rule, and
be accepted as one ordered state transition. Two concurrent or partitioned
claims cannot create two valid controllers of the same complete name. If the naming
system cannot establish which valid transition controls, the name remains
pending, conflicting, or unavailable and Resolvers fail explicitly rather than
choosing locally.

The accepted ordering is the meaning of "first"; wall-clock arrival at one Node,
one resolver's observation, network proximity, project preference, trademark,
social identity, or a manual dispute decision cannot override it in V1. A Name
Lease is a network control state, not permanent property or a claim that its
holder is the person, organization, or brand suggested by the label.

Every Name Lease expires unless renewed by its controlling authority under the
eventually accepted lifecycle. Names are not owned forever merely because they
were claimed first. Exact duration, renewal window, grace behavior, stale-cache
handling, and when an expired name becomes claimable again remain P4-D3. A
Resolver cannot treat an expired lease as current while those transitions are
unresolved.

An active parent Name Authority may authorize creation or delegation of bounded
subordinate names within its own subtree and bind each to a chosen Name Authority.
It gains no authority over siblings or ancestors. P4-D2c and P4-D3 still define
the exact hierarchy and whether parent, child, and lease lifecycle interact after
delegation.

Permissionless does not mean zero-cost or unlimited. P4-D6 and R-010 must select
and measure a bounded anonymous anti-squatting and Sybil cost that does not require
a global User account, identity document, mandatory wallet, token balance, or one
registrar. No cost mechanism, consensus family, ledger, auction, or proof system
is selected by this decision.

## Remaining decisions

1. **P4-D2c — Canonical syntax and name safety:** define label grammar,
   serialization, hierarchy limits, normalization, and confusable-name handling.
2. **P4-D3 — Record lifecycle:** define Name Record versioning, expiry, renewal,
   caching, stale data, equivocation, partitions, and convergence.
3. **P4-D4 — Name Authority lifecycle:** define custody, rotation, transfer,
   loss, compromise, recovery, and the limits of any recovery authority.
4. **P4-D5 — Resolution privacy:** define what resolvers and naming
   infrastructure learn, how exact-name queries resist enumeration and linking,
   and what metadata remains an honest limitation.
5. **P4-D6 — Governance and abuse:** define capture, disputes, squatting, Sybil
   pressure, denial, accessibility, transparency, forks, and exit behavior.

## Hypotheses

- **H1 — Separate name control:** a distinct Name Authority can preserve a
  human-readable name across target replacement without making the online
  Service Authority or one registrar the permanent continuity root.
- **H2 — Target controls name:** Service Authority also controls its Service
  Name, reducing key count but losing truthful recovery from target compromise.
- **H3 — Registry controls updates:** namespace governance directly authorizes
  every binding change, simplifying recovery while concentrating censorship and
  redirection power.
- **H0:** no evaluated naming contract provides useful human names without an
  unacceptable control graph, query graph, capture root, or recovery failure.

## Evaluation criteria

Every candidate naming contract must state:

1. the canonical name and Namespace identity shown to a User;
2. who can create, update, transfer, recover, expire, or retire a binding;
3. how a Resolver authenticates current state and detects stale or conflicting
   state without silently selecting a target;
4. what naming infrastructure learns about names, authorities, resolvers, and query
   relationships under honest, malicious, colluding, and partitioned operation;
5. how target compromise, Name Authority compromise, loss, squatting, capture,
   denial, rollback, equivocation, and forks become bounded visible states;
6. which operators, quorums, roots, clocks, fees, tokens, or external systems are
   mandatory, and how a User exits or survives their failure;
7. the latency, caching, storage, bandwidth, accessibility, and operational cost
   against the accepted R-023 contract.

## Evidence plan

### Primary sources

Compare primary specifications, security models, incident documentation, and
maintained implementations for relevant Tor, I2P, ENS-like, content-addressed,
and replicated naming families. Reference systems shape candidate questions but
do not override the Ardents exact-name, no-directory, location-privacy,
non-centralization, and catastrophe-recovery contracts.

### Experiment

No experiment is justified before P4-D2 through P4-D5 define observable naming
states and adversaries. Later prototypes must use synthetic names and authorities
and retain reproducible conflict, partition, stale-cache, recovery, and private-
query evidence without recording real User activity.

### Failure scenarios

- the Service Authority is stolen while Name Authority remains safe;
- Name Authority is lost, copied, coerced, or used maliciously;
- old and new Name Records remain visible in different partitions;
- a resolver, registry, or quorum equivocates by User or network location;
- visually confusable names direct Users to a different Service;
- an attacker enumerates or monitors exact-name queries;
- one registrar, namespace root, operator set, or external chain disappears,
  censors, captures updates, or becomes unaffordable;
- an attacker floods claims, renewals, resolution, or recovery with Sybils;
- a direct Service Target connection is incorrectly redirected after a name
  update.

## Findings

- **Product Owner decision:** Name Authority and Service Authority are separate
  product authorities with different powers and failure boundaries.
- **Inference:** this separation is necessary for the accepted R-006 claim that
  a stable Service Name can recover from lost or compromised Service Authority
  by binding to a replacement Service Target.
- **Inference:** target authentication cannot repair a malicious but valid name
  update; naming integrity and target authentication are separate security gates.
- **Product Owner decision:** V1 uses one canonical network-wide Namespace. One
  complete Service Name cannot acquire different meanings from different honest
  resolvers, local configuration, or silent fallback to another naming system.
- **Product Owner decision:** canonical names may be hierarchical and delegate
  bounded subordinate authority. Local aliases are explicitly non-canonical and
  must never masquerade as shareable Service Names.
- **Product Owner decision:** a root name is acquired without administrator
  approval by the first valid claim in deterministic shared-state order. The
  claim creates a renewable, time-bounded Name Lease for its Name Authority,
  not permanent property or human identity.
- **Product Owner decision:** a parent Name Authority may issue subordinate names
  only inside its subtree. Concurrent claims cannot create resolver-selected
  controllers; unresolved order becomes explicit conflict or unavailability.
- **Product Owner decision:** permissionless claims may carry a bounded anonymous
  anti-abuse cost, but no global account, identity document, mandatory wallet,
  token balance, or single registrar is required.
- **Assumption:** V1 can make separate Name Authority custody understandable to
  one Developer without requiring an always-online naming administrator.

## Options

- **Separate Name Authority:** preserves an independent catastrophe path and
  permits offline custody, at the cost of another authority lifecycle.
- **Reuse Service Authority:** simplest routine operation but contradicts
  recovery from copied or compromised target authority.
- **Registry-authorized updates:** may provide recovery and disputes, but makes
  registry governance a redirection and censorship root.
- **One canonical Namespace:** gives a shared unambiguous human destination and
  explicit conflict behavior, but creates a common allocation and governance
  problem that cannot be hidden behind resolver choice.
- **Several network namespaces:** could distribute roots but makes a shared name
  ambiguous or longer and moves trust selection into every client.
- **Resolver-local aliases:** convenient for one endpoint but cannot serve as a
  portable network name and create dangerous same-looking destinations.
- **Permanent first claim:** simple continuity, but converts early capture and
  squatting into irreversible property without proving identity or use.
- **Renewable Name Lease:** permits deterministic permissionless allocation and
  eventual reuse without manual control judgment, but adds expiry, renewal,
  censorship, and recovery failure modes.
- **Administrator or auction allocation:** can moderate conflicts or scarcity,
  but introduces approval, identity, payment, capture, or accessibility roots.

## Recommendation

Use the accepted separate Name Authority as the stable control root for each
Service Name inside the accepted canonical network-wide Namespace. Keep concrete
custody, registry, and recovery mechanisms reversible. Allocate root names as
renewable permissionless Name Leases under deterministic shared-state ordering,
with subordinate claims authorized only inside the parent subtree. Next define
canonical syntax, serialization, normalization, hierarchy limits, and confusable
name handling before comparing protocols.

Confidence is high that Service Authority cannot also provide truthful recovery
from its own compromise. Confidence is low that a usable, privacy-preserving,
capture-resistant Name Authority lifecycle exists without a meaningful
governance or availability cost. The single canonical Namespace is also
unverified against partitions, capture, allocation abuse, and accessible
operation. The leased first-valid-claim policy is unverified against front-
running, squatting, renewal censorship, and affordable Sybil resistance; these
are the central R-003 research risks.

The strongest counterarguments are that two authorities are too complex for a
small V1 Developer journey and that one canonical Namespace creates a shared
Control Plane. The first cost is accepted because merging authorities makes the
catastrophe-recovery promise false. The second must be constrained by P4-D2b
through P4-D6 without pretending that resolver-dependent names are safer. A
third is that permissionless first-valid claims reward front-running and
squatting; leases make that harm reversible but do not remove it.

## Disposition

- State: `active`; P4-D1, P4-D2a, and P4-D2b are accepted; P4-D2c is next.
- `Name Authority` becomes canonical product language.
- `Namespace` now means the one canonical Ardents network-wide naming boundary,
  not a resolver-selected provider or local alias scope.
- `Name Lease` becomes canonical product language for time-bounded Namespace
  control by a Name Authority.
- R-006 target lifecycle remains unchanged and now has a distinct naming
  continuity authority.
- No ADR: no irreversible registry, governance, recovery, key, or protocol
  mechanism has been selected.
- No experiment and no production code.
