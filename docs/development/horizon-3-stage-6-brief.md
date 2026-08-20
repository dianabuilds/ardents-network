# Horizon 3 Stage 6 implementation brief

Status: **the Product Owner authorized the bounded S6.1 implementation slice on
2026-08-20. Stage 6 as a whole is not ready: R-042, R-044, R-045, R-046,
exact evidence serialization, and the general coding-start decision remain
open. ADR-0013 is withdrawn.**

Authoritative inputs: accepted ADR-0005, ADR-0009, ADR-0012, R-003, R-024,
R-026, R-039, R-041, R-043, the product contract, threat model, operating
model, H3 technical design, package map, and repository rules.

Blocking research/status references: open R-042, R-044, R-045, R-046,
withdrawn ADR-0013, the readiness checklist, and the S6.0 preparation summary.
They constrain entry status but are not accepted design authority.

## Current disposition

- R-039 accepts the bounded Stage 6 scope; it does not authorize implementation.
- The Product Owner recorded the maintained Stage 5 S5.1-S5.5 development
  `advance` on 2026-08-19. Full R-037 qualification remains an S9.6 gate and is
  not a Stage 6 predecessor.
- R-041 freezes the canonical name profile and R-043 freezes the semantic
  persistence boundary. R-042 ordering/inclusion, R-044 cryptography,
  R-045 measured admission, and R-046 field-level role views are open.
- ADR-0013 is withdrawn and authorizes no cryptographic dependency or interface.
- Existing Stage 6 foundation packages are work in progress, not evidence that
  the Stage 6 entry or completion gate has passed.
- The Product Owner authorized maintained S6.1 parser, encoding, lifecycle, and
  parent-binding work on 2026-08-20. This narrow exception depends only on the
  decided R-039/R-041 semantics and does not authorize cryptography, claim
  ordering, persistence adapters, role views, recovery, or later slices.

The general entry verdict remains **not ready** because four research decisions
and the evidence serialization are unresolved. The recorded S6.1 exception is
not a claim that the general entry gate passed.

## Entry gate

Stage 6 implementation may start only when all of the following are true:

1. R-033 through R-037 are decided and the Product Owner has recorded the
   maintained Stage 5 `advance`, including S5.4 and S5.5. **Satisfied
   2026-08-19.**
2. This brief, the development plan, readiness checklist, and evidence contract
   have been accepted by the Product Owner after review.
3. R-003 remains authoritative without a contradictory interpretation.
4. One accepted S6.0 decision profile freezes all six required boundaries.
   **Open: R-042, R-044, R-045, and R-046.**
5. Package and command ownership is factual in `package-map.md`; any new verifier
   package or command is added only with its implementation, tests, non-test
   caller, `doc.go`, and exact permitted imports.

No implementation slice may silently choose an undecided foundation.

## Outcome and completion levels

One Developer can claim or receive one exact canonical Service Name, bind it to a
current Service Target, and later migrate, withdraw, or replace that Target while
preserving the accepted name, authority, and connection semantics.

One User can resolve only an exact known name through the accepted private path
and receives an explicit success or classified failure. No list, search, DNS,
public resolver, recommendation, fuzzy match, local alias, or alternate Namespace
fallback is permitted.

Stage 6 has two non-interchangeable completion levels:

- **Slice complete:** the slice behavior and negative cases pass maintained tests
  against the accepted S6.0 decisions relevant to that slice.
- **Stage complete:** all slices are wired into the maintained service path; the
  immutable evidence campaign passes independent verification; J02, J03, and J05
  are traceable; and the Product Owner records the Stage 6 disposition.

Neither level creates an anonymity, non-enumerability, decentralization,
independent-operator, production-qualification, or release claim.

## Normative state model

Lease, consistency, and recovery are separate state dimensions:

- Lease: `Active -> Grace -> Released`, with renewal from `Active` or `Grace`
  back to `Active` in the same Name Generation.
- Consistency: `Current | Conflict | Fork | Unavailable`. A consistency failure
  stops resolution but does not release, transfer, or mutate a valid Lease.
- Recovery: `Stable | Recovery Pending`. Recovery Pending stops resolution and
  ordinary authority transitions for its bounded delay; it does not extend a
  Lease indefinitely.

`Conflict -> Released` is forbidden. Release occurs only through an authenticated
release transition allowed by the accepted lifecycle or through expiry after
Grace. Injecting conflict must never become a forced-release mechanism.

Each accepted claim creates a new Name Generation. Reclaim never revives records,
signatures, revisions, policies, delegations, descendants, or cache proof from an
earlier generation. Revisions are monotonic within one generation.

## Stage 6 slices

### S6.1 - Name contract encoding

- parse and validate the frozen lowercase ASCII dot hierarchy and explicit
  `ardents://` Service Link;
- reject Unicode, IDNA, Punycode, empty labels, non-canonical spellings, and every
  value outside the frozen label, total-length, and depth limits;
- encode Name Lease, Name Generation, Name Record, revision, parent, and schema
  version deterministically;
- preserve parent-on-the-right semantics and prohibit a child from outliving or
  being revived independently of its parent generation.

### S6.2 - Role separation in naming

- enforce separate field-level views for endpoint-adjacent, naming/Rendezvous,
  local resolver, authority-operation, and bounded observer responsibilities;
- cryptographically hide the exact name and publicly testable name-derived value
  from the endpoint-adjacent role;
