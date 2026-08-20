---
id: R-046
title: What exact field-level role matrix enforces Stage 6 naming knowledge separation?
status: decided
owner: Product Owner
started: 2026-08-19
reviewed: 2026-08-20
---

# R-046 — Stage 6 field-level role matrix

## Decision this unlocks

Freeze the exact per-operation inputs, outputs, observations, identifiers,
Role Domains, known-family exclusions, and Isolation Context boundaries needed
by S6.2. The Product Owner accepted the matrix and its O1 recommendation on
2026-08-20.

## Current contract

R-039 requires that one ordinary Node never receive both an Endpoint location
and the exact Service Name or a publicly testable name-derived lookup value for
one operation. Names remain guessable; naming-side metadata, collusion,
Correlated Control, and a Broad Traffic Observer remain limitations. ADR-0005
restricts Destination Resolution to the Rendezvous Role Domain, excludes each
used resolution identity and known family from the same destination/context
Rendezvous, and forbids endpoint-adjacent overlap.

S6.2 owns knowledge separation and a fail-closed private exchange. It does not
select claim ordering, Anonymous Cost, Name Authority signatures, Recovery
Policy cryptography, Namespace replication, or evidence serialization. Those
later mechanisms may replace an opaque proof field, but may not widen any role
view fixed here.

## Hypotheses

- **H1:** four execution roles plus one bounded observer role, with separate
  resolution and control-operation payloads, enforce the forbidden combined
  view without a superset request object.
- **H2:** five operation-specific execution roles are required.
- **H0:** the current composition necessarily exposes a forbidden combined view
  and must be redesigned before Stage 6.

## Evaluation criteria

1. For resolve, claim, renew, record, release/transfer, delegate, policy, and
   recovery, list every semantic input and output plus cache, retry, error, log,
   and evidence fields.
2. Mark every field required, optional, or forbidden at each role; absence is
   enforced by concrete decoders rather than convention.
3. Bind identifiers to one role, operation, and Isolation Context; define their
   lifetime and permitted commitment.
4. Assign Role Domain and known-family exclusions to every network operation.
5. Missing or invalid role proof fails closed before plaintext disclosure or
   Lease mutation and maps to a bounded R-002 result.
6. State the protected information, adversary, conditions, measurement, and
   honest limitation for each claimed separation.

## Evidence plan

### Primary sources

Accessed 2026-08-20:

- R-003 and R-039 — privacy, lifecycle, and fail-closed contract;
- ADR-0005 — Role Domain and known-family exclusions;
- R-002 — bounded product-result requirement;
- R-026 and RFC 9458 — measured Client/Relay/Gateway knowledge split and its
  key-management, padding, replay, and collusion limitations;
- CONTEXT.md — Isolation Context and Destination Resolution Role.

### Experiment

Implement one bounded resolution tracer only after this record and the
query-hiding decision are accepted. Serialize every role's actual input,
output, local state, error, log, and evidence view. Mutation tests add every
forbidden field one at a time; the role decoder and an independent verifier
must reject it. Separate cells exercise cross-context reuse, known-family
overlap, replay, stale deadline, malformed padding, missing role assignment,
and forbidden fallback.

### Failure scenarios

- Endpoint-adjacent input, logs, or errors contain exact name/lookup bytes.
- Naming-side input or observation contains Endpoint location or a stable User
  identifier.
- Retry, cache, OHTTP configuration, or evidence identifiers distinguish one
  Isolation Context at a remote role or link different contexts.
- One identity or known family occupies conflicting Role Domains.
- Missing role proof reaches the private decoder or mutates a Lease.
- Error detail, telemetry, or a fallback path reconstructs a forbidden view.

## Findings

- **Sourced fact:** RFC 9458 separates Client origin from plaintext only when
  Relay and Gateway are distinct roles and warns that configuration choice,
  differential treatment, and traffic analysis can partition anonymity sets.
- **Sourced fact:** ADR-0005 makes Destination Resolution a Rendezvous-domain
  subrole and requires local exclusion from the same connection's Rendezvous.
- **Measurement:** R-026 Gate C passed fixed-size OHTTP role-split,
  modification, replay, nonce, stale, binding, offline-supply, and
  combined-view probes for one closed Ubuntu experiment.
- **Inference:** the local resolver is inside the Endpoint trust boundary and
  may see both local Application context and exact name. It exports neither.
- **Inference:** the authority-operation role receives naming semantics from
  the Gateway but no network peer information; a fifth network role adds no
  privacy boundary.

## Selected candidate matrix

