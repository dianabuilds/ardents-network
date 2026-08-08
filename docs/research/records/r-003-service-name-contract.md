---
id: R-003
title: How does a Service Name bind and recover without becoming a directory?
status: decided
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

**Scope clarification, accepted 2026-08-08:** human-readable exact naming stays
in the long-term Product Core, but the complete Namespace is a separate delivery
horizon. Carrier Lab has no Service Name. The first Named Unlisted Site slice
uses one pre-provisioned exact Name and tests private binding/resolution only.
Permissionless initial claims, leases, delegation, recovery, Anonymous Cost,
governance, forks, and public convergence are Public Beta mechanisms and cannot
block Target-first Route research.

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

A connection opened from a Service Name/Link retains an authenticated
Destination Binding containing Name generation/revision→Target. Same-Target
renewal or Grace may refresh its finite Work Safety Lease. Learned Recovery
Pending, Release, or a valid rebind to another Target stops new leg/recovery work
and closes by a finite signed deadline. The stream never retargets silently; the
Application must open a new connection. A direct Target/Target-Link connection is
intentionally pinned and receives no Name recovery.

A Name Authority is powerful: a valid malicious rebind can direct name-based
Users to an attacker-controlled Target, whose authentication would then be
correct for the poisoned binding. Direct connections to an explicitly supplied
Service Target do not follow a changed Name Record and cannot silently fall back
to the name. P4-D3 and P4-D4 therefore define separate expiry, loss, compromise,
rotation, transfer, and precommitted recovery behavior without pretending target
authentication repairs a captured name. P4-D5 defines resolution privacy and
P4-D6 below completes abuse, governance, and transparency boundaries.

P4-D2a through P4-D6 fix the Namespace, leased claim, syntax, lifecycle,
authority-transition, privacy, governance, and abuse product contracts without
selecting registry, key, threshold, delay, quorum, or consensus technology. No
single project or network administrator gains implicit authority over every name.

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
role. P4-D2c below fixes the V1 label alphabet, hierarchy direction, canonical
form, and explicit link form while requiring finite protocol limits.

An endpoint or Application may maintain a local alias for convenience, but the
alias is not a Service Name, has no network-wide meaning, and must be visibly
distinguished before sharing or connecting. Resolution never silently tries an
external namespace, another namespace root, ordinary DNS, a search result, or a
same-looking local alias when the canonical name fails. An explicit gateway may
translate another naming system only by producing a canonical Service Name or
Service Target under a separately visible Application policy.

One canonical Namespace is a consistency and user-experience contract, not a
decision to use one server, registrar, administrator, ledger, operator set, or
project-controlled root. P4-D6 below constrains naming and governance power and
makes partitions, capture, and forks visible.
If honest clients cannot establish one current binding during a conflict, they
fail explicitly rather than accepting resolver-dependent destinations.

Global uniqueness does not make a Service listed, authorized, or secret. Ardents
still supplies no index, search, recommendation, or public browsing surface for
Unlisted Services. P4-D5 fixes query-linking limits and P4-D6 below fixes the
product boundary for enumeration cost and abuse.

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

Every Name Lease ends unless renewed by its controlling authority under the
accepted P4-D3 lifecycle below. Names are not owned forever merely because they
were claimed first. Exact numeric durations remain protocol parameters. A
Resolver cannot invent lifecycle state when current state is unresolved.

An active parent Name Authority may authorize creation or delegation of bounded
subordinate names within its own subtree and bind each to a chosen Name Authority.
It gains no authority over siblings or ancestors. P4-D3 below fixes the parent,
child, and lease lifecycle relationship after delegation.

Permissionless does not mean zero-cost or unlimited. P4-D6 below fixes a bounded
anonymous anti-squatting and Sybil cost that does not require a global User
account, identity document, money, IP reputation, mandatory wallet, token balance,
or one registrar. R-010 and later mechanism research must select and measure it;
no cost mechanism, consensus family, ledger, auction, or proof system is selected.

### P4-D2c — Explicit ASCII hierarchical syntax

