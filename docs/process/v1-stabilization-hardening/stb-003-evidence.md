# STB-003 Evidence — Workload Exit Observation

## Root Cause

The product process observer was not the failing component. Two test-only
Windows probes depended on management commands that are unavailable under the
supported managed execution token:

- `tests/testkit.ProcessRunning` invoked `tasklist`, which returned
  `ERROR: Access denied` even for the caller's own PID;
- the E2E marker lookup invoked `Get-CimInstance Win32_Process`, which failed
  with the same access boundary.

The runtime already uses a native Windows probe based on `OpenProcess` and
`WaitForSingleObject`.

## Resolution

- the common testkit process assertion now delegates to the production native
  process probe instead of duplicating it with `tasklist`;
- the E2E unexpected-exit fault injection obtains the exact workload PID
  through an integration-only runtime hook and kills that PID directly;
- all post-fault assertions still cross public workload, discovery, and
  diagnostics APIs;
- executor regression coverage now proves both normal exit code `0` and
  non-zero exit code `7`; existing E2E scenarios prove forced termination and
  graceful restart recovery.

## Red/Green Evidence

- before repair, focused integration reproduction failed 3/3 times with
  `tasklist ... Access denied`;
- after repair, the same integration scenario passed 5/5 times;
- after repair, the E2E observed-exit scenario passed 3/3 times;
- natural/non-zero executor exit tests passed 10/10 times;
- full tagged integration layer: 8/8 packages pass;
- full tagged E2E layer: 7/7 packages pass;
- full fast layer: pass.

The original broken external-command probes are the deliberate negative
control: both fail deterministically in this environment while the native probe
and exact-PID fault injection pass.

## Size Guard

Checked:

- `internal/workload/execution/process_exit_test.go`;
- `internal/workload/execution/process_executor_test.go`;
- `tests/e2e/workload/lifecycle_test.go`;
- `tests/testkit/process.go`.

No file or function soft/hard limit breach was reported. Ownership remains in
Workload Control execution and its test boundary.
