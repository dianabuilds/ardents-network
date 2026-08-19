---
id: R-046
title: What exact field-level role matrix freezes Stage 6 resolution and naming control operations so no ordinary role receives both endpoint location and exact Service Name?
status: decided
owner: Product Owner
started: 2026-08-19
reviewed: 2026-08-19
---

# R-046 — Stage 6 role matrix

## Decision this unlocks

Freeze the role set, per-role field-level visibility, stable-identifier
rules, Role Domain assignments, identity/known-family exclusion,
Isolation Context boundary, and fail-closed behavior so the S6.2
(role separation) implementation can compile-check the privacy claim
and the S6.5 (concurrency/fork) implementation can reference the role.
Without this freeze, S6.2 would replace the contract with a convention
(`do not pass a superset object and hide fields by convention`) and the
verifier could not test the field-level boundary mechanically.

## Current contract

R-039 § Fixed product contract fixes:

- exact-name resolution and naming control operations separate endpoint
  location from the exact name/control view for any one ordinary Node
  under the accepted conditions;
- Private Resolution separates User location from the exact name/lookup
  view against one ordinary Node and across Isolation Contexts; names
  remain guessable, naming-side metadata and collusion remain visible
  limits, and no less-private resolver fallback exists.

`horizon-3-stage-6-brief.md` S6.2 fixes the goal and the high-level
separation. ADR-0005 fixes the Route Domain concept and its adjacency
rules. ADR-0009 fixes the Go project foundation (concrete types, not
interface hiding). R-005 fixes hostile bootstrap, R-035 fixes Bridge
state, CONTEXT.md defines `Network-Isolated Application Boundary`,
`Application Principal`, `Role Domain`, `Entry Set`, and `Destination
Resolution Role`.

What remains open before S6.2 can start is the exact matrix: which roles,
which fields per role, which rules bind identifiers and Role Domains, and
the fail-closed contract.

## Hypotheses

- **H1:** five named roles with per-role request/observation types and a
  single forbidden combined view are sufficient to satisfy R-039's
  privacy claim and the S6.2 / S6.5 dependencies.
- **H2:** a smaller role set with hidden fields by convention is
  functionally equivalent and faster to implement.
- **H0:** a superset object model is required to model the dynamic
  between resolver and authority work, so the contract cannot be
  per-role types.

## Evaluation criteria

1. **Per-role types are concrete Go types** (ADR-0009); no interface
   that hides fields by convention.
2. **Forbidden combined view:** no role sees both `User/Endpoint
   location` and `exact Service Name / lookup value` at the same time.
3. **Stable identifiers** (`query_id`, `session_id`, `nonce`,
   `ephemeral_handle`) are scoped to one role and one Isolation Context;
   they do not cross contexts, do not appear in evidence paths, and do
   not enter long-term state.
4. **Role Domain** assignments follow ADR-0005:
   `endpoint-adjacent` → Initiator / Responder; `naming-rendezvous` →
   Introduction / Rendezvous; `local-resolver` → Destination Resolution
   in the non-adjacent Rendezvous Domain only; `authority-operation` is
   its own state machine, not a Route role; `observer` is read-only.
5. **Identity/known-family exclusion** follows ADR-0005, R-005, and
   R-035: a Node identity and its known family cannot serve
   conflicting Role Domain duties in the same destination context.
6. **Isolation Context boundary** follows CONTEXT.md
   § Network-Isolated Application Boundary and the Application Principal
   definition: a role outside its boundary is fail-closed.
7. **Fail-closed behavior** on missing role proof, invalid signature,
   stale Role Domain, or expired role state: resolution stops, no Lease
   mutation, classified failure (per R-002 Connection Result taxonomy).

## Evidence plan

Primary sources, accessed 2026-08-19:

- R-039 — H3 private naming lifecycle (accepted 2026-08-17).
- `horizon-3-stage-6-brief.md` S6.2 (role separation).
- `stage-6-readiness-checklist.md` §B.6.
- ADR-0005 — Route Domains and Bounded Entry Exposure.
- ADR-0009 — Go Project Foundation.
- R-005 — Hostile Bootstrap and Bridge Entry.
- R-035 — H3 Bridge State.
- R-002 — Service Connection Connection Result taxonomy.
- CONTEXT.md — `Network-Isolated Application Boundary`, `Application
  Principal`, `Role Domain`, `Entry Set`, `Destination Resolution Role`.

The matrix itself is implemented in S6.2 against this contract; no new
experiment is required for R-046.

## Failure scenarios

- `endpoint-adjacent` receives the exact Service Name through carrier
  bytes, SOCKS logging, or an error path.
- `naming-rendezvous` receives the querying User location through an
  observation copy or timing side channel.
- A stable identifier from one Isolation Context appears in evidence,
  in another context, or in long-term state.
- A Node identity and its known family serve both `Initiator` and
  `Destination Resolution` for the same destination.
- Resolution proceeds after missing or invalid role proof.
- A Lease mutates because of a role-level error (release from
  conflict, claim from a stale generation).
- `authority-operation` receives Application-level data.
- `observer` reads query content, User location, or exact name.

## Options and recommendation

1. **Option A — five roles, per-role concrete types (recommended).**
   Each role is a distinct Go type for both request and observation
   shapes. The compiler rejects accidental superset passing. The
   forbidden combined view is a contract test, not a convention. The
   implementation in S6.2 mirrors the contract one-to-one.
2. **Option B — three roles with hidden fields by convention.**
   `endpoint`, `naming`, `authority` collapsed to three; one superset
   object with role-tagged visibility. Rejected: violates the brief's
   explicit `do not pass a superset object and hide fields by
   convention`; a logging or error path can leak a field; the
   contract cannot be compiler-checked.
3. **Option C — five roles with a superset object and runtime
   visibility checks.** Same five role names, one Go struct with
   `omitempty` per role, runtime guard. Rejected: same convention
   problem as B; a future maintainer can drop the guard or copy a
   field.

Recommendation: **Option A**, accepted by the Product Owner on
2026-08-19.

## Disposition

- R-046 becomes `decided`. The open row in `docs/research/questions.md`
  is updated to point at this record and the frozen contract.
- §B.6 of `stage-6-readiness-checklist.md` is checked.
- S6.2 (role separation) may define per-role request/observation
  concrete types and a verifier predicate against the forbidden combined
  view. S6.5 (concurrency/fork) may reference the role set when
  classifying the loser.
- This freeze does not authorize code; the Stage 6 coding gate remains
  closed until §B.2 through §B.5 of the readiness checklist are also
  checked and the corrected brief, plan, and evidence contract are
  accepted.
- No ADR is required: this is a contract freeze, not a technology
  selection that creates lock-in.
