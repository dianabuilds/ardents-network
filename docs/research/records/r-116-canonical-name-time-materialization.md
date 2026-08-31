---
id: R-116
title: Canonical Name time materialization
status: decided
owner: Product Owner and Codex
started: 2026-08-26
reviewed: 2026-08-26
---

# R-116 — How does a canonical Name move from Active through Grace to Released in threshold-authenticated current Namespace state, without creating a hidden registrar or treating one Endpoint clock as global control?

## Decision this unlocks

Select the H4-4B lifecycle-materialization rule. It must make a current
canonical Service Name visibly enter Grace after its finite Lease, then stop
resolving when Grace ends, while preserving the rule that a resolver verifies
one authenticated current Destination Binding or fails explicitly.

The decision also specifies the smallest dependency that H4-6's authenticated
Epoch close must provide. It must not introduce a project operator, an
administrative registrar, or a new consensus system merely to advance a clock.

## Current contract

- [Naming](../../technical/naming.md) requires
  an Active → Grace → Released lifecycle, exact current binding verification,
  and no registrar discretion.
- The [threat model](../../security/threat-model.md) requires Grace to preserve
  resolution with a warning, Released Names to resolve nothing, and current
  generation/revision/Lease uncertainty to fail explicitly.
- [ADR-0020](../../adr/0020-authenticate-current-namespace-materialization.md)
  makes threshold-attested Epoch materialization, rather than a self-signed
  Record, the source of current state. [ADR-0022](../../adr/0022-bind-name-record-validity.md)
  makes Record validity fail closed at the signed limit. [ADR-0023](../../adr/0023-pending-signed-namespace-successors.md)
  preserves an authenticated transition before materialization.
- The retained technical contract now uses a V3 materialized resolution leaf
  whose authenticated lineage summary carries the earliest active-to-Grace
  boundary plus the finite `notAfter` boundary. A V1/V2 proof retains its
  historical static state.

No public Epoch producer, close transcript, shared clock rule, or public
Namespace governance is selected. H4-6A's alpha-control catalog does not
provide one.

## Hypotheses

- **H1:** the verifier derives Active, Grace, and Released from two signed
  deadlines in an otherwise current proof. The proof commits enough timing and
  freshness evidence that a bounded local clock can show Grace without a
  separate per-Name transition.
- **H2:** the authenticated Epoch close deterministically materializes the
  time transition from its declared cutoff/time rule. The resulting current
  proof carries the explicit Grace or Released Record state.
- **H0:** neither approach can meet the explicit lifecycle, fork, and
  no-hidden-control requirements. In that case canonical Name lifecycle must
  remain unavailable rather than presenting a partial public Namespace.

## Evaluation criteria

- At one declared decision time, every compatible Endpoint returns the same
  result for a current proof: active Binding, Grace Binding plus warning, or
  explicit unavailable. A proof cannot silently use a previous state.
- A Name Authority cannot extend, release, or reclaim a Name without its
  authenticated transition rule; a clock-only rule cannot let an operator pick
  a target or generation.
- A stale, replayed, withheld, equivocated, or forked time/materialization
  input fails closed. A released generation cannot revive an old Binding,
  delegation, or recovery state.
- Private Resolution continues to hide the exact name from the ordinary Relay
  under its stated single-Node adversary condition. The timing design must not
  send a new direct name query, DNS request, or HTTP fallback.
- The design states clock skew/tamper limits, proof freshness, offline
  behavior, storage/record growth, and the effect of a missing Epoch close.
  It must remain within the retained proof envelope or trigger an explicit
  compatibility/scale decision.
- The selected rule is implementable and operable by the actual project team;
  no assumed external validators, registrar staff, or governance body may be
  smuggled into the alpha claim.

## Evidence plan

### Primary sources

- [Naming and private resolution](../../technical/naming.md),
  accessed 2026-08-26.
- [Naming and private resolution technical contract](../../technical/naming.md),
  accessed 2026-08-26.
- [Threat model lifecycle contract](../../security/threat-model.md), accessed
  2026-08-26.
- [ADR-0020](../../adr/0020-authenticate-current-namespace-materialization.md),
  [ADR-0022](../../adr/0022-bind-name-record-validity.md), and
  [ADR-0023](../../adr/0023-pending-signed-namespace-successors.md), accessed
  2026-08-26.
