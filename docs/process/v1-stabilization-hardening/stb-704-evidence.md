# STB-704 Performance And Resource Safety Evidence

## Result

STB-704 is complete. The representative release profiles remained inside the
predeclared thresholds in `stb-704-performance-profile.md`. No product path was
replaced by a benchmark substitute: network measurements use the canonical Waku
transport, data measurements use encrypted Waku transfer and repair, and
workload measurements use the Docker executor.

## PERF-01: Two-Node Waku Relay And Store

Command:

```powershell
./tests/run.ps1 -Suite integration -Scenario NFI-007 \
  -ReportDir tests/.artifacts/reports/stb-704-nfi007-acceptance-final
```

The command ran in the canonical Linux test container with a 120-second host
timeout. Exactly one catalog entry ran and passed in 2.247 seconds.

| Metric | Measured | Threshold |
| --- | ---: | ---: |
| remote startup | 2.096 s | <= 15 s |
| local startup plus relay readiness | 82.4 ms | <= 15 s |
| local/remote peer count | 1 / 1 | 1-4 each |
| local/remote relay peer count | 1 / 1 | 1-4 each |
| 16-message encrypted batch | 1.299 ms | <= 5 s |
| batch throughput | 12,314.29 msg/s | >= 3 msg/s |
| delivery p95 | 1.156 ms | <= 2 s |
| Store query | 1.411 ms | <= 5 s |
| local/remote shutdown | 8.7 / 7.4 ms | <= 5 s each |

Evidence:

- `tests/.artifacts/reports/stb-704-nfi007-acceptance-final/summary.json`
- `tests/integration/network-foundation/performance_test.go`
- `docs/qa/integration/network-foundation-performance-baseline.md`

An initial draft used the already occupied `NFI-004` identifier and therefore
selected a second reachability test. The new scenario was moved to the free
`NFI-007` identifier; the final catalog selection contains exactly one test.
During code-size refactoring the static `testkit.Spec` was briefly moved behind
a helper, which made it invisible to the catalog extractor. The acceptance run
failed at selection before starting a test; restoring the literal at the
`BeginScenario` call fixed traceability and the final run passed.

## PERF-02 Through PERF-06: Existing Canonical Scenarios

Fresh evidence from the same code line was reused instead of rerunning broad
suites. Changes after these runs affected test documentation, the new
performance scenario, and multi-host orchestration only; they did not change the
measured data, hosted-service, workload, or network product paths.

| Profile | Evidence | Result |
| --- | --- | --- |
| hosted-service probe bound | STB-703 CH-06, `TestControllerReportsBoundedProbeTimeoutAfterWarmup` | a 100 ms dependency against a 10 ms deadline passed by reporting `probe_timeout`, never false readiness |
| DAI-003 encrypted chunked transfer | `tests/.artifacts/reports/stb606-integration/raw/TestDataSubstrateFetchesAndResumesChunkedPayloadOverPrivateWaku-1784547117182487370.json` | passed in 2.867 s, below 30 s |
| DAI-004 repair after corrupt replica | `tests/.artifacts/reports/stb606-dai004-final/raw/TestDataAvailabilityRepairsCorruptReplicaToDifferentWakuPeer-1784548716654495915.json` | passed in 16.542 s, below 60 s |
| Docker workload quotas and overload | `tests/.artifacts/reports/stb-703-workload-pressure/compose.log` | 9/9 passed in 11.633 s, below 30 s |
| seven-node multi-host topology | `tests/.artifacts/reports/stb-703-nfm-001/summary.json` | 13/13 passed in 166.9 s, below 240 s |
| multi-host container resources | `tests/.artifacts/reports/stb-703-nfm-001/docker-stats.jsonl` | maximum observed memory 51.2 MiB; maximum observed CPU 3.08% |

The workload profile includes readonly root enforcement, isolated networking,
tmpfs, PID limits, memory/OOM handling, CPU quota, immutable image admission,
and explicit unsafe-intent rejection. This is the quota/backpressure evidence
required by STB-704, not a synthetic resource mock.

## PERF-07: Bounded Stability Sample

The canonical multi-host runner gained a bounded stability mode rather than a
second CI script:

```powershell
./tests/ci/multihost-gate.ps1 -BuildMode Never \
  -StabilityOnly -StabilitySeconds 180 \
  -ReportDir tests/.artifacts/reports/stb-704-stability-final
```

The host command had a 330-second timeout. It completed in 228.3 seconds,
including startup and teardown. The steady-state step covered 183.9 seconds and
captured seven samples for each of seven nodes (49 rows).

| Metric | Measured | Enforced bound |
| --- | ---: | ---: |
| maximum node memory | 32.96 MiB | < 2% container-host memory per node |
| maximum node CPU sample | 3.84% | recorded; no runaway retry observed |
| maximum writable layer | 0.309 MiB | <= 128 MiB per node |
| writable-layer growth, first to final | 0 bytes on all seven nodes | bounded/no growth |
| readiness checks | all seven nodes at every sample | no loss/deadlock |
| remaining project containers after teardown | 0 | 0 |

Node memory rose during Go/Waku warmup and then remained in the bounded
approximately 26-33 MiB range. Writable layers did not grow at all. Host state
before the final run was 27.3 GiB available RAM and 197.65 GiB free disk; after
teardown it was 27.2 GiB available RAM, 197.65 GiB free disk, and 2.43 GiB in
`vmmemWSL`.

The first stability attempt failed after startup because the sampler counted the
internal workload engine in addition to the seven declared nodes. It performed
full teardown. The sampler now resolves the seven named node services
explicitly; the corrected run passed.

Evidence:

- `tests/.artifacts/reports/stb-704-stability-final/summary.json`
- `tests/.artifacts/reports/stb-704-stability-final/stability-samples.json`
- `tests/.artifacts/resources/stb704-stability-retry.json`
- `tests/.artifacts/resources/stb704-stability-after.json`

## Acceptance

- The thresholds were committed before measurement in commit `9720cea`.
- All test execution occurred in Linux containers; Windows only orchestrated
  Docker and captured host resources.
- Every runtime command had a hard external timeout and completed below it.
- No full integration suite was run for STB-704.
- The code-size guard passed for the new integration test after scenario steps
  were split into cohesive fixture methods.
- The new Go test changes no product API or domain ownership and uses the real
  Network Foundation / Messaging path.
- Performance thresholds fail runnable tests or the multi-host gate; they are
  not informal observations.
- No release-blocking performance or bounded-resource failure remains for the
  declared profiles.

This is a bounded safety baseline, not an Internet-scale capacity claim.
Multi-day leak, churn, and recovery evidence remains owned by STB-705.
