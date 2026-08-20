# Stage 6 private naming evidence contract

Status: **accepted S6E1 development-evidence contract. The Product Owner
accepted R-042, R-044, R-045, R-055, and ADR-0017 through ADR-0019 and
authorized S6.3-S6.6 implementation on 2026-08-20. Maintained implementation,
mutation coverage, and the bounded command campaign are complete with independent
`pass`; Product Owner disposition remains pending. This is development evidence,
not a Qualification verdict.**

This document defines what Stage 6 must prove. Accepted records own persistence
properties, ordering, recovery cryptography, Anonymous Cost, and artifact
encoding; no storage engine, consensus system, or public wire is selected.

## Verdict meaning and trust split

Stage 6 uses three disjoint artifact authorities:

1. `manifest`: immutable schema/profile identity, fixture inputs, role views,
   expected runtime outcomes, hashes, and bounded clocks.
2. `evidence`: immutable ordered observations and cleanup inventory produced by
   the runner without verdict authority.
3. `verdict`: independent recomputation from the manifest and evidence.

The verifier returns:

- `pass`: every required artifact and behavioral predicate matches the manifest;
- `fail`: artifacts are valid, but at least one observed behavior breaches the
  Stage 6 contract;
- `invalid`: schema, provenance, hashes, sequence, clock bounds, required records,
  trust separation, or cleanup integrity is missing, ambiguous, or contaminated.

An expected runtime failure such as `state-conflict` can produce verifier `pass`
when fail-closed behavior is the cell's declared outcome. Command exit text is
never a verdict.

## Required artifact responsibilities and S6E1 schema

The authority split and canonical S6E1 manifest, evidence, cleanup, and verdict
bytes are frozen by R-055. Maintained structs and strict readers must implement:

- artifact schema/profile identity, run/cell identity, and source commitments;
- the accepted R-041 textual and wire name profile;
- fixture, authority, generation, record, target, parent, policy, and role-view
  commitments appropriate to the cell;
- the accepted R-042 eligible-set/order proof, monotonic clock origin, deadlines,
  and R-043 cache/replay bounds;
- the accepted R-046 role views, expected runtime class, and exact predicates; and
- separate paths and hashes for manifest, evidence, private fixture material, and
  verifier output, using the artifact commitment algorithm selected by R-055.

Evidence contains only declared per-role inputs/observations, ordered transitions,
terminal outcomes, resource observations, and complete cleanup inventory. It
cannot contain an expected verdict field or a mutable pointer to verifier output.

## Normative state dimensions

- Lease: `Active`, `Grace`, `Released`.
- Consistency: `Current`, `Conflict`, `Fork`, `Unavailable`.
- Recovery: `Stable`, `Recovery Pending`.

Conflict, Fork, and Unavailable stop resolution but do not alter Lease ownership or
cause Release. Recovery Pending stops resolution and ordinary authority changes
for one bounded policy deadline and cannot extend a Lease indefinitely.

## Failure classes

Runtime outcomes use at least:

- `invalid-input`: syntax, canonical form, bounds, schema, or field combination;
- `unavailable`: current authenticated state cannot be obtained;
- `stale-proof`: generation, revision, parent, policy, or cache proof is stale;
- `state-conflict`: accepted ordering cannot establish one current transition;
- `fork-unresolved`: partition or rule fork cannot establish one Namespace state;
- `recovery-policy-absent`: recovery was requested without an effective policy;
- `recovery-pending`: resolution or ordinary transition is blocked by recovery;
- `admission-denied`: the accepted bounded cost/resource gate rejects work.

These runtime classes are distinct from verifier `invalid`.

## Mandatory evidence matrix

Every row is a required immutable cell. Unless stated otherwise, correct handling
of the declared positive or adversarial fixture has expected verifier result
`pass`.

