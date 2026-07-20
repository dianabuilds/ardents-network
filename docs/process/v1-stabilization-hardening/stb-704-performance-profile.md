# STB-704 Performance And Resource Safety Profile

## Purpose

This profile turns the first `v1` performance baseline into bounded release
criteria. It is a safety gate, not a capacity claim. The values below are
deliberately conservative and apply to the canonical Linux-container test
environment on a developer host with at least 4 CPU cores and 8 GiB available
memory.

## Declared Profiles And Thresholds

| ID | Profile | Workload | Release threshold |
| --- | --- | --- | --- |
| PERF-01 | Two local service nodes over real Waku TCP relay/store | Start both nodes, join one peer, deliver 16 encrypted 1 KiB-class messages, query Store, stop both nodes | each startup/readiness <= 15 s; 1-4 peer and relay connections; batch <= 5 s; p95 delivery <= 2 s; throughput >= 3 msg/s; Store query <= 5 s; each stop <= 5 s |
| PERF-02 | Hosted-service probe controller | normal readiness plus bounded slow dependency | normal probe remains within its scenario timeout; a 100 ms probe against a 10 ms deadline becomes explicit `probe_timeout`, never false readiness |
| PERF-03 | Chunked encrypted data transfer | canonical DAI-003 payload and Waku carrier | scenario <= 30 s; payload hash and decrypted length match; no silent truncation |
| PERF-04 | Availability observation and repair | canonical DAI-004 peer-loss repair | scenario <= 60 s; operation reaches repaired or an explicit terminal reason |
| PERF-05 | Docker workload resource enforcement | readonly root, network isolation, tmpfs, PID, memory/OOM, CPU quota | all nine canonical workload checks pass within 30 s test time; overload is rejected or terminated explicitly |
| PERF-06 | Seven-container multi-host topology | join, loss, partition, churn, Store recovery | gate <= 240 s; host free memory stays >= 4 GiB; host disk usage < 90%; no container remains after teardown |
| PERF-07 | Bounded stability sample | the PERF-06 topology for at least 180 s | no monotonically unbounded container-memory or disk growth; no deadlock/runaway retry; final health and teardown are observable |

## Measurement Rules

- All product tests execute in Linux containers. PowerShell on Windows only
  orchestrates Docker and captures host resource snapshots.
- Every command has an orchestration timeout. A timeout is a failed gate, not a
  reason to wait indefinitely.
- Functional assertions remain mandatory. Fast execution without the expected
  relay payload, Store record, repaired state, or enforced quota is failure.
- Timing limits are evaluated by the runnable scenario wherever possible. Host
  CPU, memory, and disk limits are evaluated from retained resource snapshots.
- A threshold breach blocks release or produces an explicit degraded diagnostic;
  it cannot be recorded as an informal observation and ignored.

## Scope Boundary

This baseline proves that the current representative profiles are bounded and
usable. It does not claim Internet-scale capacity, geographic latency, or final
hardware sizing. Multi-day leak and recovery evidence belongs to STB-705.