The recommendation uses four execution roles and one observation role. `R`
means required, `O` optional, and `F` forbidden. A field not listed for a role
is forbidden. Semantic names are stable; later records may select canonical
proof bytes without changing visibility.

| Role | Trust/placement | Required Role Domain | Endpoint location | Exact name/control history |
|---|---|---|:---:|:---:|
| local resolver | Network-Isolated Application Boundary | none; not a Node | R, local only | R |
| endpoint relay | first network role | endpoint-adjacent Initiator duty | R | F |
| naming Gateway | OHTTP Gateway and Destination Resolution Role | Rendezvous | F; Relay origin only | R after decapsulation |
| authority operation | naming-side transition owner behind Gateway | no network duty | F | R for one operation |
| observer | bounded evidence consumer, off path | none | F | F; commitments only |

### Resolution request and response

| Field | Local resolver | Endpoint relay | Naming Gateway | Authority operation | Observer |
|---|:---:|:---:|:---:|:---:|:---:|
| `service_name_wire` | R | F | R | F | F |
| `isolation_context_local` | R | F | F | F | F |
| `network_id` | R | F | R | F | F |
| `operation = resolve` | R | F | R | F | F |
| `fresh_nonce[32]` | R | F | R | F | F |
| `deadline` | R | O, transport bound only | R | F | F |
| authenticated `gateway_key_config` | R | F except OHTTP key ID | O, own identity | F | F |
| `relay_endpoint` | R | R, own endpoint | F | F | F |
| fixed `gateway_endpoint` | R | R | R, own endpoint | F | F |
| opaque fixed-size request/response | O, transient | R | O before/after decapsulation | F | F |
| canonical `signed_record_chain` (child then parents) | O, success output | F | R or explicit failure | F | F |
| bounded resolution result class | R | O, transport class only | R | F | O |
| Endpoint/User/Application identity or address | O, local input | R, transport origin only | F | F | F |
| Service Target plaintext | O, after authenticated success | F | R inside record | F | F |

The Gateway accepts exactly one canonical request with the listed plaintext
fields. Unknown, missing, duplicate, reordered, trailing, non-canonical, stale,
or replayed input fails before lookup. A response binds the same nonce,
operation, network, deadline, name, generation, revision, and result. The local
resolver authenticates every record and the complete parent lineage before
exposing a Target.

### Control-operation common envelope

Every control operation uses the same endpoint-relay OHTTP boundary as
resolution. The relay sees only its Endpoint transport origin, fixed Gateway,
OHTTP key ID, deadline, ephemeral local handle, and opaque fixed-size envelope.
The Gateway strictly decodes plaintext and passes a new operation value to the
authority role without Relay identity, address, HTTP headers, transport handle,
timing, or OHTTP configuration.

| Operation | Exact naming-side semantic fields |
|---|---|
| claim | `operation`, `service_name_wire`, proposed `generation`, chosen `authority`, `lease_not_after`, `network_id`, `nonce`, `deadline`, opaque `ordering_proof`, opaque `admission_proof` |
| renew | `operation`, `service_name_wire`, `generation`, `expected_revision`, `lease_not_after`, `network_id`, `nonce`, `deadline`, opaque `authority_proof`, opaque `admission_proof` |
| record | `operation`, `service_name_wire`, `generation`, `expected_revision`, canonical `target`, `record_not_after`, `network_id`, `nonce`, `deadline`, opaque `authority_proof` |
| release/transfer | `operation`, `service_name_wire`, `generation`, `expected_revision`, optional `successor_authority`, `network_id`, `nonce`, `deadline`, opaque `authority_proof` |
| delegate | `operation`, `parent_name_wire`, `parent_generation`, `parent_revision`, `child_name_wire`, proposed `child_generation`, chosen `child_authority`, `lease_not_after`, `network_id`, `nonce`, `deadline`, opaque `parent_authority_proof`, opaque `admission_proof` |
| policy | `operation`, `service_name_wire`, `generation`, `expected_revision`, opaque canonical `recovery_policy`, `policy_not_before`, `network_id`, `nonce`, `deadline`, opaque `authority_proof` |
| recovery | `operation`, `service_name_wire`, `generation`, `expected_revision`, `policy_id`, `recovery_step`, `recovery_not_before`, `network_id`, `nonce`, `deadline`, opaque canonical `recovery_proof` |

The proof fields are explicit but their mechanisms remain open in R-042,
R-044, and R-045. They may not contain an Endpoint address, Application
identity, Isolation Context, Relay identity, transport handle, or remote retry
token. Each authority output contains only operation, name, generation,
resulting revision, bounded result class, and canonical accepted state or a
bounded public reason code.

