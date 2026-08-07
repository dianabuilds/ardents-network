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

## Remaining decisions

1. **P4-D2 — Namespace and initial allocation:** define canonical name scope,
   uniqueness, initial claim, conflicts, normalization, and spoofing resistance
   without one mandatory registrar or an enumerable service directory.
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
  unacceptable ownership graph, query graph, capture root, or recovery failure.

## Evaluation criteria

Every candidate naming contract must state:

1. the canonical name and Namespace identity shown to a User;
2. who can create, update, transfer, recover, expire, or retire a binding;
3. how a Resolver authenticates current state and detects stale or conflicting
   state without silently selecting a target;
4. what naming infrastructure learns about names, owners, resolvers, and query
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
- **Assumption:** V1 can make separate Name Authority custody understandable to
  one Developer without requiring an always-online naming administrator.

## Options

- **Separate Name Authority:** preserves an independent catastrophe path and
  permits offline custody, at the cost of another authority lifecycle.
- **Reuse Service Authority:** simplest routine operation but contradicts
  recovery from copied or compromised target authority.
- **Registry-authorized updates:** may provide recovery and disputes, but makes
  registry governance a redirection and censorship root.

## Recommendation

Use the accepted separate Name Authority as the stable control root for each
Service Name, while keeping concrete custody and recovery reversible until
P4-D4. Next define Namespace identity and initial allocation before comparing
protocols: uniqueness and conflict policy determine what a human-readable name
actually means.

Confidence is high that Service Authority cannot also provide truthful recovery
from its own compromise. Confidence is low that a usable, privacy-preserving,
capture-resistant Name Authority lifecycle exists without a meaningful
governance or availability cost; that is the central R-003 research risk.

The strongest counterargument is that two authorities are too complex for a
small V1 Developer journey. That cost is accepted because merging them makes
the already accepted catastrophe-recovery promise false rather than merely less
convenient.

## Disposition

- State: `active`; P4-D1 is accepted and P4-D2 is next.
- `Name Authority` becomes canonical product language.
- R-006 target lifecycle remains unchanged and now has a distinct naming
  continuity authority.
- No ADR: no irreversible registry, governance, recovery, key, or protocol
  mechanism has been selected.
- No experiment and no production code.
