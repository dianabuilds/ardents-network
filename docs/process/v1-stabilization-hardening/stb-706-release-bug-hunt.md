# STB-706 Release Bug Hunt

Date: 2026-07-20

## Scope And Runtime Truths

The failure-oriented hunt covered startup/recovery, shutdown, local API truth,
Waku discovery and reachability, workload-backed publication, policy
enforcement, retained data, and repeated commands. The required truths were:

- failed durable commits never become successful in-memory terminal state;
- payload availability and durable metadata change atomically;
- duplicate commands do not rewrite authoritative history;
- closed runtime subscriptions cannot preserve stale reachability or spin CPU;
- deployment failure leaves no partial runnable topology;
- probes and policy decisions affect publication and behavior, not only logs.

## Closed Release Blockers

### BH-001 — Transfer history could contradict durable state

- Severity: high; release-blocking before correction.
- Trigger: make the state database unwritable during transfer start, progress,
  completion, or failure; or start a second transfer with the same ID.
- Previous outcome: the in-memory ledger could retain an uncommitted transition,
  and duplicate start could replace the original operation history.
- Correction: commit `f549a5b` restores the previous ledger/state on save
  failure and rejects duplicate IDs without mutation.
- Evidence: deterministic failure-path and duplicate-ID tests passed as part of
  `internal/data` in a Linux container.

### BH-002 — Blob payload and metadata could diverge on commit failure

- Severity: high; release-blocking before correction.
- Trigger: fail metadata persistence after storing a payload, or while dropping
  or expiring a retained payload.
- Previous outcome: store could leave an orphan payload; drop/expiry could
  remove the file while durable metadata still claimed local availability.
- Correction: commit `c03fa10` rolls back new payloads and stages removal by a
  private same-directory rename until metadata commits. Failed commits restore
  both the blob snapshot and payload filename.
- Evidence: the three focused failure scenarios passed (`0.048s`), the full
  `internal/data` package passed (`1.315s`), and production code-size checks
  passed in Linux containers.

## Mandatory Category Results

| Category | Probe/result | Release impact |
| --- | --- | --- |
| startup/recovery drift | Partial Compose startup failure was reproduced and now removes exact-project containers; retained data load/reconcile returns failures instead of ready. | Closed by `b561a06`; no open blocker. |
| shutdown persistence loss | Transfer/blob terminal writes now propagate save failure and retain the last committed snapshot. | Closed by `c720505`, `f549a5b`, and `c03fa10`. |
| stale/false local API state | Failed transfer and blob transitions restore prior observable truth rather than exposing uncommitted state. | Closed; no open blocker. |
| stale discovery after runtime change | Closed reachability subscription withdraws the claim to `unknown` and falls back to bounded reconciliation. | Closed by `a9d4fe8`. |
| publication/runtime mismatch | Hosted-service publication remains gated by real readiness/liveness probes; proxy connection failure does not manufacture readiness. | No finding. |
| trust/policy not enforced | Retention authorization is invoked before mutation; untrusted service and network paths remain behaviorally rejected by existing gates. | No finding. |
| corrupt/missing retained state | Missing payload is not treated as availability; failed metadata/payload transitions are atomic and explicit. | BH-002 closed. |
| repeated commands | Duplicate transfer IDs previously rewrote history; deterministic reproduction now returns an error with the original record unchanged. | BH-001 closed. |

## Diagnostics Assessment

The corrected paths return an error instead of silently continuing. Private
staging names contain no resource ID, are not considered available payloads,
and are removed during recovery if a process stops after the metadata commit.
No payload bytes, keys, peer routes, or retained resource identifiers are added
to diagnostics.

## Decision

The bug hunt has no unresolved release blocker. Findings outside these concrete
failure conditions are not added to STB-706 as speculative refactoring scope;
they belong to the subsequent refactoring plan unless another review proves
release impact.
