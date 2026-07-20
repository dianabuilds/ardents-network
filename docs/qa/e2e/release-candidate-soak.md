# Release Candidate Multi-Day Soak

## Scenario ID

`SOAK-001`

## Layer

`e2e`

## Domain

Repository release process across Network Foundation, Data Substrate, Workload
Control, Hosted Services, Network Privacy, and Diagnostics.

## Category

Multi-day release-candidate stability, recovery, and resource-trend gate.

## Goal

Prove that one immutable release candidate repeatedly forms a real Waku network,
survives declared process and network faults, executes workload/service and
encrypted data lifecycles, excludes plaintext from the external carrier, and
remains bounded for 48 hours.

## Preconditions

- the STB-702 mutation campaign and STB-704 performance/resource baseline pass;
- the candidate Git commit and testnet image exist and are frozen;
- Docker has at least 4 GiB available host memory and host disk usage is below
  90%;
- no previous soak-owned container is running;
- all invoked scenarios use the canonical Linux-container runners.

## Steps

1. Record candidate commit, image ID, tool versions, start time, and deadline.
2. At each hourly checkpoint, verify candidate identity and host resources.
3. Run a 50-minute seven-node stability window at ordinary checkpoints.
4. Every six checkpoints run the full NFM-001 fault/recovery cycle.
5. At the same full checkpoints run DAI-003, DAI-004, WKE-001,
   E2E-NPI-001, and the Docker workload resource/security suite.
6. Retain all checkpoint artifacts and append the outcome to soak state.
7. At 48 hours verify the complete schedule, trends, final health, and teardown.

## Expected Result

All 48 checkpoints pass against the same candidate. Every injected degraded
interval is explained and recovers, every data/workload operation has a final
outcome, resource bounds hold, carrier captures contain no protected plaintext,
and teardown leaves no soak-owned resources.

## Failure/Degraded Variant

Any failed command, missed deadline, candidate drift, unexplained degraded
state, resource breach, plaintext finding, or teardown leak terminates the run
and records the exact checkpoint and retained artifacts. A failed soak must be
restarted from zero after remediation.

## Related Tests

- `tests/ci/soak-gate.ps1`
- `tests/ci/multihost-gate.ps1` (`NFM-001`)
- `tests/integration/data-substrate/chunked_transfer_test.go` (`DAI-003`)
- `tests/integration/data-substrate/availability_repair_test.go` (`DAI-004`)
- `tests/e2e/workload/lifecycle_test.go` (`WKE-001`)
- `tests/e2e/network-privacy/carrier_capture_test.go` (`E2E-NPI-001`)
- `tests/run-workload-docker.ps1`

## False Positive Risk

A loop that only checks process liveness can hide broken network, data,
publication, or privacy behavior. Full checkpoints therefore rerun the actual
cross-domain scenarios and retain their canonical reports.

## False Negative Risk

Transient host pressure unrelated to Ardents can fail a checkpoint. Host
resource snapshots, child logs, and bounded thresholds distinguish environmental
pressure from a product failure; the result remains failed until explicitly
classified and rerun.

## Notes

The exact duration, cadence, and success rules are fixed in
`docs/process/v1-stabilization-hardening/stb-705-soak-plan.md` before execution.
