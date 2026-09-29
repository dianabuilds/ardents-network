---
id: R-169
title: When may a local Direct Source collision guard be released?
status: decided
owner: Network State
started: 2026-09-29
reviewed: 2026-09-29
---

# R-169 — When may a local Direct Source collision guard be released?

## Decision this unlocks

Pin the release boundary for State-owned serving and outbound Direct Source
identity/family guards before repairing their lifetimes. This is a bounded
clarification of the selected local-role contract, not a new Source protocol.

## Current contract

[ADR-0005](../../adr/0005-route-domains-and-bounded-entry-exposure.md) excludes a
Direct-Origin Source identity and known family from Route and Destination
Resolution eligibility during overlapping exposure and retains a contacted
Source until derived state and work terminate. The
[threat model](../../security/threat-model.md#direct-source-separation-claim)
states the protected information (Endpoint origin and public artifact request),
adversary (malicious Source/ordinary Node or family), conditions, measurement,
and limitation. The [product scope](../../product/scope.md) bounds C0 to the
selected closed profile. The [Network owner](../../technical/network-route-node.md)
already requires the serving guard through a retired owner's joined Close after
uncertain conflict publication, but describes its normal lifetime as the current
Epoch validity window. Those cannot both define guard release while the Source
server can still resolve retained bytes.

## Hypotheses

- **H1:** Holding each guard through the joined completion of every dependent
  local operation, and through the authenticated terminal bound of any derived
  state, preserves the exclusion without changing Source wire behavior.
- **H2:** A guard can end at the Epoch validity or 15-second wave deadline if
  every dependent reader/contact is synchronously refused and joined then.
- **H0:** Neither strategy can preserve exclusion within the existing 64-record
  installation cap and recoverable local-role ownership.

## Evaluation criteria

The selected behavior must prevent a Route/Resolution collision while a Source
response or contact can still expose origin/artifact. A second owner must never
read `Conflict=false` for a protected identity/family, including at a boundary
second, after a successor A→B→C, or across close/reopen. Every admitted response
and finite wave must have a definite join; uncertain publication/recovery must
refuse new work. The installation-wide 64 Direct Source record cap remains hard:
exhaustion causes explicit unavailability before more exposure or publication.
No new network message, cryptographic primitive, dependency, operator, or
governance authority is acceptable for this correction. Local durable writes
and guarded predecessor retention may increase latency and reduce availability;
these costs must be measured in implementation. Accessibility and developer
cost favor one State-owned lifecycle with observable terminal results over
implicit time-only cleanup. Existing Go/runtime and license constraints apply.

## Evidence plan

### Primary sources

Accessed 2026-09-29 on published `dev` `9da5a055`: ADR-0005 and the current
threat model; `internal/network/state/{server,local_roles,source_wave,lifecycle}.go`,
`internal/network/duty/{store,persistence}.go`, and their behavior tests. The
current Network owner is a statement of intent; source and tests establish the
implemented behavior. No `old` branch or external protocol is needed.

### Experiment

Reproduce with deterministic clocks and a local role root: (1) open a serving
State at an Epoch validity boundary, keep its listener and a matching request
alive, then query Duty conflict; (2) hold an outbound wave after its journal
deadline, query conflict before the wave joins; (3) publish A→B→C while response
handlers retain each predecessor, checking every identity and family before and
after each join; (4) crash/reopen around guard, State pointer, and release
commits; (5) fill the cap and attempt replacement. Capture response status,
role-root generations, journal/pointer state, and `Conflict` results. A single
`ok`/contact while its collision is false falsifies H1's implementation.

### Failure scenarios

Clock boundary, blocked handler, delayed verification, cancellation, short
write or failed directory sync, changed family with stable identity and vice
versa, multiple rapid successors, cap exhaustion, crash before/after pointer
commit, uncertain recovery, malicious Source withholding, and another local
producer attempting the same identity/family.

## Findings

- **Sourced fact:** `state/server.go` selects `s.current` for `LATEST` or matching
  `BY_DIGEST` without testing its validity end. `state/local_roles.go` writes
  `direct-source/live` with `NotAfter=ValidUntil`. `duty/store.go` ignores an
  expired record on `Conflict` and prunes it on another producer's `Replace`.
- **Measurement:** The published `TestServingSourceDutyTracksAcceptedSuccessor`
  asserts no Duty conflict one minute after Epoch expiry while the serving State
  remains open. Its focused package test passed on `9da5a055` on 2026-09-29.
  This is a reproducible code/test contradiction, not a claim about deployed
  traffic or independent operator behavior.
- **Sourced fact:** `state/source_wave.go` records a finite 15-second cycle and
  initially retains `direct-source/exposed` only to its journal deadline. It
  separately extends exposure to a selected Epoch's `ValidUntil` on accepted
  pending/active publication. A journal deadline does not join a currently
  running contact or verification operation.
- **Measurement:** A disposable local test represented `cycleActive` and an
  `inFlight` attempt just after `cycleDeadline`; `duty.ReadConflict` returned
  false. It did not demonstrate a real goroutine crossing the deadline; the
  implementation test must supply that concurrency evidence.
- **Sourced fact:** `state/lifecycle.go` joins State work and its Source server
  before removing the serving producer. `duty/persistence.go` currently rejects
  even same-producer identity/family overlap, so it cannot retain guarded
  serving predecessors A→B→C as separate records unchanged. The Source-plan
  validator requires its two simultaneous Sources to differ in identity,
  family, endpoint handle, address, and key; distinct retained tuples from
  successive waves may still need same-producer overlap.
- **Inference:** A timestamp is a necessary bounded validity/exposure floor,
  but cannot by itself prove that all dependent operations have joined. H2
  would require a new synchronous refusal and drain at every boundary, and
  would alter current Source response behavior. H1 preserves the selected
  exclusion with the smaller observable behavior change.
- **Assumption:** The Product Owner's selected direction is to retain guards
  through joined dependent work. There is no selection here of a time witness,
  expired-Epoch response policy, new durable grammar, or stronger privacy claim.
- **Inference:** A crashed process has no resumable response handler once its
  exclusive root lease is reacquired; the journal and verified active/pending
  floors, rather than a nonexistent durable handler list, must decide whether
  an old guard also protects retained derived State. Ambiguous dependencies
  cannot be reclaimed automatically.

## Options

| Option | Fit and cost |
|---|---|
| H1: lifetime follows dependent work and derived state | Keeps the accepted exclusion and current Source response contract. Requires explicit State ownership of predecessor/attempt joins, conservative crash recovery, bounded local-role records, and some availability loss on uncertainty or cap exhaustion. No new governance root. |
| H2: deadline closes work synchronously | Could use existing time records, but requires denying previously readable Source bytes and aborting/joining wave work exactly at boundary. The Node Source mode has no selected live clock-observation input, so an expiry policy has an additional contract dependency. |
| H0: defer | Leaves a demonstrated local collision window; incompatible with ADR-0005 while the current server and wave remain enabled. |

## Recommendation

Choose H1 with high confidence in the invariant and moderate confidence in the
implementation shape. The strongest counterargument is that sticky predecessor
guards can fill the finite installation set and leave a crashed installation
unavailable until verified recovery. This is preferable to releasing a guard
while the same owner can still expose or serve dependent data. Accepted
[ADR-0118](../../adr/0118-retain-direct-source-guards-through-dependent-work.md)
pins the trade-off; exact storage mechanics belong to the bounded
implementation and must not weaken cross-producer collisions.

## Disposition

Decision accepted in ADR-0118 and promoted to the Network technical owner; no
runtime change or GitHub issue publication in this slice. Implement and verify
it under a selected C0 implementation issue. The disposable probe is
not maintained experiment code; replace it with deterministic package and
integration behavior tests, including a real blocked operation. R-167 remains
the selected active C0 research question.
