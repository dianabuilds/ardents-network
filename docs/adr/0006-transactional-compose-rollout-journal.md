# ADR 0006: Transactional Compose rollout journal

- Status: Accepted
- Date: 2026-07-24
- Decision owners: Deployment, Operations

## Context

A rolling image change recreated and started a Node before adding it to the
automatic rollback set. A readiness failure could therefore leave the current
Node on the failed target digest while earlier Nodes returned to the fallback
digest. Process interruption could also lose the in-memory rollback set.

## Decision

Every supported Compose upgrade and rollback uses
`<StateDir>/rollout-transaction.json` with schema
`ardents.rollout-transaction/v1`. The journal records the action, target and
fallback immutable image references, phase, failure, and per-Node mutation
state.

Before the first destructive step for a Node, the orchestrator writes and
flushes its `mutation_pending` entry through an atomic same-directory rename.
The rename uses write-through semantics on Windows and an explicit parent
directory `fsync` on Linux. Recreation, start, and readiness are distinct
boundaries recorded as `recreated`, `started`, and `applied`. A Node becomes
`applied` only after readiness succeeds. The transaction becomes
`ready_to_commit` only after every Node is applied.

Any failure changes the journal to `compensating`. Compensation processes every
journalled Node in reverse order and recreates it at the one recorded fallback
digest. A Node becomes `restored` only after fallback readiness succeeds.
Failures remain as `rollback_failed`; successful compensation removes the
journal.

The cluster manifest is atomically replaced after the journal reaches
`ready_to_commit`, then the journal is removed. On restart, a journal whose
target is already present in the cluster manifest is a completed commit and is
cleared. Every other journal resumes compensation before a new rollout may
begin. A completed resume exits without starting the newly requested rollout,
so the Operator must invoke that operation again deliberately.

## Consequences

- The current Node is always in the compensation set before recreation.
- All compensated Nodes converge on one durable fallback digest.
- Repeating compensation is safe when interruption happened between a Docker
  mutation and its following journal update.
- Operators must preserve the journal during rollout incidents.
- Composite readiness remains a separate decision and implementation task.
