# Installed text-worker lifecycle

`make text-worker-lifecycle-check` selects `TestInstalledExecutionRepeatedLaunch`
from the current Execution runtime on a dedicated Ubuntu 24.04 x86-64
qualification host with systemd 255, cgroup v2 and polkit. The fixed
Endpoint account, Text artifacts and activation units must be installed.
Local grant declarations are fixtures.

Build `execution-runtime.test` outside the repository using the selected Go
toolchain:

    go test -c -tags text_worker_installed -trimpath -buildvcs=false -o OUTPUT ./internal/successor/execution/runtime

Install it at `/usr/lib/ardents/qualification/execution-runtime.test`, owned
by root with mode 0555. The root-owned temporary
`/run/systemd/system/ardents-endpoint.service` selects only
`-test.run=^TestInstalledExecutionRepeatedLaunch$`, `-test.v` and `-test.timeout=2m`,
`RuntimeMaxSec=150s`, the fixed Endpoint user and group, `Type=exec`,
`RemainAfterExit=no`, `ExitType=main`, `Restart=no` and `RestartMode=normal`.
The service must begin inactive with no drop-ins. Supply independently
recorded `ARDENTS_TEXT_LIFECYCLE_SHA256` and
`ARDENTS_TEXT_LIFECYCLE_UNIT_SHA256` to the root driver.

Both local surfaces perform twelve actual qualified launches on one original
Session per surface. Each launch uses a distinct Job nonce and systemd
invocation. The test observes the original cgroup and process before and
after joined cleanup. The manager must collect each original worker unit
within three seconds after the join. Replacement invalidates the previous
Job's completion provenance, and Session retirement refuses another Job.
The driver requires exact PASS receipts for all twenty-four launches and the
terminal successful Endpoint status. Retain failed attempts and exact
candidate, platform and manager receipts outside Git.

The shared runner also has one explicit stream-activation case (#654 3c-A),
invoked as `sh run-ubuntu.sh stream-activation` with the same independent
ARDENTS_TEXT_LIFECYCLE_SHA256 and ARDENTS_TEXT_LIFECYCLE_UNIT_SHA256 digests.
It runs TestInstalledStreamWorkerActivation from the successor worker package
test binary: build it outside the repository with
go test -c -tags text_worker_installed -trimpath -buildvcs=false -o OUTPUT ./internal/successor/execution/worker
and install it root:root mode 0555 at
/usr/lib/ardents/qualification/worker-stream.test. The temporary unit has the
same shape and ExecStart flags with
-test.run=^TestInstalledStreamWorkerActivation$. Before running, install the
fixed stream qualification inventory as root from a prepared static
CGO_ENABLED=0 linux/amd64 cmd/ardents-stream-qualification binary:
python3 packaging/stream-qualification-worker/install.py PREPARED_BINARY
(the separate ardents-endpoint account must already exist). The test proves
the trusted ActivateStream confinement observation against the real system
manager, the text inventory's refusal of the qualification unit,
ObserveInstanceStream identity re-observation, the fixed stream INIT/READY
exchange over the successor attachment, and joined original-cgroup cleanup for
both roles. It is worker-boundary component evidence, not the NET-14 workload
measurement (3c-C) and not a whole-host privacy qualification.

The shared runner also has one explicit stream-qualification case (#654 3c-B1),
invoked as `sh run-ubuntu.sh stream-qualification` with the same independent
digests and the same installed stream qualification inventory prerequisite. It
runs TestInstalledStreamQualificationSeam from the successor execution/runtime
package test binary: build it outside the repository with
go test -c -tags text_worker_installed -trimpath -buildvcs=false -o OUTPUT ./internal/successor/execution/runtime
and install it root:root mode 0555 at
/usr/lib/ardents/qualification/execution-runtime.test. The temporary unit has the
same shape and ExecStart flags with
-test.run=^TestInstalledStreamQualificationSeam$. The test proves the trusted
LaunchStreamQualification seam (stream worker activation, fixed INIT/READY,
installed reobservation), the text inventory's refusal of the qualification
unit, the one-use RunStreamConnections handoff guards, the bridge entry over the
retained attachment (fast-fail on an incomplete stream set), and joined
original-cgroup cleanup retaining the first failure, for both roles. It is
runtime-seam component evidence; the positive 64/256-stream NET-14 workload
through authenticated Service streams (3c-B2/3c-C) and whole-host privacy
qualification are not established here.

The [Execution tree profile](../execution-tree/README.md) selects
`TestInstalledExecutionHostileTree` through this same driver.