**Product Owner decision, accepted 2026-08-08:** a canonical V1 Service Name is
one or more non-empty labels separated by dots. The rightmost label is the root;
each label to its left is subordinate to the suffix on its right. For example,
the Name Authority controlling `alice` may issue `blog.alice` but gains no
authority over any sibling root.

Canonical labels are serialized only with lowercase ASCII letters `a-z`, digits
`0-9`, and hyphen `-`. Unicode, IDNA, and Punycode are not valid canonical V1
Service Name forms and cannot create parallel Namespace entries. Applications
may display Unicode titles, but those titles are Application Data rather than
resolvable names. Exact finite limits on label length, total serialized length,
and hierarchy depth remain protocol-validation parameters; no implementation may
leave them unbounded.

The name itself is `alice` or `blog.alice`. Its explicit shareable Service Link
is `ardents://alice` or `ardents://blog.alice`. The scheme identifies the Ardents
Namespace and is not part of the Service Name. Dot hierarchy does not make the
name an ordinary DNS name, allocate a public top-level domain, inherit DNS trust,
or permit a DNS query or fallback. Failure to parse or resolve the Ardents name
remains an explicit Ardents failure.

ASCII-only canonical form removes cross-script and Unicode-normalization
ambiguity but does not eliminate deception using visually or semantically
similar ASCII names. Clients must present the complete canonical Service Name or
Service Link where destination trust matters; P4-D6 below fixes squatting,
reserved-name, and deceptive-name policy boundaries.

### P4-D3 — Lease, generation, and record lifecycle

**Product Owner decision, accepted 2026-08-08:** every Name Lease follows three
observable states. It is **Active** during its normal term. At the end of that
term it enters a finite **Grace** period in which the existing Name Authority
alone may renew it, the current Name Record may still resolve, and the User and
Developer receive a visible expiry warning. A successful renewal during Active
or Grace preserves the same Name Generation and returns the Lease to Active.

At the end of Grace the Lease becomes **Released**. The name no longer resolves,
the former Name Authority loses its exclusive renewal right, and the name becomes
available to the ordinary first-valid claim process. No resolver or cache may
extend Grace locally. Exact Active term, renewal window, Grace duration, warning
thresholds, and time/convergence mechanism remain bounded protocol parameters
that must be selected and tested later.

Each accepted claim creates a new Name Generation. Every Name Record and Lease
transition is authenticated and bound to that generation. Reclaiming a Released
name never revives the preceding generation: all previous records, revisions,
renewals, delegations, and signatures remain invalid even if their bytes are
replayed. Within one generation, accepted Name Record revisions are monotonic;
a Resolver cannot roll back to an older revision or choose between conflicting
ones.

A subordinate Name Lease may end earlier than its parent but cannot remain valid
beyond the parent's lifecycle. While a parent is in Grace, a still-current child
may resolve with the inherited expiry warning. When the parent becomes Released,
every descendant stops resolving and renewing. A later claim of the parent starts
a new parent generation and does not revive any former descendant.

Cached naming evidence has finite freshness and cannot outlive the proven Lease,
generation, or parent state. A Resolver that cannot prove one current generation,
revision, and lifecycle state because evidence is stale, conflicting, partitioned,
or unavailable fails explicitly instead of returning a guessed Service Target.
The exact cache interval and state-convergence mechanism remain research inputs,
not selected technologies.

### P4-D4 — Name Authority rotation, transfer, and recovery

**Product Owner decision, accepted 2026-08-08:** ordinary rotation or transfer
requires the current Name Authority to authenticate one successor transition.
The accepted transition preserves the Name Generation, Lease, descendants, and
current Name Record while giving all future Name Authority power to the successor.
The former authority cannot later update, renew, rotate, transfer, or recover the
name merely by replaying its old credentials. The network treats transfer as the
same authority transition as rotation; identity, payment, sale, and legal
ownership semantics remain outside Ardents.

Recovery exists only when a Recovery Policy was committed before the incident.
The optional policy is bound to one Name Generation, survives ordinary rotation
and transfer, and defines a set of scoped Recovery Authorities, an authorization
threshold, and a finite visible delay. No Recovery Authority is implicitly a
User, registrar, project operator, or network-wide administrator. Exact key,
threshold, delay, and cryptographic mechanism remain bounded protocol parameters.