| Cell | Scenario | Expected runtime outcome | Expected verifier |
|---|---|---|---|
| A0 | Canonical name and `ardents://` link | One lowercase ASCII canonical name; deterministic round trip | `pass` |
| A1 | Uppercase, Unicode, IDNA/Punycode, empty label, bad hyphen, bad scheme, or exceeded finite limit | `invalid-input`; no state mutation or fallback | `pass` |
| A2 | Initial valid claim and exact-name resolution | New generation, Active Lease, revision 1, exact Target | `pass` |
| A3 | Renewal in Active and Grace | Same generation; Active; monotonic transition; Grace warning when observed | `pass` |
| A4 | Grace expiry, Released, and reclaim | Released resolves nothing; reclaim creates a new generation; old material is stale | `pass` |
| A5 | Parent delegation and parent lifecycle | Child is subtree-bounded, warns in parent Grace, and stops on parent Release; parent reclaim does not revive it | `pass` |
| B0 | Authority rotation | One successor in the same generation; predecessor loses future power | `pass` |
| B1 | Authority transfer | Same transition semantics as rotation, with no identity/payment meaning; delegation is not invoked | `pass` |
| B2 | Recovery Policy add, replace, or disable | Change is delayed and visible; preceding policy remains effective until completion | `pass` |
| B3 | Authorized recovery | Bounded Recovery Pending; resolution blocked; successor installed; fresh monotonic record required | `pass` |
| B4 | Recovery without effective policy or with insufficient authorization | `recovery-policy-absent` or authenticated denial; no admin/manual bypass | `pass` |
| B5 | Old authority, old policy, cancellation, rollback, and replay attempts | Explicit denial; no privilege restoration or silent policy erasure | `pass` |
| C0 | Routine Service Instance migration under the same Target | Name Record unchanged; Target authentication preserved | `pass` |
| C1 | Compromised/lost Target replacement | Fresh monotonic record binds the replacement Target; old binding is stale | `pass` |
| C2 | Existing name-origin connection during Recovery Pending, Release, or different-Target rebind | No new recovery work; finite close; stream never retargets | `pass` |
| C3 | Existing direct Target connection during naming changes | Remains pinned; no Name recovery or remap | `pass` |
| C4 | Concurrent claims with provable accepted ordering | Exactly one accepted claim and deterministic loser from authenticated order | `pass` |
| C5 | Concurrent/partitioned claims without provable order | `state-conflict` or `fork-unresolved`; Lease is not forced to Released | `pass` |
| C6 | Observation copying, front-running, withholding, rollback, and equivocation | No local-arrival priority or silent canonical branch; classified failure where order is unresolved | `pass` |
| C7 | Sustained claim/renew/resolve/policy/recovery pressure | Accepted measured Anonymous Cost and local limits remain bounded; excess work is `admission-denied` | `pass` |
| D0 | Resolver knowledge split | No ordinary role observes both User location and exact name/publicly testable lookup value | `pass` |
| D1 | Entry query hiding and Rendezvous restrictions | Entry lacks query value; destination-aware identity is in the allowed Role Domain and excluded family | `pass` |
| D2 | Claim/update/renew/release/policy/recovery control path | No ordinary role observes both controlling Endpoint location and name/control history | `pass` |
| D3 | Isolation Context separation | No network-generated stable query/session identifier links two contexts | `pass` |
| D4 | Missing/stale/invalid private path | Explicit classified failure; no DNS, HTTP, search, alias, alternate Namespace, or less-private fallback | `pass` |
| D5 | Stale cache, generation/revision rollback, parent replay, or reordering | `stale-proof`; no Target returned | `pass` |
| D6 | Incompatible Namespace rule fork | Explicit `fork-unresolved`/different-network state; no local branch selection | `pass` |

For every row, a valid artifact set showing behavior different from the expected
runtime outcome receives verifier `fail`. Missing or contaminated required
artifacts receive `invalid`.

D2 contains twelve exchanges covering the eight frozen claim, renew, record,
release, transfer, delegate, policy, and recovery families, including policy
add/disable and recovery initiate/cancel/complete/resume. Anonymous admission binds the digest of every
canonical static operation field, not a caller-authored label. Evidence retains
the predecessor and authority-produced canonical result records; the independent
verifier recomputes the admission binding, transition authorization, monotonic
state effect, and forbidden role-field separation.

## Privacy measurement contract

| Claim | Protected information | Adversary | Conditions | Measurement | Honest limitation |
|---|---|---|---|---|---|
| Private Resolution | Association of User endpoint location with exact name or publicly testable lookup value | One malicious ordinary Node | Valid role assignment, query hiding, family exclusion, no collusion | Per-role manifest and observed fields contain no forbidden pair | Collusion, Correlated Control, Broad Traffic Observer, timing/volume, dictionaries, and popularity remain |
| Private naming control | Association of controlling Endpoint location with exact name/control operation | One malicious ordinary Node | Accepted private control path and no collusion | Per-role operation fields contain either location view or name/control view, never both | Operations on one name remain linkable as authenticated name history |
| Cross-context unlinkability | Stable network-generated query/session identifier across Isolation Contexts | One naming or endpoint-adjacent role | Context separation and fresh bounded session state | Identifier equality/link test across contexts must fail | External timing and Application behavior may still correlate contexts |
| No name-secrecy claim | None | Any participant | Exact-name product contract | Evidence must not label names as secret or non-enumerable | Short names can be guessed, queried, counted, and dictionary-tested |

