# STB-703 Bounded Chaos Campaign

Status: completed on 2026-07-20.

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
| CH-01 | `a1` received `SIGKILL`; the same persisted node rejoined. | Joined in 2.6 seconds. | robust | Canonical NFM step now injects a real crash. |
| CH-02 | Readonly, external network, tmpfs, PID, and OOM attempts failed inside the workload boundary; OOM required operator action. | DIND topology removed after the 11.6-second suite. | robust | None. |
| CH-03 | Corrupt state produced an explicit startup error and no ready state. | Temporary state removed by test cleanup. | robust | None. |
| CH-04 | Expired/future envelope time was rejected before message acceptance. | No persistent fault. | robust | None. |
| CH-05 | Isolated `b1` entered `restricted_defense`. | Rejoin restored steady provider shape within 40.5 seconds. | robust | None. |
| CH-06 | Slow probe produced `probe_timeout`/not-ready truth. | Immediate on the next healthy observation. | robust | None. |
| CH-07 | Zone-B partition preserved truthful local participation. | Bridge and peer rejoin restored cross-segment steady state. | robust | None. |
| CH-08 | Three consecutive real peer restarts completed with joined checks after each. | Churn block completed in 32.7 seconds. | robust | None. |
| CH-09 | Seed loss did not collapse the alternative live mesh. | Remaining nodes stayed joined; disposable seed remained stopped until teardown. | robust | None. |
| CH-10 | Expired CA-issued WSS material was rejected with `expired`. | No transport started; temporary secrets removed. | robust | None. |
| CH-11 | Cross-node use after capability revocation was rejected. | Scenario reached explicit revoked truth in 47.6 ms. | robust | None. |
| CH-12 | Quota denial participated in an honest terminal-loss outcome, not false availability. | E2E completed in 17.4 seconds. | robust | None. |
| CH-13 | Waku peer crash/restarts exposed transient unavailability during bounded waits and recovered without state repair. | Crash, rejoin, churn, and Store recovery all passed. | robust | None. |