Adding, replacing, or disabling a Recovery Policy is itself delayed and visible.
The previously accepted policy remains effective until that change completes,
so possession of the current Name Authority cannot silently erase recovery.
Neither ordinary rotation nor transfer bypasses a pending policy change or an
already accepted recovery. Exact cancellation and contest rules must be
precommitted by the policy; the current Name Authority alone cannot cancel a
recovery intended to survive its compromise.

A policy-authorized recovery initiated while the Lease is Active or in Grace
enters **Recovery Pending**. Resolution and ordinary Name Authority transitions
fail closed for its finite delay because the current binding may be compromised.
The pending state may hold the name from becoming Released only until that fixed
outcome deadline; repeated initiation cannot extend it indefinitely. Recovery
participants can therefore cause a bounded denial of name resolution but cannot
silently redirect Users.

Successful recovery installs a successor Name Authority inside the same Name
Generation and permanently removes the preceding authority's future power.
Resolution resumes only after the successor authenticates a fresh monotonic Name
Record revision; a possibly compromised old binding is not silently reused. A
direct Service Target connection remains pinned and does not follow this process.

Existing Name-origin connections treat Recovery Pending as no-new-recovery state
and close within their finite binding deadline. After a fresh replacement record,
new connections may authenticate the new Target; old streams never migrate to it.

Without a precommitted usable Recovery Policy, lost Name Authority material has
no administrative recovery path. The name becomes claimable only if its Lease
eventually reaches Released. If a compromised authority keeps renewing or a
recovery threshold is itself captured, the network cannot infer the rightful
human controller: the attacker may retain or visibly obtain control under the
accepted rules. Recovery adds a bounded alternative authority, not proof of
personhood or guaranteed restoration.

### P4-D5 — Private Resolution without name secrecy

**Product Owner decision, accepted 2026-08-08:** V1 protects the association
between a querying User endpoint and an exact Service Name against any one
malicious ordinary Node. It does not promise that a Service Name, its existence,
or its popularity is secret. A short or predictable name can be guessed and
queried by anyone, and knowing it remains discovery rather than authorization.

The protocol exposes no list, search, recommendation, public browsing, or
plaintext-directory API for Unlisted Services. This product omission is not a
cryptographic non-enumerability claim. A naming participant that sees an exact
name or a stable name-derived lookup identifier may infer the name, count its
queries, and test dictionaries. P4-D6 below requires bounded Anonymous Cost
without claiming to make low-entropy names unguessable; R-010 remains the
mechanism and shared-capacity question.

Resolution uses multi-node knowledge separation. An endpoint-adjacent entry role
may observe the User endpoint's ordinary location and traffic metadata but does
not receive the Service Name or a publicly testable name-derived lookup value. A
naming participant may receive the exact name or lookup identifier needed by the
eventually selected mechanism, but receives no User location or network-generated
stable User identifier. No one ordinary role receives both views for one query.

V1 restricts the destination-aware Resolution role to Rendezvous-domain
identities, never an endpoint-adjacent domain, and excludes each resolution
identity/known family from acting as the same destination/context connection's
Rendezvous. The query remains cryptographically hidden from Entry; role
assignment alone is insufficient.

Resolution sessions and query-derived state do not cross Isolation Contexts in a
way that supplies a stable cross-context identifier. A naming participant may
still link repeated work within one context or for the same visible lookup value,
and timing, volume, retries, cache behavior, and query popularity remain metadata.
Colluding entry and naming roles, Correlated Control, or a Broad Traffic Observer
may correlate those views; V1 makes no stronger resolution-anonymity claim.

The local Resolver authenticates current Namespace state and returns the accepted
Name Record or an explicit failure. If the knowledge-separating resolution path
is blocked, unavailable, stale, conflicting, or invalid, the endpoint fails
closed. It never sends the name directly to a public resolver, DNS, ordinary
HTTP service, alternate namespace, or less-private fallback.

