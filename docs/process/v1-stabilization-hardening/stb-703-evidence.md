# STB-703 Evidence

Date: 2026-07-20  
Decision: accepted

## Capability

The bounded chaos campaign verifies that node, Waku network, workload, hosted
service, data, privacy, and diagnostics truth remain fail-closed and recoverable
under the 13 fault classes required by STB-703.

The fault definitions, blast radii, rollback paths, expected operator truth,
and per-fault results are retained in `stb-703-chaos-campaign.md`.

## Executed Docker Evidence

- CH-03/04/06/10 targeted batch: four packages passed, zero failed, 6.4 seconds
  wall time. It covered corrupt persisted node state, bounded envelope clock
  skew, hosted-service probe latency, and expired WSS certificates.
- CH-11 `NPI-001`: 1/1 passed; canonical report at
  `tests/.artifacts/reports/stb-703-npi-001/summary.json`.
- CH-02 isolated workload DIND: all nine `TestDockerExecutor*` tests passed;
  test time 11.6 seconds and wall time 61.7 seconds. Readonly filesystem,
  external network, tmpfs, PID, and OOM faults remained contained; OOM state
  was terminal and operator-actionable. Compose teardown removed its network,
  volumes, and containers.
- CH-12 `DAE-003`: 1/1 passed in 17.4 seconds; canonical report at
  `tests/.artifacts/reports/stb-703-dae-003/summary.json`.
- CH-01/05/07/08/09/13 `NFM-001`: 13/13 multi-host steps passed in 171.5
  seconds. The retained report is
  `tests/.artifacts/reports/stb-703-nfm-001/summary.json`, with topology,
  snapshots, Docker stats, versions, and compose logs beside it.

The multi-host campaign now sends a real `SIGKILL` to `a1` before recreating
the container. It also proves isolated-node packet loss and restricted defense,
segment partition/rejoin, loss of the seed, three churn cycles, dependency
restart recovery, and fresh-client Waku Store recovery.

## Resource And Cleanup Evidence

- During the only long run, the 60-second live check found all seven active
  topology/workload-engine containers healthy, CPU at 48.5%, approximately
  26.6 GiB RAM available, and disk activity at 1.7%.
- Other retained snapshots showed approximately 25.4–27.3 GiB RAM available,
  `vmmemWSL` at 2.6–4.4 GiB, and approximately 198 GiB disk free.
- Final `docker ps -a` filters found no STB-703 multi-host or workload campaign
  containers.

## Acceptance

- Faults were bounded, reversible, and confined to disposable Docker state.
- Corresponding runtime truth never remained healthy, available, or published
  while its observed prerequisite was absent.
- Pending/failing operations reached recovery or explicit terminal fate.
- No new foundation, dependency, product behavior, or deferred critical path
  was introduced.
- The ordinary restart step was strengthened into a reproducible crash
  regression and its scenario document was updated.

STB-703 is accepted with no open chaos finding.