## Verifier predicates

The independently built verifier must recompute at least:

- exact schema/profile/source identity and immutable path/hash separation;
- canonical parsing and deterministic encoding from manifest bytes;
- legal state dimensions and transition order, including all monotonic deadlines;
- generation, revision, parent, policy, Target, and cache-proof freshness;
- predecessor and recovery-policy authorization at each authority transition;
- per-role allowed/forbidden field sets and cross-context identifier checks;
- expected runtime outcome for every required cell;
- the exact maximum-depth `127`-label Namespace proof size from its complete
  signed Record corpus, independently of a runner-authored counter;
- resource/admission observations against the future accepted C7 profile; and
- complete cleanup before the next cell and after terminal state.

The verifier cannot import runner command sets, trust runner summaries, read an
expected verdict from evidence, or write into manifest/evidence trees.

## Falsification and stop conditions

Stage 6 evidence fails if any valid cell shows:

- Conflict or Fork releasing or transferring a Lease;
- two accepted controllers for one complete name;
- renewal, record, descendant, policy, or cache reuse across generations;
- delegation escaping its subtree or outliving parent state;
- current authority silently erasing recovery or bypassing pending recovery;
- resolution resuming after recovery without a fresh successor record;
- a live stream silently retargeting;
- one ordinary role receiving a forbidden combined location/name view;
- a stable network-generated cross-context query identifier;
- public or less-private fallback; or
- unbounded, identity-linked, or unfrozen admission behavior.

## Accepted S6.0 decisions

The behavior inventory depends on these accepted decisions. Every row
contributes exact verifier predicates.

| Decision | Status | Evidence consequence | Record / ADR |
|---|---|---|---|
| Canonical name profile | decided | A0/A1 textual and wire vectors may be specified | [R-041](../research/records/r-041-canonical-name-limits.md) |
| Persistence property boundary | decided | restart/tamper/stale/atomic predicates may be designed; adapter evidence remains future work | [R-043](../research/records/r-043-persistence-restart-rollback.md) |
| Field-level role matrix and query hiding | decided | D0-D4 exact field predicates use R-046/R-047 and ADR-0014 | [R-046](../research/records/r-046-role-matrix.md), [R-047](../research/records/r-047-stage-6-query-hiding.md), [ADR-0014](../adr/0014-private-naming-ohttp.md) |
| Threshold Recovery Authorities | decided | B2-B5 use bounded individual Ed25519 signatures and delayed policy/recovery | [R-044](../research/records/r-044-cryptographic-suite.md), [ADR-0018](../adr/0018-threshold-recovery-multisignatures.md) |
| Claim ordering and inclusion | decided | C4-C6 use authenticated epoch input ordinal and explicit unavailable/fork | [R-042](../research/records/r-042-claim-ordering.md), [ADR-0017](../adr/0017-authenticated-name-claim-ordering.md) |
| Anonymous Cost and admission | decided | C7 uses the measured O1b per-surface profile | [R-045](../research/records/r-045-anonymous-cost.md), [ADR-0019](../adr/0019-bounded-anonymous-name-admission.md) |
| Artifact serialization | decided | S6E1 fixes canonical bytes, bounds, roots, clocks, and mutation classes | [R-055](../research/records/r-055-stage-6-evidence-serialization.md) |

A development fixture is citable only after the complete S6E1 campaign receives
an independent `pass`. S6E1 freezes one deterministic episode per A0-D6 cell;
it is not a qualification schedule. The Stage 5 R-037/S9.6 campaign is unrelated
and is not inherited by this evidence contract.

## Bounded campaign result

The separate launcher and verifier commands completed on 2026-08-20 with
`status=pass` and no diagnostics. The retained commitments were:

- source commit: `7b474ab020d29bfacf5ad2e9aad4ec63cf9c8499`;
- dirty-worktree digest: `5407c6f35f4cdfe9a7f2ed4c4cfa17ab0b5e2fac5bd1c25e0a3c7edbbdf9439d`;
- campaign: `d33428e3c0bf6dc5d86ee688a12d311cac643408471c7954f6b78551485c4083`;
- evidence: `138aff9d4b3e44f7139835bb32e3d4f3ce52947476d4b6357734e1c619b2dcfb`;
- verifier: `b1df251975c28f18e3d396abe65ddf0cea008d66796de184febd5fd528e118a8`.

Generated roots remain outside the repository as required. These commitments
identify the exact dirty development snapshot; they do not convert it into a
release or qualification result.