**Inference from the accepted Endpoint Location Privacy claim:** name claims,
updates, renewals, policy changes, and recovery operations also cannot use a
direct naming path that reveals the controlling endpoint's ordinary location and
Service Name to one ordinary Node. Naming state necessarily identifies the Name
Authority and generation being changed, so operations on one name remain linkable
as that name's authenticated control history. Exact routing, replication, DHT,
oblivious-query, PIR, or snapshot mechanisms remain unselected.

### P4-D6 — Non-administrative governance and bounded abuse cost

**Product Owner decision, accepted 2026-08-08:** no network administrator,
project operator, registrar, legal claimant, trademark process, or manual dispute
panel may delete, seize, block, transfer, or reassign a canonical Name Lease.
Canonical control follows only the accepted deterministic Namespace state and
its authenticated transitions. A name proves current network control, not human
identity, endorsement, legal ownership, or a right to application content.

The protocol may define a finite, transparent set of **Protocol-reserved Names**
or labels solely to prevent parsing, compatibility, or protocol ambiguity. The
set is fixed by an explicit protocol version, not chosen case by case for brands,
content, governments, or project preference. A change cannot silently seize an
existing Lease; an incompatible reservation or meaning is a visible protocol and
Namespace compatibility boundary.

Naming capacity is protected by bounded **Anonymous Cost** and local resource
admission for claim, renewal, resolution, recovery, and other state operations.
No mandatory cost may require money, a global account, identity document, IP or
source reputation, stable identity, cross-context linking, wallet, token balance,
or governance coin. The exact computation, memory, bandwidth, storage, delay,
quota, or proof mechanism remains R-010 and mechanism research, and must satisfy
accepted honest-client accessibility, privacy, resource, and performance bounds.

Anonymous Cost raises the price of mass Sybil work but does not prove one human,
one actor, fair allocation, legitimate use, or rightful control. A funded or
well-provisioned actor may still claim many names, squat, enumerate, or exhaust
finite capacity. Leases make abandoned capture reversible; they do not solve
scarcity or human disputes. The product makes no anti-squatting guarantee beyond
measured cost, finite lifecycle, and explicit overload or unavailability.

An initial-claim mechanism must not grant priority merely because an observer
copied a pending claimant's revealed name. Every candidate must model and test
front-running, observation, withholding, flooding, and partitioned ordering;
commitment, reveal, ordering, and proof details remain unselected. If no candidate
can bound trivial copying without violating privacy or accessibility, root-name
allocation must be redesigned rather than assigned to a central registrar.

An Endpoint Owner, Node operator, Application, or gateway may locally refuse or
hide a Service Name under its own explicit policy. That decision changes neither
the canonical Name Record nor what the complete name means to other compatible
clients. Ardents supplies no mandatory global content blacklist or name takedown
surface; Application authorization, moderation, and content law remain outside
the carrier's canonical naming state.

Namespace rules, compatibility inputs, and accepted state-transition evidence
are versioned and publicly inspectable without publishing query logs. No single
operator may unilaterally alter canonical rules or state. Exact multiparty update,
quorum, release, emergency, and fork governance remains R-012, but it must preserve
this boundary. A partition, rollback, capture signal, or incompatible fork is
shown as conflicting, unavailable, or explicitly different network state; a
Resolver never silently selects another fork or Namespace.

If no naming mechanism meets the accepted security, privacy, performance,
accessibility, convergence, and non-centralization gates, Ardents revisits or
removes permissionless human-readable root names. It does not satisfy the product
contract by adding one mandatory registrar, paid auction, token, administrator,
or resolver.

## Decision completeness

P4-D1 and P4-D2a through P4-D6 close the Service Name product contract. Exact
registry, consensus, ordering, clock, cache, anonymous-cost, lookup, routing,
cryptographic, recovery, and governance mechanisms remain downstream research
and must be rejected if they cannot implement this contract.

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