### Identifiers, retry, cache, errors, logs, and evidence

| Surface | Permitted fields | Lifetime/boundary | Forbidden fields |
|---|---|---|---|
| local resolver session | local random handle, context, operation, name, deadline | one operation; never serialized | reuse across contexts; global User ID |
| local resolver cache | context, name, network, generation/revision, expiry, record | context-local and authenticity-bounded | cross-context key or remote cache signal |
| relay retry | local random handle, attempt ordinal, fixed Gateway, deadline | one operation; not forwarded | name-derived key, context/User ID, alternate Gateway |
| Gateway replay set | key ID, nonce, operation, expiry | deadline plus bounded skew | Endpoint/Relay stable ID, context |
| Gateway namespace cache | name, network, generation/revision, authenticated expiry | naming-side only | Endpoint/Relay identity, per-client partition |
| authority replay state | name, generation, revision, transcript commitment | control-history lifetime | Endpoint/Relay identity, context |
| network error | bounded stage and class | one operation | name, Target, proof, peer address, nonce |
| local error | bounded class plus local diagnostic | process-local only | secret key/proof, raw remote message |
| protocol log | role, operation class, result, local ordinal | bounded local retention | name, Target, peer address, nonce, context, envelope |
| retained evidence | schema, cell/ordinal, role, size/timing bucket, result, artifact commitments | one evidence run | name/Target, Endpoint address, context, stable session ID |

Retries always create a fresh HPKE context, nonce, session handle, and transport
state. No alternate Relay, Gateway, DNS, HTTP resolver, public directory, local
alias, previous success, or cached Target may replace a failed private
operation. Cached accepted state may avoid a network query only inside the same
Isolation Context while its authenticated validity and freshness hold.

### Role proof and family exclusions

- The local resolver validates finite Role Domain Assignments for Relay and
  Gateway before constructing plaintext.
- Relay must have an endpoint-adjacent Initiator assignment. Gateway must have
  a Rendezvous assignment covering the complete deadline.
- Identities and known families must differ. Neither may be a retained
  Direct-Origin Source. Gateway identity/family enters the destination/context
  exclusion set and cannot serve that connection's Rendezvous.
- Missing, stale, overlapping, wrong-domain, same-family, or unverifiable proof
  terminates locally before OHTTP construction or Gateway decapsulation.
- Reassignment follows ADR-0005 drain/quarantine rules and cannot rescue an
  in-flight operation.

## Options

### O1 — four execution roles plus bounded observer

Recommended. Each role owns one deep responsibility. Resolution never calls
the authority role. Control operations do not pass network metadata across the
Gateway seam. Observer consumes commitments and bounded classes only.

### O2 — five network execution roles

Rejected. Making the local resolver a network role invents another operator
but cannot remove the endpoint-local point where context and name meet.

### O3 — superset request plus runtime visibility checks

Rejected. Convention-based hiding permits leaks through errors, logs, retries,
and future call sites.

## Recommendation

Choose O1 and implement only the resolution vertical slice in S6.2. The shared
private-exchange boundary may later carry control operations, but their semantic
types stay in their owning slices after R-042/R-044/R-045 decide proofs. The
deletion test passes: removing role-specific types would require reintroducing
one superset object and convention-based field hiding.

Confidence is medium-high. The strongest objection is that types cannot prevent
one operator from controlling Relay and Gateway; Role Domain evidence and
declared families bound the honest protocol view but do not solve Sybil identity
or collusion.

## Privacy statement

- **Protected information:** association between one Endpoint's ordinary
  location and an exact Service Name or publicly testable lookup value.
- **Adversary:** one malicious ordinary Relay, Gateway, or authority Node.
- **Conditions:** valid non-overlapping assignments, distinct declared families,
  fixed-size query hiding, fresh state, no less-private fallback, no collusion,
  and no endpoint compromise.
- **Measurement:** forbidden-field mutations, role-view byte inspection,
  cross-context cells, family-conflict cells, replay/deadline tests, and packet
  proof that only the fixed Relay/Gateway path is contacted.
- **Limitation:** names and popularity are not secret; Gateway sees exact
  queries, Relay sees Endpoint metadata, timing/volume remain visible, and
  collusion, Correlated Control, Sybil families, endpoint compromise, or a
  Broad Traffic Observer may correlate the association.

## Disposition

- State: `decided`; the Product Owner accepted O1 on 2026-08-20.
- Selected option: O1, four execution roles plus bounded observer.
- S6.2 maintained implementation is authorized with accepted R-047/ADR-0014.
- R-042, R-044, and R-045 remain open. Their later fields may not widen this
  matrix.
