# Installed Execution Endpoint death and fresh start

This privileged component profile proves the new Execution local physical
lifetime after an Endpoint exit that cannot run Go cleanup. It grants no
Installation, Release, Service authority, logical Connection recovery or
whole-host qualification. Local grants and the hostile artifact are explicit
qualification fixtures, not an ordinary Application selection.

Build outside the repository with the selected Linux amd64 toolchain:

    go test -c -tags text_worker_hostile -trimpath -buildvcs=false -o HOSTILE ./internal/application/textdocument
    go test -c -tags text_worker_installed -trimpath -buildvcs=false -o RUNTIME ./internal/successor/execution/runtime

Use the admitted Ubuntu24.04/systemd255/cgroup-v2 manager and fixed activation
inventory from the lifecycle profile. Pin HOSTILE independently at the fixed
Text executable path and update the root-owned artifact manifest. It creates
the actual child/grandchild before READY; the descendants ignore TERM and
retain their inherited attachment. Preserve/restore the ordinary artifact on
a dedicated qualification host.

Install RUNTIME as root:root mode0555 at
/usr/lib/ardents/qualification/execution-runtime.test. The independently
pinned temporary /run/systemd/system/ardents-endpoint.service must run as the
fixed Endpoint user/group and select only
TestInstalledExecutionEndpointDeathHeldWorkers, with -test.v and
-test.timeout=2m. Preserve the selected Endpoint main-process semantics,
fixed socket dependencies, no restart, finite RuntimeMaxSec and whole-group
termination. It must initially be inactive with no drop-ins.

Supply ARDENTS_TEXT_LIFECYCLE_SHA256, ARDENTS_TEXT_LIFECYCLE_UNIT_SHA256 and
ARDENTS_TEXT_HOSTILE_WORKER_SHA256, then run make execution-recovery-check
as Root outside the Endpoint unit. The profile verifies artifact/unit pins
and runs TestInstalledExecutionEndpointDeath in the independently pinned
runtime binary. Missing exact initial/restart controller PASS receipts fail.
The held actor intentionally cannot return PASS; its SIGKILL is a required
failed terminal result, not successful program termination.

Both Connection and Administration retain genuine qualified operations and
independently observed three-process hostile trees. Root binds copied readiness
candidates against current manager tuples and original kernel/proc descriptors,
then sends SIGKILL through the original Endpoint pidfd. Root performs no worker
Stop or Go cleanup. The original Endpoint and both original worker scopes and
every pinned descendant must physically disappear through the verified
BindsTo/After lifetime. Root retains the exact Endpoint signal/9 result.

After that physical join, the controller explicitly starts a fresh Endpoint
under the same fixed qualification unit. It requires a different original
Endpoint invocation, fresh local generation, Job nonce commitments and worker
invocations. It repeats the actual SIGKILL/join observation on that fresh
opening. Copied facts are never runtime permission or proof of join. A failed
controller signals only its original pidfd, never a replacement PID/unit.

The ordinary Text artifact is a negative control: absent hostile descendants
must fail before the death observation. Neither watchdog expiry nor pathname
absence, MainPID0, killed-actor logs or test exit zero proves original join.
Retain source/artifact/unit pins, original failed terminal results, controller
receipts and all invalid environments outside Git. Containers/WSL provide
explicitly labelled component evidence; no Service bytes, network recovery,
installed product acceptance, power-loss or privacy claim follows.