The product contract and observable adversaries are now complete. Candidate
prototypes must use synthetic names and authorities and retain reproducible
conflict, partition, stale-cache, recovery, governance, abuse, front-running, and
private-query evidence without recording real User activity. No candidate earns
production status merely by implementing the happy path.

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
- a Name-origin connection remains recoverable past Recovery Pending/Release or
  silently migrates its byte stream after a different-Target rebind.

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
- **Product Owner decision:** a canonical V1 Service Name is a lowercase ASCII
  dot hierarchy with the parent on the right. Unicode, IDNA, and Punycode are not
  canonical name forms; every length and depth dimension must be finitely bounded.
- **Product Owner decision:** `ardents://<Service Name>` is the explicit
  shareable Service Link. The similar shape does not invoke DNS, a public TLD,
  DNS trust, lookup, or fallback.
- **Product Owner decision:** every Name Lease moves from Active to a finite Grace
  period and then Released unless its current Name Authority renews it. Grace
  preserves exclusive renewal and resolution with a visible warning; Released
  state resolves nothing and permits a new claim.
- **Product Owner decision:** every accepted claim creates a Name Generation.
  Reclaim, including parent reclaim, never revives old records, signatures,
  delegations, or descendants. Record revisions within one generation are
  monotonic and stale or conflicting state fails explicitly.
- **Product Owner decision:** a subordinate Lease cannot outlive its parent.
  Parent Grace propagates a warning; parent Release disables every descendant.
- **Product Owner decision:** ordinary rotation or transfer is one authenticated
  successor transition inside the same Name Generation. The old Name Authority
  loses all future power; Ardents assigns no identity, payment, or ownership
  meaning to transfer.
- **Product Owner decision:** recovery exists only through an optional
  generation-bound Recovery Policy committed before the incident. It may require
  several scoped Recovery Authorities, a threshold, and a visible finite delay;
  it survives rotation, and changing it is also delayed and visible.
- **Product Owner decision:** valid recovery enters Recovery Pending and stops
  name resolution. Completion installs a successor in the same generation, but
  resolution resumes only after a fresh monotonic Name Record. Without a usable
  policy there is no administrative recovery.
- **Product Owner decision:** a Name-origin Destination Binding participates in
  the connection Work Safety Lease. Recovery Pending, Release, or a different
  Target stops new leg/recovery work and closes finitely without retargeting;
  explicit Target connections remain pinned and have no Name rescue.
- **Product Owner decision:** Unlisted Service Names have no network list, search,
  recommendation, or public browsing API, but are not secret capabilities. Short
  names, name-derived lookup values, and query popularity may be guessed,
  enumerated, inferred, or counted.
- **Product Owner decision:** Private Resolution uses multi-node knowledge
  separation so one ordinary Node receives either User location or the exact
  name/lookup view, never both. Query state and sessions cannot create a stable
  identifier across Isolation Contexts.
- **Product Owner decision:** colluding roles, Correlated Control, and a Broad
  Traffic Observer remain outside the resolution privacy claim. Unavailable
  private resolution fails closed without direct public resolver, DNS, HTTP,
  alternate-namespace, or other less-private fallback.
- **Inference:** authenticated operations on one name are necessarily linkable as
  its control history, but no direct naming path may also expose the controlling
  endpoint's ordinary location to one ordinary Node.
- **Product Owner decision:** no administrator, registrar, legal or trademark
  claimant, or manual dispute panel may seize, delete, block, transfer, or
  reassign a canonical Name Lease.
- **Product Owner decision:** a finite transparent Protocol-reserved Name set may
  exist solely for versioned protocol safety. It cannot become a discretionary
  brand, content, government, or project reservation list.
- **Product Owner decision:** claim, renewal, resolution, recovery, and naming
  state capacity use bounded Anonymous Cost and local resource admission without
  money, account, identity document, IP reputation, stable identity, wallet,
  token, or cross-context linking.
- **Product Owner decision:** Anonymous Cost raises mass-abuse cost but proves no
  personhood, fair allocation, legitimate use, or rightful control. Candidates
  must also bound observation-based front-running rather than rewarding copied
  pending claims.
- **Product Owner decision:** local filters may refuse a name but cannot alter its
  canonical meaning. Ardents supplies no global content blacklist or name
  takedown surface.
