---
id: R-024
title: Is the Ardents product lifecycle complete and where are its bottlenecks?
status: decided
owner: product research
started: 2026-08-08
reviewed: 2026-08-08
---

# R-024 — Operational product closure

## Decision this unlocks

Determine whether Ardents has one coherent product from installation through
update and withdrawal, resolve contradictions between existing contracts, and
separate remaining product ambiguity from experiments, technology selection,
external validation, and actual launch dependencies.

The Product Owner instructed this pass to make bounded decisions without
stopping for approval on each parameter. The accepted result is the
[product operating model](../../product/operating-model.md).

**Scope clarification, accepted 2026-08-08:** this record checks whether the
eventual public lifecycle has missing contradictions; it does not promote every
closed decision into the current backlog. The authoritative delivery order is
[product scope](../../product/scope.md): Carrier Lab first, Named Unlisted Site
only after Route feasibility, and public/stable operating mechanisms only at
their promotion gates.

## Evidence used

- Tor keeps a small persistent guard set because repeatedly sampling fresh
  endpoint-adjacent relays eventually exposes every endpoint to an adversarial
  candidate: [Tor Guard Specification](https://spec.torproject.org/guard-spec/).
- Tor's current path-restriction work documents that observable rejection of
  particular Rendezvous or Introduction choices can reveal hidden Guards over
  repeated attempts:
  [Proposal 354](https://spec.torproject.org/proposals/354-relaxed-restrictions.html).
- Tor separates signed current directory state from the caches distributing it,
  applies validity windows, and selects paths from a common authenticated view:
  [directory outline](https://spec.torproject.org/dir-spec/outline.html) and
  [client operation](https://spec.torproject.org/dir-spec/client-operation.html).
- I2P RouterInfo and peer profiles demonstrate signed self-publication and local
  observation while documenting that signatures do not solve Sybil capture:
  [I2P network database](https://i2p.net/en/docs/overview/network-database/).
- The Update Framework defines separated metadata roles, thresholds, expiry,
  consistent snapshots, and rollback/freeze protections suitable for the
  release-authority boundary:
  [TUF specification](https://theupdateframework.github.io/specification/).
- Network Time Security authenticates NTP transport, while Roughtime provides a
  signed rough-time construction; neither makes one server an acceptable Ardents
  time root: [RFC 8915](https://www.rfc-editor.org/rfc/rfc8915) and
  [Roughtime protocol](https://roughtime.googlesource.com/roughtime/+/HEAD/PROTOCOL.md).

These are design references, not selected libraries or proof that an Ardents
candidate qualifies.

## Contradictions found and closed

### Hidden endpoint legs versus five different Nodes

The User and Service select hidden halves independently, so neither can compare
all five Node IDs. Service rejection of a proposed Rendezvous against a hidden
Entry Set would leak that set. R-004 P5-D6 now uses disjoint stable Role Domains;
P5-D7 and P5-D8 fix common View authority, local selection, and non-oracular
diversity behavior. The epoch commits a logical complete canonical Candidate
View, input-log cutoff/root, length, and global summaries. Ordinary endpoints
verify deterministic Candidate Materializations rather than claiming global
completeness from a shard; independent full auditors recompute the View. A
pre-cutoff valid Node Record must be included or receive a verifiable
deterministic rejection, although captured threshold signers can still deny or
fork the whole log.

### One malicious Node versus Node-plus-endpoint confirmation

The earlier “one malicious ordinary Node” wording incorrectly sounded as if it
also covered that Node operator acting as a User/Service or observing another
endpoint. An endpoint-adjacent Node plus controlled endpoint/probe source can
actively confirm a known Target through chosen low-latency timing/volume. The V1
guarantee is therefore role-local protocol knowledge separation only. Both
confirmation directions receive mandatory characterization evidence, but no
anonymity pass threshold; a stronger claim requires another Route Profile.

### Carrier privacy versus Application networking

A correct Route did not stop a Publisher from exposing an ordinary-network
listener, stop an ordinary client page from performing DNS,
external fetch, WebSocket/WebRTC, QUIC, or direct socket access, nor stop a
malicious User request from provoking Publisher callback/webhook/SSRF behavior.
The carrier now claims only traffic submitted to Ardents. A claim-bearing
private-application profile contains both endpoint Application/helper process
trees in Network-Isolated Application Boundaries with only scoped local IPC/
loopback, no ordinary ingress/listeners, deny-by-default egress, per-context
origin/storage, and no clearnet fallback. Public qualification of the Reference Application uses a controlled
single-response client and deterministic no-ordinary-network HTTP
Service. Generic adapters remain compatible but visibly lack the Application-
level claim; Ardents still does not become a mandatory browser/runtime.

### Isolation versus cumulative Entry exposure

An Entry Set per freely creatable Isolation Context lets a malicious or merely
busy Application force unlimited fresh Entry sampling. V1 now has only ordinary
and Bridge regimes and at most one small long-lived Entry Set for each activated
adjacent Role Domain × regime per installation: Initiator, Responder, and
Introduction. Co-resident client/publisher roles remain domain-separated. A
Bridge Invite proves one Bridge key's epoch-bound adjacent domain and changes
only that bounded set; Applications, Services, Targets, generations, contexts,
destinations, and Invites cannot create another. Contexts separate every higher
Route, destination, channel, key, session, and recovery state.

### Stable Role Domains versus temporal reassignment

A Node could otherwise flip domains at an epoch boundary while an old long-lived
Entry or Introduction duty survived. Every assignment is now finite; new work is
eligible only when its maximum duty/drain lifetime fits before `not-after`.
Reassignment publishes stop-new-work and quarantines the identity/known family
until all old-domain duties terminate. Emergency may close work and cause
unavailability but never authorizes overlapping old/new-domain eligibility.

### Direct bootstrap sources versus later private roles

A bootstrap/materialization/time/update source contacted before a Route may see
requester origin, then later combine that view with a destination-aware or Route
role. A globally advertised source identity/known family is therefore
incompatible with all Route and Destination Resolution assignments. An ordinary
candidate contacted directly is instead retained in one bounded installation-
wide Direct Source Exposure Set and locally excluded through the terminal bound
of every derived state/work item. Pre-contact selection rejects retained Entry/
Interior/Introduction/prepared-role and live-work conflicts; source and candidate
sequences are endpoint-precommitted and finite, with explicit unavailability on
collision or exhaustion. Every mandatory pre-Route artifact class needs three
beta/five stable effective authenticated source-only families under the normal
concentration caps. External/CDN distribution without authenticated family
evidence cannot count as independence and remains a first-contact limitation.

### One readiness flag versus different capabilities

Current state plus one reachable Entry did not prove private resolution,
qualified Route construction, descriptor publication, Introduction coverage, or
contribution eligibility. All capabilities share only a Common Readiness Base;
Target Connect, Private Resolution, Publish, and Contribute add independent
role-path prerequisites and explicit disabled, authority-locked, probation,
draining, stale, clock-uncertain, blocked, conflicting, incompatible,
update-unavailable/required, revoked, resource, and unqualified states. R-023's
existing `5 s`/`15 s` startup numbers mean Target Connect Ready only.

### Expiry versus untrusted time

Epochs, leases, descriptors, invitations, credentials, and updates all depended
on an unstated wall-clock assumption. Time Confidence now combines monotonic
runtime, a persisted non-decreasing watermark, authenticated epoch bounds, and
optional independent authenticated observations. Insufficient evidence fails
closed.

### Online root authority versus hostile Service hosts

The initial R-006 lifecycle placed permanent Service Authority on the V1 host
while the threat model treats host compromise as normal, then ambiguously treated
a Credential as if it were a secret. The runtime now holds a host-generated
private Service Instance Key and a public bounded monotonic Credential signed by
Service Authority. Copying the Credential alone grants no power. Connections and
recovery cannot outlive Credential validity or learned supersession deadlines.
V1 remains single-instance; concurrent multihoming is not smuggled in.

### Service administration versus Authority Custody

Earlier glossary wording put permanent Service Authority inside ordinary
Service Administration and collapsed the strongest local grant into runtime
publication control. The three boundaries are now explicit: Connection carries
bytes; per-Service Administration publishes/configures using an already
authorized public Credential and matching non-exportable Instance Key; separate
Authority Custody alone creates/imports/exports/reconciles/signs with roots,
issues Credentials, rotates/transfers Name Authority, and initiates Service
Target replacement. V1 does not rotate Service Authority in place: a replacement
root creates a new Authority and Target. Neither lower boundary can export a root
or Instance Key.

### Long-term authentication versus recorded past traffic

The earlier payload claim did not say whether later seizure of Service or Node
long-term keys decrypted recorded connections. V1 now requires fresh
authenticated ephemeral session/leg keys and best-effort erasure. Later
long-term-key compromise does not decrypt an honestly completed connection; live
endpoint compromise, memory/snapshot remnants, and post-compromise healing inside
an existing connection remain explicit non-claims.

### Root backup versus monotonic authority state

A root-only backup could be restored behind the network's accepted generation
and sign conflicting old state; Local Grants also cannot be reconstructed from a
root. The Authority Recovery Bundle now carries encrypted root material plus
authority-owned commitments and signing watermarks, excludes Grants/runtime
keys, and remains isolated/non-signing until authenticated reconciliation permits
a strictly higher generation or revision. Otherwise it is `authority locked` and
export-only.

### Uninstall versus a non-empty Authority Vault

A Recovery Bundle is an explicit Owner action and may not exist when uninstall
starts. Ordinary removal now preserves security watermarks and either retains a
non-empty Vault in place or blocks until an explicit Owner-chosen Recovery Bundle
export verifies; it cannot silently choose a secret or destination. An empty
Vault removes normally. Erasing authority/watermarks is a separately confirmed
destructive purge that enumerates the affected authority classes and warns that
recovery may be impossible.

### Qualification versus installed updates

The repository qualified candidate builds but had no trusted update, rollback,
drain, state-migration, or revocation contract. The operating model selects a
TUF-shaped release authority with `3-of-5` authorization for every public
executable digest, atomic local replacement, and separate protocol-transition
and build-safety machines. Normal protocol overlap is `90 days`; vulnerable or
revoked builds have no such entitlement. Every live workload has a finite Work
Safety Lease. Expired/revoked repair cannot use Ardents itself and has only an
external privacy proxy, explicit direct disclosure, or offline import.

### Diagnostics versus metadata privacy

Useful status was required, but logging and telemetry could recreate a
User/Service graph or cross Local Grant boundaries. Diagnostics now have four
separate audiences: connection Application, one Service administrator, Endpoint
Owner aggregate, and one Contributor role; Authority Custody is separate. Remote
telemetry and per-route/per-service logs are off by default, and previewable
exports omit authority, identifiers, grants, Entry membership, and continuity.

### Local grant revocation versus live child sessions

A revoked Application or administration grant previously left the fate of its
open handles undefined. Revocation now immediately blocks new work and
invalidates descendants; custody/admin sessions close immediately, while data
connections close immediately unless the Owner explicitly preselected a finite
drain-then-revoke action. Persistent policy can survive restart, but bearer state
cannot and must be rebound to the authorized OS-local principal.

### Per-Application grants versus one desktop user

A Unix UID or Windows user token often identifies all same-user applications;
PID, loopback, and copyable file tokens also do not prove one Application. A
claim-bearing Local Grant now binds to an OS-enforced or launcher-brokered
Application Principal covering one process tree/session. Qualification attempts
sibling attachment, bearer theft/replay, PID reuse, and restart. If a supported
platform cannot distinguish applications, they form one local trust domain and
the V1 model narrows to broker-launched/OS-isolated apps; generic same-user
adapters receive no malicious-sibling isolation claim.

### Endpoint roles versus public contribution

Running a public Contributor on the same V1 installation as a protected
Client/Publisher would add public reachability, self-selection, correlated-control,
and an unmeasured shared resource profile. Public V1 contribution therefore uses
a dedicated host/installation; development co-residence is unqualified, and an
Endpoint excludes its own controlled Node identities/families. Client+Publisher
co-residence remains supported with separate domain sets but no same-host
unlinkability claim and needs a combined R-023 profile.

### Human naming versus carrier progress

The canonical permissionless Namespace is a hard feasibility problem. It no
longer blocks the carrier: an explicit Target Link exercises the same exact
target authentication and Route without naming. A failed Name never silently
turns into that Target destination.

### Target Link versus descriptor-resolution privacy

Bypassing the Namespace did not bypass the need to find a Target's descriptor.
Every Target therefore uses Private Reachability Resolution. Destination-aware
resolution identities are restricted to the non-adjacent Rendezvous Domain and
excluded by identity/known family from that destination/context connection's
Rendezvous; the query is also hidden from Entry. This preserves four capacity
domains. A Node plus controlled endpoint can still traffic-confirm a known Target
and remains an explicit low-latency non-claim.

### Catastrophe name recovery versus live old-Target work

Rebinding a Name after Service Authority compromise did not previously constrain
an already-live connection authenticated to the old Target. A Name-origin
Destination Binding now joins the Work Safety Lease: learned Recovery Pending,
Release, or different-Target rebind stops new leg/recovery work and closes
finitely without retargeting. Explicit Target connections stay pinned and have no
Name rescue.

## Decisions by lifecycle concern

### Installation

Accept signed ordinary Windows/Ubuntu packages, unprivileged runtime by default,
local-only Application Interface, separate opt-in Publisher and Contributor
roles, distinct development/test/public roots, seven state classes, finite disk,
monotonic Authority Recovery Bundles with locked restore, no automatic
cloud/help-desk recovery, no account or automatic relay role, and a dedicated-host
rule for qualified public Contributors. Per-Application grants require an OS-
enforced or launcher-brokered Application Principal; indistinguishable apps form
one trust domain. Claim-bearing apps use the Network-Isolated Application
Boundary. Ordinary uninstall preserves security watermarks and either retains a
non-empty Authority Vault or blocks until an Owner-chosen Recovery Bundle export
verifies; destructive purge is separate and explicit.

### Warm-up

Accept threshold-authenticated Network Epoch plus independent byte distribution,
Time Confidence, non-decreasing freshness, a logical complete Candidate View,
proven endpoint materializations, transparent input and independent full audit,
the Common Readiness Base, separate Target Connect/Private Resolution/Publish/
Contribute readiness, and finite Work Safety Leases for live work. Direct-Origin
Sources are globally source-only or locally retained in the installation-wide
Direct Source Exposure Set; pre-contact collision checks, retries, and set
growth are finite. Every mandatory pre-Route artifact class has three beta/five
stable effective authenticated source-only families under the `40%`/`25%` caps.

### Operation

Accept the Tor-shaped split-circuit family, four Role Domains, domain-scoped Entry
Sets and Bridge proofs, endpoint-local selection, separate Introduction,
endpoint-only route continuity, bounded failure evidence, grant-scoped
privacy-safe diagnostics, explicit drain, and no Application operation replay.
Local Grant revocation recursively terminates or finitely drains child work, and
restart never revives an old bearer capability.
Destination resolution is a private non-adjacent role, and exact Name-versus-
Target provenance remains immutable for the connection lifetime. Every Role
Domain Assignment is finite; new duty must fit before its `not-after`, and
reassignment uses stop-new-work, drain, family quarantine, and only then later-
domain eligibility. Emergency closes work rather than overlapping duties.

### Security

Accept host-generated private Service Instance Keys plus public bounded
Credentials, deterministic Node eligibility and probation, resource-specific
anonymous admission, no Sybil/personhood claim, separate Control Plane roots,
`3-of-5` public epoch/release-root/new-executable baseline, `4-of-5` expiring
emergency action, Forward Secrecy from ephemeral authenticated session/leg keys,
transparent Candidate input, independent full audit, and an explicitly
centralized provisional test network during one-to-one development. Service
Administration never implies Authority Custody. Public family thresholds apply
after the maximum mandatory exclusion union, not to nominal pre-exclusion Node
counts. The Application Principal and Network-Isolated Application Boundary are
security conditions, not SDK convenience.

### Performance

Keep every accepted R-023 budget and interpret its current startup figures as
Target Connect Ready. Add budgets for every other enabled capability, finite
disk/update/diagnostic/recovery work, Candidate Materialization/audit, and
per-Role-Domain infrastructure workloads. Client+Publisher co-residence needs a
combined simultaneous profile; standalone maxima are not additive. Route
security is never weakened to pass. V1 has only the Interactive Route.
Established-work isolation does not guarantee a free slot: open-Service
viability still needs measured anonymous attacker cost-to-deny and useful new
admission under a declared hostile budget.

### Updateability

Accept automatic authenticated metadata checks but locally controlled package
execution; `3-of-5` new-executable authorization; reproducible packages and two
independent build attestations; separate protocol/build machines; `90 days`
ordinary current/previous protocol overlap; authenticated highest-compatible
qualified selection; capacity-before-required; finite signed work/deadlines;
atomic copy-on-write migration; bounded drain; safe rollback only; and explicit
`update required` or revoked results. Private-only is default with no direct
fallback; direct and offline modes are explicit. Direct sources still see IP,
platform, exact digest/release, timing, repetition, and download history.

### Privacy

Accept no mandatory telemetry/account/graph, per-context higher-state isolation,
bounded domain-specific Entry exposure, shared immutable public network state,
grant-scoped diagnostics, coarse Application errors, explicit update-distribution
metadata, direct-source observation/exclusion, and all existing honest
limitations for endpoints, Entries, naming participants, hidden common control,
and Broad Traffic Observation. Carrier privacy covers only bytes submitted to
Ardents; the stronger Application-level location claim additionally requires
the tested Network-Isolated boundary on both endpoint process trees.

## Disposition of the research queue

| Question | Product disposition after this pass |
|---|---|
| R-004 | **Decided candidate order:** Tor-shaped split circuits are first in Carrier Lab; no production family is frozen. Concrete component and qualification remain. |
| R-007 | **Decided:** positive-evidence failure matrix, bounded recovery, otherwise indeterminate; no semantic retry. |
| R-008 | **Decided:** Local Grants, separate authority custody, endpoint-bounded Entry exposure, per-context higher state, local-only diagnostics. |
| R-009 | **Decided:** threshold-authenticated Network Epoch, independent distribution, and Bridge Invites bound to exactly one Initiator/Responder/Introduction adjacent Role Domain. |
| R-010 | **Decided product boundary:** resource-specific staged admission and bounded Anonymous Cost; no universal identity, money, IP reputation, personhood, or fairness claim. Exact mechanisms remain experiments. |
| R-011 | **Decided product boundary:** logical complete Candidate View plus transparent input, proven endpoint materialization, independent full audit, local selection, hard identity/domain/family constraints, soft concentration evidence, and no User reputation graph. Exact log, weights, and thresholds remain measured parameters. |
| R-012 | **Decided product boundary:** separate threshold roots, expiry, transparency, delayed rotation, narrow emergency power, explicit forks, and no decentralized claim for project-only keys. |
| R-020 | **Decided for public promotion:** volunteer or institution-funded opt-in contribution; no token or payment. Every domain/subrole retains three beta/five stable effective families after maximum exclusions, and every mandatory pre-Route artifact class has three/five effective authenticated source-only families. Actual supply derives from fixed `x_d`; `15`/`25` are only all-zero-exclusion theoretical infrastructure floors. Insufficient independent role/source capacity blocks public launch. |
| R-005 | **Rejected for the first public product:** no second delayed/cover Route Profile. The Route Module seam remains. |
| R-021 | **Deferred outside the Product Core:** no retained delivery or replicated-content Overlay in the carrier. |
| R-022 | **Decided default:** Application-owned identity only; no shared network identity model. |
| R-015 | **Product behavior decided:** authenticated capability/profile negotiation, overlap, required/retired phases, and no downgrade. Encoding and conformance tooling remain technology work. |
| R-013 / R-014 | **Still open by design:** components and language are selected after bounded prototypes; they are not missing product behavior. |
| R-023 | **Active public-qualification gate:** budgets are decided but unmeasured; Carrier Lab uses only coarse decision metrics and earns no partial qualification. |
| R-016 / R-018 | **External-evidence gates:** target users and vocabulary are hypotheses; the one-to-one team cannot manufacture demand or novice comprehension evidence. |

## Public launch gates

The project may build research prototypes immediately. A qualified public
network additionally requires:

1. one Route candidate passing the complete R-001/R-023 matrix;
2. a naming mechanism passing its convergence, privacy, abuse, and governance
   gates before human names are called stable;
3. at least five real independent Control Plane custodians for the accepted
   thresholds;
4. at least two full Candidate View auditors independent from each other, the
   Network Epoch signer threshold, and the Candidate operator families they
   audit, with retained public inclusion, summary, concentration, and control-
   independence evidence;
5. an effective post-exclusion pool of at least three independent eligible
   operator families in every Role Domain and required subrole, including
   Destination Resolution, with no family above `40%` and workload reserve still
   available. “Effective” means after the profile's maximum permitted union of
   own-family, Direct Source Exposure, exact resolver-family, assignment drain/
   quarantine, and other mandatory local exclusions. Four exclusive domains
   have only a theoretical pre-exclusion beta floor of `12`. If `x_d` is the
   fixed maximum distinct excluded-family union in domain `d`, the actual route-
   family floor is at least `Σ_d(3 + x_d)` and capacity shares may require more;
   with one resolver-family exclusion and no others it is already at least `13`.
   Every mandatory pre-Route artifact class additionally has at least three
   effective authenticated direct-source-only families with no family above
   `40%`; the same three may serve several classes and count once, while
   external/CDN/file sources without authenticated family evidence do not count.
   Thus even with all `x_d=0`, the theoretical total infrastructure-family beta
   floor is `15`; with one resolver exclusion and no others it is at least `16`.
   Until every `x_d`, source class, and effective reserve passes, public beta is
   blocked;
6. reproducible supported-platform packages and two matching build attestations
   from builders independent of each other and of the release-Targets threshold;
7. hostile bootstrap/Direct Source collision and exhaustion, Candidate omission/
   withholding, private Name/Target/descriptor resolution, bidirectional Node-
   plus-endpoint active-confirmation, Role Domain reassignment, same-user
   Application Principal isolation, endpoint-Application network isolation,
   anonymous admission/cost-to-deny, clock, update, rollback, fork, drain, Work
   Safety expiry, uninstall/purge, and Authority Recovery Bundle drills;
8. measured startup/resource budgets for every capability called usable, plus
   qualified current-generation capacity and drain reserve in every domain/role
   before a protocol becomes required;
9. a passing combined Client+Publisher profile if that co-resident mode is called
   usable; public Contributor capacity comes only from dedicated V1 hosts;
10. external usability review and independent security review before high-risk
   safety or market claims.

For a stable decentralization claim, each Role Domain and required subrole has
at least five **effective post-exclusion** independent operator families with no
family above `25%` of eligible capacity and retains workload reserve. `20` is
only the theoretical pre-exclusion four-domain floor; the actual route-family
floor is at least `Σ_d(5 + x_d)`. Every mandatory pre-Route artifact class also
has at least five effective authenticated source-only families with no family
above `25%`; the same five may cover several classes and count once. Therefore
the theoretical all-zero-exclusion infrastructure floor is `25`, and actual
capacity may require more. These are release concentration gates, not proof
against concealed ownership.

## Final result

The public-product picture is closed enough to constrain bounded prototypes.
Only Carrier Lab is currently authorized; “closed” does not mean “implement all
mechanisms.” Remaining unknowns are visible feasibility measurements,
technology choices, real-world
operator/custodian supply, and external evidence. None authorizes a silent
security downgrade or invented decentralization claim.
