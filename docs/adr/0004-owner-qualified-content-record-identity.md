# ADR 0004: Owner-qualified Content Object and Manifest identity

- Status: Accepted
- Date: 2026-07-24
- Decision owners: Content, Identity, Persistence

## Context

Content Object and Manifest authorization already targets `(Owner Principal, kind,
local ID)`, but handlers and catalogue stores historically looked records up by
local ID alone. Consequently, an exact grant for Bob's `X` could resolve or
overwrite Alice's `X`.

Content References are different: they identify immutable payload bytes and are
globally content-addressed.

## Decision

A Content Object is identified by `(Owner Principal, Object ID)`. A Content
Manifest is identified by `(Owner Principal, Manifest ID)`. Object and Manifest
IDs are local to their owner.

The authenticated Effective Principal supplies Owner at the API boundary.
Protobuf Get requests continue to carry only the local ID: callers cannot select
another owner. Admission builds the policy target from `(Effective Principal,
kind, ID)`, and handlers must pass that same Owner and ID through domain lookup,
catalogue keys, overwrite checks, transfer, replication, and SDK-facing calls.
Nested Manifest references resolve within the containing Manifest's owner.

Catalogue schema v3 stores Object and Manifest maps under an opaque,
unambiguous encoding of the owner-qualified key. Loading schema v2 first
requires every legacy map key to equal the embedded local ID, then rekeys each
record from its embedded Owner and ID. This fail-closed check prevents malformed
legacy maps from silently changing identity. Migration validates the complete
snapshot before atomically writing v3; the original v2 snapshot remains the
rollback source.

## Consequences

- Alice's `X` and Bob's `X` can coexist and are isolated for Get and Publish.
- Lists exposed on a principal-owned API are owner-scoped.
- Content References and blob payload paths remain globally content-addressed;
  blob ownership bindings remain owner-qualified separately.
- Transfer and replication roots must carry a Manifest owner as well as its
  local ID.
- Replication schema v3 persists that owner on intents, snapshots, and repairs.
  Its v2 migration may resolve a legacy bare Manifest ID only when exactly one
  owner exists; this resolver is not available to runtime operations.
- Old software cannot read v3 state. Operational rollback restores the
  pre-upgrade v2 snapshot; a failed in-process migration never replaces it.

## Alternatives considered

Globally unique Object and Manifest IDs would simplify lookup, but would turn
caller-chosen metadata IDs into a global namespace and conflict with existing
principal-owned policy semantics. Retaining global ID lookup while checking
Owner after retrieval was rejected because overwrite and collision behavior
would remain ambiguous.