- restrict destination-aware resolution to the accepted Rendezvous Role Domain
  and enforce identity/known-family exclusion for the same destination context;
- prevent stable query/session identifiers from crossing Isolation Contexts;
- apply the same location/name separation to claim, update, renew, policy change,
  release, transfer, and recovery operations;
- fail closed when the private path or role proof is unavailable or invalid.

Privacy claim: the protected information is the association between one User or
controlling Endpoint location and one exact Service Name/lookup value; the
adversary is one malicious ordinary Node; the conditions are valid role
separation, cryptographic query hiding, and no collusion; measurement is the
per-role field/observation matrix; limitations include collusion, Correlated
Control, a Broad Traffic Observer, timing/volume correlation, dictionary testing,
and the absence of name secrecy or non-enumerability.

### S6.3 - Target migration and continuity

- bind resolution to exact `Name generation/revision -> Service Target` evidence;
- keep Name Authority and Service Target Authority orthogonal;
- preserve the same Target during routine Service Instance migration without a
  naming update;
- require a fresh monotonic Name Record for a replacement Target;
- never rewrite naming secrets during Target migration;
- keep direct Target connections pinned and outside naming recovery;
- for name-origin connections, Recovery Pending, Release, or a different-Target
  rebind stops new recovery work and closes within the frozen finite binding
  deadline; an existing stream never retargets silently.

### S6.4 - Authority transition, delegation, and recovery

- implement rotation and transfer as authenticated successor transitions within
  one generation; the predecessor permanently loses future authority power;
- treat subordinate delegation separately from transfer: it grants bounded
  authority only inside the parent's subtree and cannot outlive the parent;
- support precommitted generation-bound Recovery Policy creation, replacement,
  and disablement with delayed, visible activation while the preceding policy
  remains effective;
- enforce precommitted cancellation/contest rules and prohibit current-authority,
  administrator, registrar, or manual bypass;
- bound Recovery Pending and require a fresh successor-authenticated monotonic
  Name Record before resolution resumes.

### S6.5 - Concurrency, fork, and abuse resistance

- distinguish a claim ordered by the accepted authenticated rule from a state where
  order cannot be established;
- give a deterministic loser only when authenticated shared state proves the
  ordering; otherwise expose Conflict, Fork, or Unavailable and fail resolution;
- cover observation copying, front-running, withholding, flooding, partition,
  rollback, equivocation, and incompatible rule forks;
- enforce the measured and accepted Anonymous Cost and resource-admission profile
  without money, global account, identity document, IP reputation, stable
  identity, wallet, token, or fairness/personhood claim;
- never select a canonical branch from in-memory race timing or local best effort.

### S6.6 - Independent evidence and verifier

- freeze canonical manifest, observation, cleanup, and verdict serialization
  only after R-042, R-044, R-045, and R-046 are decided;
- run the A0-D6 behavior inventory through a runner that cannot author a verdict;
- independently reconstruct every predicate from immutable raw evidence and
  return only `pass`, `fail`, or `invalid`;
- reject unknown, missing, duplicated, reordered, stale, contaminated, or
  non-canonical artifacts; and
- retain the exact profile identity, source commitments, clocks, observations,
  and cleanup inventory needed to reproduce the result.

## Pass conditions

Stage 6 passes only when all of these conjuncts hold:

1. Encoding, schema version, finite limits, hierarchy, and Service Link behavior
   exactly match the accepted R-041 profile.
2. Lease, generation, revision, parent/child, consistency, recovery, and cache
   transitions satisfy the normative state model, including explicit failure.
3. Rotation, transfer, delegation, policy mutation, and recovery have no stale-key,
   admin, rollback, cancellation, or privilege-escalation bypass.
4. Same-Target migration, different-Target replacement, name-origin connection
   closure, and direct-Target pinning preserve their distinct semantics.
5. Resolution and control-operation privacy meet the declared protected
   information, adversary, conditions, measurement, and limitation contract.
6. Every required evidence cell receives `pass` from the separately built
   verifier; malformed or contaminated artifacts receive `invalid`.
7. J02, J03, and J05 are traceable from maintained command entrypoints through
   naming state to terminal evidence.

## Stop/redesign conditions

- a global account, registrar, operator, manual panel, or administrator is needed
  for ordinary claim, renewal, transfer, release, or recovery;
- conflict or fork can release a valid Lease or silently select one branch;
- public DNS, search, recommendation, local alias, alternate Namespace, or a
  less-private path is used as fallback;
- one ordinary role receives both endpoint location and exact name/lookup view;
- query state creates a stable identifier across Isolation Contexts;
- Target or name-origin streams are silently retargeted;
- an unbounded or identity-linked Anonymous Cost is required; or
- implementation requires an unresearched storage, consensus, wire, cryptographic,
  or governance foundation.

## Evidence and artifact split

Stage 6 has three disjoint artifact classes:

- `manifest`: immutable schema/profile identity, fixture inputs, expected cell
  outcomes, role views, hashes, and bounded clocks;
- `evidence`: immutable ordered observations and cleanup inventory, with no
  expected verdict authority;
- `verdict`: independent recomputation to `pass|fail|invalid`.

The runner cannot author or mutate a verdict. Command exit text is never evidence
of Stage 6 success. The current A0-D6 inventory and draft responsibilities are
defined in `stage-6-private-naming-evidence.md`; serialization and exact
predicates remain open until the unresolved S6.0 research closes.
