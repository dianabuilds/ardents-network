# STB-703 Bounded Chaos Campaign

Status: predeclared; execution pending.

## Execution Contract

- Run faults only in disposable Docker test environments.
- Give every runner a hard orchestration timeout and retain its canonical report.
- Record the fault, blast radius, expected operator truth, recovery target, and
  rollback before execution.
- Do not treat container survival as success: assert readiness, diagnostics,
  publication/availability truth, and pending-operation fate where applicable.
- Tear down every topology after the experiment and verify no campaign
  containers remain.
- Convert any missing or false signal into a canonical scenario/test before
  STB-703 completes.

## Predeclared Fault Matrix

| ID | Fault | Injection and blast radius | Expected truth / recovery target | Canonical detector |
| --- | --- | --- | --- | --- |
| CH-01 | Process crash | Send `SIGKILL` to multi-host node `a1`; only that disposable node loses its process. Recreate it through Compose. | No topology-wide false failure; `a1` rejoins and reports ready/participating within 60 seconds. | `NFM-001` crash-and-recover step. |
| CH-02 | Disk pressure | Fill the workload's bounded tmpfs beyond its 1 MiB limit; workload container only. Compose teardown removes it. | Command fails inside isolation; host/node remains responsive and no success/publication is reported. | `WKI-004` `TestDockerExecutorDeniesFilesystemNetworkProcessAndMemoryPressure/tmpfs`. |
| CH-03 | Corrupted state | Start a node from deliberately malformed persisted state in its temporary data directory. Directory is deleted by test cleanup. | Startup fails explicitly and diagnostics never report ready. | `NRI-001` `TestNodeFailsWhenStateLoadIsCorrupt`. |
| CH-04 | Bounded clock skew | Advance envelope evaluation time beyond the authenticated expiry window without changing wall clock. | Open fails with expiry truth; no plaintext/message acceptance. | `TestPrivateEnvelopeRejectsOuterVersionFlagsTimeAndSize`. |
| CH-05 | Packet loss | Disconnect isolated node `b1` from `zone_b`, producing 100% packet loss for that node until reconnect. | `b1` enters restricted defense and cannot claim healthy provider participation; reconnect returns it to steady within 75 seconds. | `NFM-001` isolated-node loss/rejoin steps. |
| CH-06 | Dependency latency | Delay the hosted-service probe response to 100 ms against a 10 ms deadline. | Readiness is `not_ready` with `probe_timeout`; publication eligibility remains false. | `TestControllerReportsBoundedProbeTimeoutAfterWarmup`. |
| CH-07 | Network partition | Disconnect the dual-homed bridge from `zone_b`; zone B remains internally connected but separated from zone A. | Local segment participation stays truthful; bridge reconnection restores the full topology. | `NFM-001` segment partition/rejoin steps. |
| CH-08 | Peer churn | Restart `b1` three times with readiness checks after every iteration. | Both zone-B peers return to joined state each time without unbounded retry or stale identity. | `NFM-001` churn step. |
| CH-09 | Bootstrap loss | Stop seed after the topology has alternative live peers. | Remaining nodes keep healthy real participation; no dependency on one bootstrap hub is hidden. | `NFM-001` bootstrap-loss step. |
| CH-10 | Certificate expiry | Validate an already expired CA-issued WSS certificate in an isolated temporary secret directory. | Transport configuration is rejected with an explicit expiry error before startup/publication. | `TestWSSConfigRejectsExpiredCertificate`. |
| CH-11 | Capability revocation | Revoke an imported sender capability after cross-node private use. | Subsequent private use is rejected and revocation truth is visible; no stale selector authorization. | `NPI-001` `TestPrivateCapabilitySelectorsInteroperateAndRevokeAcrossNodes`. |
| CH-12 | Quota exhaustion | Offer a replica to a peer whose configured retention quota cannot admit it. | Placement records `quota` denial and later operations reach explicit terminal loss rather than false availability. | `DAE-003` `TestAvailabilityFailureMatrixEndsInHonestTerminalLoss`. |
| CH-13 | Dependency restart | Restart real Waku peers during multi-host recovery/churn while other nodes remain live. | Dependants expose transient loss and recover to joined/steady without manual state repair. | `NFM-001` crash, rejoin, and churn observations. |

## Planned Runs

1. A bounded Docker Go-test batch for CH-03, CH-04, CH-06, and CH-10.
2. Canonical scenario `NPI-001` for CH-11.
3. Isolated workload Docker runner for CH-02 plus its OOM/PID/network controls.
4. Canonical scenario `DAE-003` for CH-12.
5. Canonical multi-host scenario `NFM-001` for CH-01, CH-05, CH-07,
   CH-08, CH-09, and CH-13.

## Result Ledger

| ID | Actual signal | Recovery | Result | Follow-up |
| --- | --- | --- | --- | --- |
| CH-01 | pending | pending | pending | pending |
| CH-02 | pending | pending | pending | pending |
| CH-03 | pending | pending | pending | pending |
| CH-04 | pending | pending | pending | pending |
| CH-05 | pending | pending | pending | pending |
| CH-06 | pending | pending | pending | pending |
| CH-07 | pending | pending | pending | pending |
| CH-08 | pending | pending | pending | pending |
| CH-09 | pending | pending | pending | pending |
| CH-10 | pending | pending | pending | pending |
| CH-11 | pending | pending | pending | pending |
| CH-12 | pending | pending | pending | pending |
| CH-13 | pending | pending | pending | pending |
