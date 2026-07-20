# STB-705 Release Candidate Soak Plan

## Candidate Freeze

The soak is valid only for one immutable Git commit and one immutable
`ardents/ardd-testnet:dev` image ID. The driver records both before the first
checkpoint and fails immediately if either changes. Any release-blocking code
change invalidates the run and requires a new 48-hour soak from checkpoint zero.

## Duration And Cadence

- wall-clock duration: 48 hours;
- checkpoint interval: 60 minutes;
- expected checkpoints: 48 plus the final summary;
- ordinary checkpoint: a 50-minute seven-node stability window;
- full checkpoint: every sixth checkpoint, including checkpoint zero;
- expected full checkpoints: 8;
- the driver sleeps only for the remainder of the checkpoint interval and never
  overlaps test commands.

## Full Checkpoint Workload

Each full checkpoint executes, in order:

1. the complete NFM-001 multi-host cycle with process kill/recovery, address
   recreation, bootstrap loss, partition, restricted-defense transition,
   rejoin, peer churn, and retained Waku Store recovery;
2. DAI-003 encrypted chunked transfer/fetch/resume;
3. DAI-004 corrupt-replica observation and repair to a different Waku peer;
4. WKE-001 workload and hosted-service lifecycle;
5. E2E-NPI-001 external carrier capture and plaintext exclusion;
6. the Docker workload resource/security suite covering readonly root,
   isolated networking, tmpfs, PID, CPU, memory, and OOM outcomes.

## Evidence Per Checkpoint

- candidate Git commit and image ID;
- start/end timestamps and command exit status;
- host CPU, available memory, `vmmemWSL`, and disk snapshots;
- multi-host versions, topology, node snapshots, Docker stats, and logs;
- canonical JSON/JUnit reports for selected integration/e2e scenarios;
- Docker workload compose log;
- checkpoint result JSON and append-only soak event log.

## Fail-Fast Rules

The soak stops and performs teardown when any of the following occurs:

- candidate commit or image ID changes;
- a child gate returns non-zero;
- a checkpoint misses its bounded completion deadline;
- a node loses readiness outside an injected-fault interval;
- an injected partition does not recover;
- transfer/repair/workload lifecycle has no success or explicit terminal fate;
- carrier capture exposes protected plaintext;
- node memory or writable-layer bounds from STB-704 are breached;
- free host memory falls below 4 GiB or disk usage reaches 90%;
- a project container remains after a child gate's teardown.

## Success Gate

STB-705 passes only when the full 48-hour deadline is reached, every scheduled
checkpoint is present and passed, the final candidate identity matches the
initial identity, no unexplained degraded interval or pending operation remains,
and the final teardown leaves no soak-owned container.

This plan intentionally reuses canonical scenario runners. It does not create a
second test model or reinterpret an arbitrary loop as soak evidence.