- **Product Owner decision:** Namespace rules and state-transition evidence are
  versioned and publicly inspectable without query logs. No single operator may
  change canonical state; partitions and incompatible forks remain explicit.
- **Product Owner decision:** failure to find a viable accessible, private, and
  decentralized naming mechanism reopens or removes root names rather than
  introducing a registrar, paid auction, token, administrator, or resolver.
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
- **Lowercase ASCII canonical names:** make comparison and cross-platform display
  tractable while excluding many natural-language names; ASCII lookalikes and
  semantic deception remain possible.
- **Canonical Unicode names:** improve language inclusion but add normalization,
  versioning, script-mixing, font, and homograph failure modes to the security
  boundary.
- **Explicit Ardents scheme:** distinguishes a Service Link from DNS without a
  public TLD; bare-name entry may remain an Ardents-client convenience only.
- **Immediate release at term end:** minimizes stale control but turns a missed
  renewal into abrupt outage and immediate capture risk.
- **Finite Grace:** preserves usability and recovery for the current authority,
  but lengthens the period in which renewal censorship and abandoned names remain
  visible.
- **Permanent control:** removes renewal outages but makes squatting, abandoned
  names, and compromised authority effectively irreversible.
- **Generation-bound records:** prevent pre-release records and delegations from
  replaying after reclaim, at the cost of requiring resolvers to prove current
  generation as well as record authenticity.
- **Current-key-only rotation:** has the smallest authority graph, but permanent
  loss has no recovery and compromise can retain the name through renewal.
- **Single recovery key:** is simple to operate but merely moves catastrophic
  loss and compromise to another single secret.
- **Precommitted threshold Recovery Policy:** can recover from current-key loss or
  compromise without a network administrator, but its threshold can deny service
  and eventually take control after a visible delay.
- **Registrar recovery:** may handle human disputes, but creates a mandatory
  identity, censorship, and redirection authority rejected by the product model.
- **Direct public resolver:** is operationally simple but links User location to
  the exact name and creates a query log, blocking point, and mandatory provider.
- **Multi-node knowledge separation:** permits a naming participant to process a
  name or lookup value without receiving User location, matching the accepted
  one-malicious-ordinary-Node claim but not collusion resistance.
- **Locally replicated naming snapshot:** can remove live query disclosure at the
  cost of distribution, freshness, storage, and easier bulk state analysis.
- **Oblivious query or PIR:** may hide more from naming participants but adds
  cryptographic, latency, bandwidth, abuse, and deployment cost that must be
  measured rather than assumed viable.
- **Unrestricted free operations:** maximize initial access but let inexpensive
  Sybils capture names and exhaust shared naming capacity.
- **Identity, payment, auction, or token admission:** may raise abuse cost and
  settle scarcity, but creates excluded identity, wealth, accessibility,
  surveillance, and governance roots.
- **Bounded Anonymous Cost:** protects finite work without a stable User identity,
  but cannot enforce one-person fairness and may still favor powerful attackers.
- **Global canonical blacklist:** enables uniform takedown but gives its controller
  censorship and reassignment power. Explicit local filters preserve canonical
  meaning without pretending every Node must carry every Service.
- **Unilateral project governance:** is simple during early development but cannot
  support the accepted decentralized product claim. Versioned inspectable
  multiparty governance retains visible capture, disagreement, and fork costs.

## Recommendation

Use the accepted separate Name Authority as the stable control root for each
Service Name inside the accepted canonical network-wide Namespace. Keep concrete
custody, registry, and recovery mechanisms reversible. Allocate root names as
renewable permissionless Name Leases under deterministic shared-state ordering,
with subordinate claims authorized only inside the parent subtree. Use the
accepted lowercase ASCII dot hierarchy and explicit `ardents://` Service Link.
Use the accepted Active, Grace, and Released lifecycle with generation-bound
monotonic records and parent-bounded descendants. Use authenticated successor
rotation plus the optional precommitted, delayed, threshold Recovery Policy and
fail-closed Recovery Pending state. Use Private Resolution with multi-node
location/name knowledge separation, explicit metadata limitations, Isolation
Context separation, and no less-private fallback. Use non-administrative naming
governance, finite technical reservations, bounded Anonymous Cost, explicit local
filtering, publicly inspectable rules and transitions, and visible forks. This
contract is now the gate for comparing naming mechanisms.

