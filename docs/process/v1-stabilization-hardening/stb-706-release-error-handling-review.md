# STB-706 Release Error Handling Review

Date: 2026-07-20

## Reviewed Failure Surfaces

The review traced load/save/restore, transfer and payload persistence, startup
and shutdown, Waku bootstrap/reachability, configuration rollback, Docker
deployment and workload start, publication, diagnostics refresh, and local API
result mapping.

## Closed Mishandled Errors

| Finding | Previous false outcome | Remediation |
| --- | --- | --- |
| transfer terminal writes | fetch could succeed while transfer truth remained pending | `c720505` propagates progress/completion/failure persistence errors |
| configuration rollback | rollback errors were reported as `rolled_back` | `f9c9fb3` adds `rollback_failed`, joined causes, and degraded diagnostics |
| transfer ledger mutation | failed save or duplicate start changed in-memory history | `f549a5b` restores the committed snapshot and rejects duplicate IDs |
| blob payload/metadata transition | failed save could orphan or delete a payload behind contradictory metadata | `c03fa10` makes store/drop/expiry failure-atomic |
| partial Compose startup | provisioning failure could leave project containers | `b561a06` performs exact-project cleanup and preserves both errors |
| workload container start | post-create start/inspect/proxy failure could leak a container and discard cleanup failure | `e83971e` performs bounded stop/remove and joins the safe cleanup error |

The workload correction is covered by a mock Docker API failure test and the
complete `internal/workload/execution` package (`0.112s`, Linux container).

## Reviewed Non-Findings

- DNS refresh is best-effort only at the caller: the callee withdraws obsolete
  discovered peers, records `dnsDiscoveryError`, shortens retry cadence, and
  affects bootstrap/readiness truth before returning the error.
- Diagnostics refresh may return an observation error, but the workload owner
  first records `workload.observation.refresh_failed`, degrades health, and
  marks operator-visible truth as potentially stale.
- Ignored writes in the CLI are terminal stdout/stderr rendering; buffer/hash
  writes are infallible by contract; test cleanup ignores do not affect product
  outcomes.
- Capability/publication failures that intentionally degrade instead of fail
  record an explicit reason, subsystem health, lifecycle impact, and event.

## Decision

No critical failure path reviewed here is converted into unexplained success,
and no unresolved release-blocking error-handling finding remains.
