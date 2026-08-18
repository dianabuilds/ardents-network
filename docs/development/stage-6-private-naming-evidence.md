# Stage 6 private naming evidence contract

Status: **revised and ready for Product Owner review; not yet accepted as the
coding gate. Exact S6.0 profile values and implementation artifacts are absent.**

This document defines what Stage 6 must prove. It does not select persistence,
consensus, cryptography, ordering, Anonymous Cost, or wire mechanisms.

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

## Required artifact schema

S6.0 freezes the canonical serialization and exact values. At minimum, every
manifest contains:

- `schema_version`, `profile_id`, `run_id`, `cell_id`, and source identity;
- Namespace rule version and canonical-name limit profile;
- fixture, authority, generation, record, target, parent, policy, and role-view
  commitments appropriate to the cell;
- deterministic sequence IDs, monotonic clock origin, deadlines, and cache bounds;
- expected runtime state/failure class and the exact predicates to verify; and
- separate paths and hashes for manifest, evidence, private fixture material, and
  verifier output.

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
- `admission-denied`: the frozen bounded cost/resource gate rejects work.

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
| C7 | Sustained claim/renew/resolve/policy/recovery pressure | Frozen Anonymous Cost and local limits remain bounded; excess work is `admission-denied` | `pass` |
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
- resource/admission observations against the frozen C7 profile; and
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

## Blocking S6.0 decisions

The evidence contract is structurally complete, but it is not executable until
S6.0 freezes exact name limits/schema, claim ordering proof, persistence and cache
model, cryptographic interfaces/mechanisms, Anonymous Cost parameters, role-field
matrix, clocks, and per-cell fixture hashes. These choices must be recorded before
implementation; they cannot be selected opportunistically by the runner or
verifier.