Confidence is high that Service Authority cannot also provide truthful recovery
from its own compromise. Confidence is low that a usable, privacy-preserving,
capture-resistant Name Authority lifecycle exists without a meaningful
governance or availability cost. The single canonical Namespace is also
unverified against partitions, capture, allocation abuse, and accessible
operation. The leased first-valid-claim policy is unverified against front-
running, squatting, renewal censorship, and affordable Sybil resistance; these
are central R-003 research risks. ASCII syntax reduces Unicode ambiguity but
cannot prevent ASCII lookalikes, misleading labels, or social-engineering links.
The lifecycle prevents indefinite local extension and cross-generation replay,
but concrete clock, ordering, convergence, cache, and renewal-censorship behavior
remain unverified. Recovery avoids one mandatory administrator but adds a
generation-scoped capture set whose diversity, threshold failure, delay, denial,
policy-change, and custody behavior remain unverified. Private Resolution matches
the accepted single-Node claim but remains vulnerable to guessed names, query
frequency analysis, colluding route and naming roles, Correlated Control, and a
Broad Traffic Observer. The cost and privacy of concrete lookup mechanisms are
unverified. Governance and abuse boundaries reject hidden administrative escape
hatches, but concrete Anonymous Cost, front-running protection, multiparty rule
updates, capture evidence, fork recovery, and accessibility remain unverified.

The strongest counterarguments are that two authorities are too complex for a
small V1 Developer journey and that one canonical Namespace creates a shared
Control Plane. The first cost is accepted because merging authorities makes the
catastrophe-recovery promise false. The second is bounded by non-administrative
governance and an explicit fork rather than resolver-dependent names. A
third is that permissionless first-valid claims reward front-running and
squatting; leases make that harm reversible but do not remove it. A fourth is
that parent expiry creates a cascading outage; allowing descendants to survive
would instead violate hierarchical control and confuse a later parent claimant.
A fifth is that the Recovery Policy creates another authority capable of denial
and eventual takeover; precommitment, threshold, delay, and visibility bound but
do not eliminate that power. A sixth is that the selected privacy boundary lets
a naming participant infer names and popularity; stronger hiding remains a
candidate enhancement only if it meets performance and abuse budgets. A seventh
is that identity or paid allocation could suppress some abuse more directly;
those approaches are rejected because their surveillance, wealth, accessibility,
and capture roots contradict the accepted product.

## Disposition

- State: `decided`; P4-D1 and P4-D2a through P4-D6 are accepted.
- `Name Authority` becomes canonical product language.
- `Namespace` now means the one canonical Ardents network-wide naming boundary,
  not a resolver-selected provider or local alias scope.
- `Name Lease` becomes canonical product language for time-bounded Namespace
  control by a Name Authority.
- `Service Link` becomes canonical product language for the explicit
  `ardents://<Service Name>` shareable form.
- `Name Generation` becomes canonical product language for one claim-bounded
  lifetime whose records and descendants cannot survive reclaim.
- `Recovery Policy`, `Recovery Authority`, and `Recovery Pending` become
  canonical product language for precommitted, scoped, visible name recovery.
- `Private Resolution` becomes canonical product language for the accepted
  single-ordinary-Node User-location/name separation, not name secrecy.
- `Destination Binding` makes the Name generation/revision→Target provenance of
  a Name-origin connection immutable and gives catastrophe rebinding a finite
  effect on live old-Target work without silently moving its stream.
- `Anonymous Cost` and `Protocol-reserved Name` become canonical product language
  for bounded non-identity abuse control and finite technical reservation.
- R-006 target lifecycle remains unchanged and now has a distinct naming
  continuity authority.
- No ADR: this research record is the reversible product contract; no concrete
  registry, ordering, cryptographic recovery, Anonymous Cost, governance, or
  protocol mechanism has been selected.
- No experiment and no production code.