- [RFC 3161](https://www.rfc-editor.org/rfc/rfc3161.html), accessed
  2026-08-26: a conventional time-stamping authority is a trusted third party
  that must use a trustworthy time source and signs a token.
- [Roughtime, draft-ietf-ntp-roughtime-19](https://datatracker.ietf.org/doc/draft-ietf-ntp-roughtime/),
  accessed 2026-08-26: authenticated rough time and inconsistency evidence are
  being specified as an experimental Internet-Draft, not a stable Ardents
  protocol dependency.

### Experiment

The maintained proof-level test
[`TestCurrentNamespaceProofDerivesGraceFromSignedDeadline`](../../../internal/naming/namespace/materialization_test.go)
uses actual signed Records, the persistent Namespace Store, a threshold
attestation, `Lookup`, and `VerifyBinding`:

1. materialize one active Name whose Lease and Grace deadlines differ;
2. verify at a time inside the signed Grace interval; the same active V3 proof
   yields its Binding with the Grace warning;
3. materialize a separately signed Grace revision in a later threshold Epoch;
   it remains compatible and yields the same warning;
4. verify past the Grace deadline and after a separately materialized Released
   revision; both fail.

Run it with:

```text
go test ./internal/naming/namespace -run '^TestCurrentNamespaceProofDerivesGraceFromSignedDeadline$' -count=1
```

This is a reproducible source-level measurement, not a live-profile or public
control experiment. It contains no participant data, credentials, captures, or
network listener.

### Failure scenarios

- An active proof is replayed after a different Endpoint has entered Grace or
  Released state.
- An Epoch closer withholds the required transition, chooses a false time, or
  emits two conflicting current states.
- An Endpoint's local clock is skewed, rolled back, or offline through a
  deadline.
- A parent becomes Grace, Released, or reclaimed while a child proof remains
  cached.
- Recovery Pending overlaps a scheduled Lease transition; a former Authority
  attempts renewal or a stale-generation reclaim.

## Findings

- **Measurement:** the initial maintained proof test established that the old
  V2 leaf made an active proof unavailable at `LeaseExpiresAt + 1 ms`. After
  the selected implementation, the same test proves that a V3 proof returns
  the same Binding with the Grace warning at that time. A separately
  materialized Grace revision remains compatible; proof verification fails at
  `GraceExpiresAt + 1 ms` and after a released revision. A second proof test
  proves that the earliest active parent deadline also produces the warning.
- **Current-code fact:** `record.applyAdvance` remains a retained explicit
  transition, but durable control has no arbitrary `advance` operation. V3
  proof derivation removes the need for one merely to expose Grace; it does not
  materialize Release or make a reclaim eligible.
- **Sourced fact:** RFC 3161 assigns time assertion to a trusted third party;
  adopting a TSA would therefore add a separate control/availability root,
  rather than eliminate the H4-6 question. Roughtime's current IETF document
  is experimental and describes authenticated *rough* time plus server-
  inconsistency evidence; it does not itself materialize an Ardents Namespace
  or decide its current binding.
- **Inference:** signed deadlines plus a threshold-authenticated V3 lineage
  summary implement Grace availability without adding a time authority. The
  remaining automatic Release/reclaim transition is still shared-control work.
- **Assumption:** any final selected profile has a bounded clock/freshness
  input. Its actual skew and availability budget are not yet selected.

## Options

1. **Verifier-derived lifecycle.** Keep a signed active Record and derive
   Grace/Released from its two signed deadlines in proof verification.
   This avoids per-Name close traffic, but must define compatible Endpoint
   clock bounds, how current-proof freshness limits old state, and how a
   derived Grace state interacts with an explicitly signed Release or recovery.
2. **Deterministic Epoch-close lifecycle.** An authenticated close uses a
   fixed cutoff/time rule to materialize Grace/Released records without a
   discretionary actor choosing their Name target. It keeps state explicit and
   consistent with ADR-0020, but needs H4-6's real close authority, withholding
   behavior, and cadence before H4-4B can operate.
3. **No canonical lifecycle until control exists.** Retain only finite alpha
   corpus expiry and explicit failure, then implement the canonical lifecycle
   with the selected H4-6 close. This is operationally honest but defers the
   H4-4B user outcome.
4. **External timestamp service.** A TSA or rough-time quorum could bound an
   Endpoint clock, but it adds time-server trust, availability, discovery, and
   privacy consequences. It cannot choose Namespace current state by itself;
   do not treat it as a shortcut around H4-6.

## Recommendation

Choose option 1 for H4-4B Grace resolution: derive it from signed deadlines
and the V3 threshold-authenticated lineage summary. ADR-0043 records the
decision. H4-6B must still select a close rule for explicit Release/reclaim,
conflicting close evidence, and public current-state control before those
parts of H4-4B can be qualified.

**Confidence:** high that the V3 rule implements the accepted Grace result
without a new controller. **Strongest argument against:** compatible Endpoint
clocks and proof-freshness bounds remain profile requirements; a bad clock can
show Grace too early or too late, so this is not a global-time claim.

## Disposition

Decided for H4-4B Grace semantics by ADR-0043. Retain the maintained
proof-level tests as behavior evidence. Do not add a project-controlled
`advance` service, browser workaround, temporary registrar, or public
Namespace claim. H4-6 remains responsible for the shared current-state
operations that proof-local deadline derivation cannot perform.
