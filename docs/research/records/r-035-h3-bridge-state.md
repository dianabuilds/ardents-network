---
id: R-035
title: What exact transport-neutral Bridge state does Horizon 3 maintain?
status: decided
owner: product research
started: 2026-08-15
reviewed: 2026-08-15
---

# R-035 — Horizon 3 Bridge state

## Decision this unlocks

Freeze the transport-neutral Bridge Invite, finite Bridge Entry Set, exposure,
regime-change, contact, restart, expiry, replacement, and terminal-result
contract for the smallest Horizon 3 Stage 5 slice.

This record is documentation research only. Acceptance would close R-035 but
would not authorize a Stage 5 implementation brief, package, dependency,
binary, Adapter experiment, integrated campaign, or censorship-resistance
claim. R-033, R-036, R-037, and a later explicit implementation brief remain
independent gates.

## Current contract

The authoritative inputs are:

- [Horizon 3 scope](../../product/scope.md#horizon-3--closed-test-network);
- [J-06 degraded/blocked-path journey](../../product/journeys.md#j-06--continue-through-degradation-or-recover-from-a-failed-path);
- [Horizon 3 technical design](../../development/horizon-3-technical-design.md#stage-5--bridge-and-blocked-entry);
- [R-009 Bridge architecture](r-009-hostile-bootstrap-and-bridge-entry.md#bridge-entry-contract-for-option-c);
- [ADR-0005](../../adr/0005-route-domains-and-bounded-entry-exposure.md);
- [operating-model Entry exposure](../../product/operating-model.md#entry-exposure-and-isolation-contexts);
- [threat-model Bridge boundary](../../security/threat-model.md#bridge-entry);
- [R-032 recovery ownership](r-032-h3-same-connection-recovery.md);
- [R-033 Stage 5 research map](r-033-h3-stage-5-research-map.md); and
- [R-034 capacity sequencing](r-034-stage-4-bridge-capacity-sequencing.md).

The canonical glossary definitions are
[Entry Set](../../../CONTEXT.md#entry-set),
[Bridge](../../../CONTEXT.md#bridge),
[Role Domain](../../../CONTEXT.md#role-domain),
[Time Confidence](../../../CONTEXT.md#time-confidence), and
[Work Safety Lease](../../../CONTEXT.md#work-safety-lease).

The accepted architecture already fixes that ordinary and Bridge entry are
separate regimes for the same endpoint-adjacent Route role, every Bridge key is
eligible for exactly one adjacent Role Domain, and an Invite changes only one
finite installation-scoped Bridge Entry Set. It also fixes that a Bridge sees
the adjacent Endpoint address and traffic pattern but receives no Service Name,
Service Target, opposite endpoint, full Route, or Application Data.

R-035 must decide product state without selecting an Adapter. Candidate-specific
configuration validation, startup, readiness, carriage, shutdown, supply, and
selection remain R-036. Exact campaign clocks, resource budgets, samples, and
verdict rules remain R-037.

## Protected claim and honest limitation

1. **Information:** one Bridge contact must not reveal the selected Service
   Name or Target, opposite endpoint, full Route, Application Data, another
   adjacent Role Domain's membership, or another Isolation Context's channel
   state. Bridge Invite secrets and membership remain local and are excluded
   from ordinary diagnostics and evidence.
2. **Adversary:** a censor, malicious or unavailable Bridge, informed probe with
   a disclosed Invite, malicious local Application, or crash/restart that tries
   to cause role reuse, replay, deadline reset, or unbounded endpoint-adjacent
   sampling.
3. **Conditions:** authenticated current Network State and Time Confidence,
   exact Initiator-domain eligibility, the Adapter and controlled profile later
   accepted by R-036/R-037, uncompromised endpoints, unchanged Route and Service
   Connection authentication, and the finite state below.
4. **Measurement:** R-037 must observe every attempted contact and fallback,
   validate durable transitions across injected crashes, inspect every role-local
   view, and reproduce the terminal result independently.
5. **Limitation:** the Bridge and its observer see the Endpoint address, timing,
   direction, duration, volume, retries, and the fact that this Bridge was tried.
   Invite disclosure permits informed probing and address blocking. This record
   claims neither invisibility, indistinguishability, anonymity against traffic
   correlation, availability, nor a way to discover an unknown Bridge.

## Hypotheses

- **H1:** one two-slot Initiator Bridge Entry Set with one replacement per slot,
  one bounded ordinary-to-Bridge transition, and a persistent non-resetting
  contact ledger is enough to exercise import, expiry, restart, replacement,
  role conflict, blocking, and exhaustion without Adapter knowledge entering
  Route or Service Connection state.
- **H2:** the set must support all three adjacent Role Domains or transport-
  specific retry policy to be meaningful in H3.
- **H0:** no finite transport-neutral state can preserve the accepted exposure
  and fail-closed rules through restart and Adapter failure.

## Evaluation criteria

The decision is acceptable only if, before candidate or integrated experiments,
it:

- produces the same authenticated network, Target, Route shape, Service
  Connection, and Application-visible ordered byte stream or one existing
  classified terminal result;
- protects the information named above from the stated adversaries while
  retaining the Endpoint/Bridge metadata limitation honestly;
- fixes finite membership, replacement, retained storage, contact, retry,
  deadline, and cleanup mechanics without borrowing R-023 normal-startup
  numbers or R-037 campaign clocks;
- makes blocking, withholding, replay, expiry, role collision, restart,
  resource pressure, and cleanup independently falsifiable;
- requires no new operator, broker, distributor, account, governance root,
  public DNS lookup, or public Bridge supply for the H3 offline/file fixture;
- fits one Product Owner and Codex, strict bounded local state, standard-library-
  first implementation, and no dependency/binary/license decision before R-036;
- keeps candidate maturity, advisories, audit history, misuse resistance,
  privilege, maintenance, license, offline supply, and removal cost wholly in
  R-036 rather than treating popularity as state-machine evidence; and
- exposes no new public UX or Application Interface control. The developer
  surface is one strict file import and local classified result; public
  acquisition and accessibility remain explicit later-horizon work.

The exact useful-work, latency, bandwidth, CPU, memory, process, socket, disk,
sample, and availability thresholds are precommitted inputs from R-036/R-037.
R-035 passes only if those profiles can plug into the fixed counters and
terminal bounds below without changing state ownership or increasing exposure.

## Exact H3 decision

### One selected Role Domain

Stage 5 activates only the **Initiator Role Domain** Bridge regime. This is the
smallest slice that can prove a blocked client reaches the same authenticated
network and exact Target. Responder- and Introduction-domain Bridge Entry Sets
are absent, not empty aliases of the Initiator set.

An Invite naming Responder, Introduction, Rendezvous, Destination Resolution,
an unknown domain, more than one domain, or a Bridge identity/family with a
conflicting retained or live duty is rejected without changing state. The
negative proves domain separation only; it does not authorize service-side
Bridge behavior.

The set identity is exactly:

```text
installation identity × network identity × Initiator Role Domain × Bridge regime
```

Applications, Services, Targets, Instance generations, Isolation Contexts,
destinations, Invites, Adapters, and process restarts cannot create another set
or another contact budget.

### Logical Bridge Invite envelope

The H3 fixture imports one bounded, versioned signed envelope from a local file.
The concrete lab encoding and golden bytes must be frozen by the later Stage 5
implementation brief; they are not a public wire protocol. The envelope contains
exact canonical `signed_body` bytes plus one separate `signature`. The signature
input is the domain-separation tag followed by the exact `signed_body`; the
signature is never a field inside its own input. The signed logical body contains
exactly:

| Field | H3 meaning |
|---|---|
| `schema_version` | Exact H3 Invite schema; unknown versions fail closed. |
| `network_id` | Must equal the installed and current authenticated network. |
| `epoch_number` and `epoch_digest` | Must equal the current accepted Network Epoch; no stale-epoch grace. |
| `route_profile_id` | Must equal the requested qualified H3 Interactive profile. |
| `role_domain` | Exactly `Initiator`. |
| `bridge_identity` and `family_id` | Must match one authenticated Node Record and its declared family. |
| `node_record_digest` and `domain_proof` | Bind the identity/family to current Initiator eligibility and assignment. |
| `assignment_not_after` | Must equal the proven assignment bound. |
| `slot_generation` | `1` for first occupancy and exactly `2` for the sole replacement in this network/Epoch/domain slot. |
| `not_before` and `not_after` | Finite Invite validity fully inside Epoch, Node Record, and assignment validity. |
| `slot` | Exactly slot `0` or `1` of the sole H3 Bridge Entry Set. |
| `replaces_invite_id` | Empty for first occupancy; otherwise names the active Invite in the same slot. |
| `candidate_envelope` | Exactly one signed, length-bounded opaque byte string; only the R-036 validator may interpret it. |
| `issuer_key_id` | Identifies the Bridge Node signing key authenticated by the Node Record. |

The outer `signature` is verified with `issuer_key_id`. `invite_id` is the
SHA-256 digest of a distinct Invite-ID domain-separation tag followed by the
exact canonical `signed_body`; it excludes the outer signature and is derived,
not distributor-selected. The signature authenticates the opaque candidate
envelope, validity, slot generation, and replacement intent. The current Network
Epoch separately authenticates network membership and domain eligibility;
possession of a valid Bridge key alone cannot assign another role. No new
cryptographic primitive or public format is selected.

The envelope contains no User, account, Application, Service Name, Service
Target, Service Instance, Isolation Context, destination, route trace, public
distribution endpoint, DNS discovery instruction, proxy, fallback address, or
policy transition command.

R-036 defines the candidate-envelope schema and maximum byte length, but it
may not add Bridge selection, domain, regime, retry, deadline, exposure, or
replacement semantics. Its two candidates use separate signed Invite fixtures
and campaign configurations. The validator returns one selected endpoint and
verified configuration to the preselected Adapter, with no second candidate or
automatic fallback.

### Validation and atomic import

Import performs the following fail-closed sequence before any durable set
membership or network contact:

1. enforce the implementation-brief file-size limit, exact schema, canonical
   representation, and absence of duplicate, trailing, or unknown fields;
2. derive `invite_id` and verify the Bridge signature;
3. match network, current Epoch, Route Profile, Node Record digest, identity,
   family, assignment, and exact Initiator-domain proof;
4. require sufficient Time Confidence and a current interval wholly inside all
   authenticated terminal bounds;
5. reject identity/family collision with Direct Source Exposure, Interior,
   Rendezvous, Responder, Introduction, Destination Resolution, prepared-role,
   quarantined old-domain, or conflicting live Route state;
6. pass the opaque candidate envelope to the campaign's preselected R-036
   validator and require one verified endpoint/configuration without DNS,
   discovery, proxy, or fallback behavior;
7. apply the slot-generation, replay, capacity, and replacement rules
   below; and
8. write one crash-consistent immutable state generation and atomically publish
   it.

A failure before step 8 changes no active, pending, retired, regime, attempt,
deadline, or exposure state. Reimporting the exact active `invite_id` is
idempotent and cannot refresh validity, retry, order, deadline, generation, or
exposure. A slot generation inconsistent with exact first occupancy/replacement,
the same slot generation with different bytes, or a retired ID is rejected as
rollback/replay or malformed input.

### Finite Entry Set and replacement

The H3 Bridge Entry Set has exactly two numbered slots. Each slot accepts at
most one initial Invite and one replacement in one Network Epoch. Therefore one
Epoch can expose at most four distinct Bridge identities through accepted set
membership, and only two are active at once.

An empty slot accepts only generation `1` with an empty `replaces_invite_id`. A
full slot accepts only generation `2` naming that slot's exact active Invite.
There is no implicit eviction, least-recently-used replacement, random
sampling, set expansion, move between slots, or replacement chosen by a Bridge,
distributor, Adapter, Application, Target, or failure.

Replacement is an explicit Endpoint Owner action in H3:

```text
ABSENT -> VERIFIED -> ACTIVE -> DRAINING -> RETIRED
            \-----------------------------> RETIRED
```

- `VERIFIED` is durable but not contactable until atomic publication completes.
- `ACTIVE` may be selected only while every authenticated bound and local
  conflict check remains valid.
- replacement or expiry of an `ACTIVE` member stops new contacts first and
  moves it to `DRAINING`;
- a replacement remains `VERIFIED` and cannot become `ACTIVE` until all work
  derived from the old member is terminal;
- expiry/ineligibility while `VERIFIED` moves directly to `RETIRED`, consumes
  that slot generation and its sole replacement quota, and cannot reactivate;
- `RETIRED` keeps the bounded Invite ID, generation, identity/family, slot,
  exposure, reason, and terminal bound needed to reject replay. It retains no
  live Adapter process or reusable admission secret.

Same-Initiator-domain ordinary Entry membership is not a conflicting role and
does not reject import. The ordinary-to-Bridge regime transition nevertheless
cannot start a Bridge contact until ordinary-derived live work for that
identity/family is terminal. The same identity/family is never contacted
simultaneously through both regimes.

Expiry, changed Epoch, lost Time Confidence, revoked/ineligible Node state,
assignment drain, or a newly learned role/family conflict never selects a
replacement automatically. It stops new Bridge work and terminates existing
work no later than the earliest applicable Work Safety bound.

### Entry regime

Import never changes the entry regime. The installation begins an H3 attempt in
`ORDINARY`. It may enter `BRIDGE` exactly once for that attempt by either:

1. explicit Endpoint Owner action; or
2. a policy armed before the ordinary attempt that permits one transition after
   the external R-037 harness has produced the precommitted ordinary-entry
   blocked condition.

The runtime does not infer “censorship” from silence. A Bridge, Adapter,
Application, Service, or Route failure cannot change the regime. There is no
automatic Bridge-to-ordinary transition, alternation, or transport-family
cycling inside the attempt. Returning to ordinary entry is a separate explicit
Owner action after all Bridge-derived work is terminal; it cannot rescue the
failed operation and creates no fallback success.

The durable regime record contains its generation, prior regime, trigger class,
policy identity, transition time evidence, attempt identity, and applicable
deadline. Restart preserves it. A new Application or Isolation Context cannot
fork it. H3 permits exactly one Bridge attempt record per accepted Network
Epoch. Returning to `ORDINARY`, another Owner action, a new Application, or a
new Isolation Context cannot start another attempt or obtain another ledger in
that Epoch. A later authenticated Epoch may allow one new explicitly initiated
attempt only after old derived work and cleanup are terminal and new Invites
pass the complete validation sequence.

### Contact order, retries, deadline, and exposure

The sole per-Epoch Bridge attempt owns one durable ledger and one absolute
terminal deadline selected by the accepted R-037 campaign profile. R-035 fixes
the mechanics, not R-037's numeric clock:

- the complete contact-start order is slot `0` initial, slot `0` retry, slot `1`
  initial, then slot `1` retry; an absent/ineligible slot or a successful
  contact skips its remaining ordinal rather than reassigning it;
- each member therefore receives at most one initial contact and one retry, for
  at most four total contact starts in the Epoch's sole attempt;
- a retry uses the same Bridge identity and preselected Adapter configuration;
- candidate-internal packets or handshakes count inside that one contact and
  cannot create hidden Bridge-state retries;
- the ledger records a contact as exposed durably **before** any dial, DNS,
  listener, helper process, or candidate startup could occur;
- an uncertain crash after that publication consumes the contact; it is never
  replayed as if unexposed;
- contact or Adapter failure cannot replace a member, change slots, switch
  Adapter family, return to ordinary entry, or reset any Route, Service
  Connection, Work Safety, attempt, or deadline state; and
- the earliest of attempt deadline, Invite expiry, Epoch expiry, assignment
  `not-after`, Work Safety expiry, cancellation, or local resource DRAIN stops
  new work.

R-037 must choose the exact attempt duration, per-contact sub-deadline, retry
spacing/hysteresis, and cleanup deadline before implementation. Every value must
fit inside the existing Service Connection recovery deadline when Bridge entry
is used during recovery; neither success nor retry extends that parent deadline.

The contact ledger has four fixed records. Each contains only attempt ID, slot,
Invite ID, Adapter profile ID, ordinal, start/terminal time evidence, bounded
outcome class, and exposure/cleanup facts. It contains no Target, Service Name,
Application Data, opposite endpoint, or full Route. Evidence exports use
manifest-scoped pseudonymous digests rather than Invite, address, or capability
bytes.

### Restart and retained history

Restart restores only authenticated durable configuration and bounded history.
It never restores a live Carrier Channel, Adapter process, listener, timer,
Route Attachment, or Service Connection.

On restart the owner:

1. validates the store generation and non-decreasing Invite/regime watermarks;
2. revalidates Network Epoch, Time Confidence, Invite validity, domain proof,
   assignment, and conflicts;
3. treats every contact published without a terminal record as consumed and
   `interrupted`;
4. marks the attempt `bridge-interrupted` because its Route/Service Connection
   consumer is gone; uncertainty also stops rather than granting time; and
5. resumes cleanup only. It never continues the contact sequence, revives live
   work, or starts a new attempt implicitly.

Current-Epoch retained state is bounded by four member records total: at most two
are `ACTIVE` and contactable, while at most two additional records are
`VERIFIED`, `DRAINING`, or `RETIRED` during/after the sole per-slot replacement.
It also contains four contact records, one regime record, and one attempt record.
When every derived Work Safety lifetime is terminal and the Epoch can no longer
be accepted, secret configuration and address material are removed; constant-
size generation/rollback floors may remain. Cleanup must not erase history early
enough to make a replay, reset, or cross-domain reuse appear new.

### Module ownership and outcomes

Transport-neutral Bridge state owns Invite validation, set membership, regime,
contact order, retry/deadline accounting, exposure, replacement, restart, and
exhaustion. It gives Route either one already established endpoint-adjacent
Carrier Channel or one classified terminal failure.

The Route Module still owns authenticated Route selection and the unchanged
Route shape. The Camouflage Adapter receives one selected endpoint and verified
candidate configuration and owns only startup, readiness, opaque bidirectional
carriage, shutdown, and cleanup. Service Connection owns continuity and its
non-resetting terminal bound. None of those consumers may mutate Bridge state
through a pass-through API.

Owner/import results are local and do not enter the Application Interface:

- `accepted` or `already-present`;
- `invalid` for malformed or unauthenticated input;
- `incompatible` for network, Epoch, profile, version, or time mismatch;
- `wrong-domain` or `conflicting-role`;
- `set-full` or `replacement-rejected`; and
- `expired` or `replay`.

Route-facing terminal results are limited to:

- `bridge-not-configured`;
- `bridge-ineligible`;
- `bridge-attempt-exhausted`;
- `bridge-deadline-exceeded`;
- `bridge-interrupted`; and
- `bridge-local-denial`.

An Adapter's remote close, block, probe, malformed carriage, or withholding is
not promoted to a guessed attacker diagnosis. The existing Application-visible
classes remain unchanged: verifiable inability to build the current path is
`Route unavailable`; indistinguishable blocking/withholding/outage is
`indeterminate failure`; the parent deadline is `timeout/cancellation`; and a
verified local resource refusal is `local denial/resource`. No Bridge identity,
slot, address, retry count, Adapter, topology, or regime control reaches the
Application.

## Falsification criteria

H1 is falsified if any conforming implementation or required campaign needs:

- more than one Initiator Bridge Entry Set, more than two active members, more
  than one replacement per slot/Epoch, or more than four contact starts per
  Epoch;
- a Responder/Introduction Bridge behavior in the maintained positive path;
- acceptance before complete authentication and atomic persistence;
- a public DNS lookup, distributor, broker, proxy, DHT, peer exchange, direct
  path, ordinary-entry retry, shorter Route, weaker profile, or second Adapter
  family to obtain success;
- Application-, Target-, Service-, destination-, or context-scoped Entry state;
- retry, exposure, regime, generation, replacement, or deadline reset after
  import, failure, cancellation, or restart;
- Adapter ownership of Bridge selection/policy or Bridge ownership of Route
  recovery/Service Connection continuity;
- use after Invite/Epoch/assignment/Work Safety expiry or unsafe Time
  Confidence;
- cross-domain or conflicting-role/family reuse;
- Bridge-specific detail in the Application Interface or secret-bearing
  evidence; or
- residual owned listeners, processes, files, queues, timers, sockets, or
  capability material after terminal cleanup.

If the two-slot contract cannot carry the later identical useful-work profile,
the result is `redesign`; H3 does not increase exposure until a new research
decision is accepted.

## Evidence plan

### Primary sources

Accessed 2026-08-15:

- the accepted Ardents records and contracts listed under Current contract;
- [The Update Framework specification 1.0.36](https://theupdateframework.github.io/specification/latest/)
  for signed, versioned, expiring metadata, canonical verification, rollback,
  freeze, mix-and-match, and fixed-start-time lessons; R-035 does not reuse TUF
  roles or formats;
- [Tor Browser Bridge manual](https://tb-manual.torproject.org/ru/bridges/)
  for explicit import of already-known non-public Bridge addresses and the
  limitation that an address may simply be unavailable;
- [Tor Pluggable Transport specification](https://spec.torproject.org/pt-spec/)
  for the separation between transport process lifecycle and the parent
  application's Bridge selection; candidate semantics remain R-036; and
- [I2P SU3 reseed specification](https://i2p.net/en/docs/specs/updates#su3-reseed-file-specification)
  for signed offline bootstrap material and explicit network-ID separation;
  R-035 reuses neither the SU3 format nor its signer model.

### Experiment

No experiment is authorized. R-037 must turn every transition and
falsification rule above into deterministic fixture cells before integrated
code. Generated Invites, keys, addresses, candidate configuration, captures,
evidence, state directories, and build outputs remain outside the repository.

### Failure scenarios

At minimum R-037 must cover malformed and oversized files, signature and body
tampering, duplicate/trailing/unknown fields, wrong network/Epoch/profile/domain,
expired/future intervals, stale generation, idempotent replay, retired replay,
set expansion, unauthorized replacement, identity/family collision, regime
oscillation, contact-order and retry reset, crash before/after every durable
boundary, withheld/slow/malformed Adapter behavior, expiry during contact,
resource DRAIN, cancellation, secret-bearing evidence, and incomplete cleanup.

## Findings

### Finding 1 — H3 needs one domain, not a generic Bridge product

**Sourced fact:** R-033 requires one selected adjacent Role Domain and separate
wrong-domain negatives; the accepted Route semantics already let a client prove
same-network/exact-Target behavior through the Initiator position.

**Inference:** Initiator is the smallest positive slice. Implementing service-
side Bridge sets would multiply state and topology without answering the Stage 5
question.

### Finding 2 — validation and exposure must be durable before contact

**Sourced fact:** ADR-0005 and the operating model make Entry exposure
installation-scoped and non-resettable by context. TUF's metadata workflow also
separates complete verification from use and retains rollback/freshness state.

**Inference:** a crash between dial and history publication would allow repeated
unrecorded probing. The contact therefore becomes consumed before candidate
startup, and an uncertain crash remains exposure.

### Finding 3 — replacement is not failure-driven rotation

**Sourced fact:** the Entry Set contract says one failure cannot force a fresh
untried candidate, while R-009 permits an Invite to add or replace only inside
the bounded set.

**Inference:** replacement must be explicit, slot-addressed, monotonic, and
atomic after old work drains. Two slots with one replacement each exercise this
behavior without inventing an open-ended public distribution lifecycle.

### Finding 4 — Adapter failure cannot define product state

**Sourced fact:** R-033 places candidate lifecycle and opaque carriage below
Route, while R-032 keeps Route recovery and Service Connection continuity above
Carrier behavior.

**Inference:** the Adapter may report only bounded lifecycle/carriage facts.
Bridge state owns the next precommitted contact, and neither layer may reset the
parent connection deadline or switch transport families.

### Finding 5 — blocked is sometimes unknowable

**Sourced fact:** the threat model requires indistinguishable censorship,
withholding, partition, and outage to remain `indeterminate failure`; R-009 says
an Endpoint cannot discover a secret address it does not possess.

**Inference:** controlled harness knowledge may label a censor profile, but the
runtime cannot turn silence into a censorship diagnosis or a reason for a
weaker fallback.

## Options

| Option | Product and security fit | Resources and availability | Operations, governance, supply, and accessibility |
|---|---|---|---|
| A — unbounded multi-domain pool | Violates domain and exposure invariants. | No finite storage/contact bound; apparent availability comes from unsafe resampling. | Requires an undeclared distribution/rotation authority and broad UX; maturity or licensing cannot repair the contract. |
| B — two-slot Initiator set | Preserves exact Target/Route/Application semantics and exercises required negatives. | Four member records, four contacts, one attempt/deadline; candidate performance remains measurable by R-036/R-037. | Offline/file input fits the one-to-one team; adds no operator or governance root. Candidate maintenance, audit, privilege, license, supply, and removal remain explicit R-036 gates. |
| C — one Bridge, no replacement | Preserves separation but omits accepted replacement/replay behavior. | Smallest state but cannot measure bounded alternate contact or replacement availability. | Simple import, but inadequate evidence would defer rather than close the product decision. |

### Option A — one unbounded multi-domain Bridge pool

Rejected. It collapses Role Domains, lets Apps/destinations expand exposure, and
cannot prove finite retry/replacement behavior.

### Option B — one two-slot Initiator set with durable bounded state

Recommended. It exercises every required state transition and negative while
preserving the Adapter, Route, Service Connection, and Application seams.

### Option C — one Bridge only with no replacement

Rejected. It cannot test order, bounded alternate contact, atomic replacement,
or retained replay history, all of which the accepted contract requires before
public work could build on the H3 result.

## Recommendation

Choose Option B with high confidence for Horizon 3 only. Freeze the logical
Invite and state machine above, then let R-036 define and compare exactly the two
candidate offerings and R-037 assign numeric clocks/resources and campaign
verdicts. A later implementation brief may map this contract onto cohesive
Modules only after every predecessor is accepted.

The strongest counterargument is that two slots and one replacement per slot
are too small for real censorship recovery. That is intentional: H3 must prove
bounded mechanics with one Product Owner and Codex. It is not a public Bridge
distribution, availability, or censorship-resistance qualification.

## Disposition

- State: `decided`; the Product Owner accepted the recommendation on 2026-08-15.
- Selected research recommendation: one two-slot Initiator-domain Bridge Entry
  Set, one replacement per slot/Epoch, one bounded ordinary-to-Bridge transition,
  at most four contact starts, and restart without state reset.
- R-036 still owns the Adapter interface, exact candidate configurations,
  pinned supply, comparison, and maintained selection.
- R-037 still owns exact numeric clocks, budgets, topology, observers, evidence,
  and `pass|fail|invalid` rules.
- R-033 is accepted; R-036 remains active and no Stage 5 implementation brief
  exists.
- This record authorizes no maintained code, package, dependency, binary,
  public protocol, or public security claim. The Product Owner separately
  authorized only the disposable R-036 comparison.
