# Installed text-worker lifecycle

This explicitly selected profile runs TestInstalledTextWorkerLifecycle from a
Linux amd64 Endpoint test binary built with tag text_worker_installed. It uses
a fixture only for the local owner's authorization. Actual Endpoint code must
verify Ubuntu 24.04/systemd 255, the parent MainPID/account, installed artifact,
accepted socket and worker invocation before INIT/readiness and a scoped Grant.
Twelve activations for each role require fresh jobs and pinned cgroups that are
empty or removed before cleanup returns. Worker loss preserves its context;
context revoke prevents late launch and old completion. After that independent
kernel cleanup proof, each retired unit must disappear from the manager inventory
within three seconds. Effective collection policy is checked on the actual live
unit, including rejection of missing, malformed and nonselected observations.

This is installed lifecycle evidence, not the complete hostile-worker matrix,
normal protected Service journey, command adoption, or full #56 acceptance.
Never run it against an operational Endpoint. The operator must prepare a
dedicated clean Ubuntu 24.04 x86-64 VM/host, security-updated packages, cgroup v2,
active systemd 255/polkit, the separate ardents-endpoint account and all exact
candidate worker artifacts. Containers/WSL remain separately labelled evidence.

Build the test binary with the selected Go toolchain, outside the repository:
go test -c -tags text_worker_installed -trimpath -buildvcs=false -o OUTPUT ./internal/endpoint

Install it root:root mode 0555 at /usr/lib/ardents/qualification/endpoint.test.
Install a root:root mode 0644 temporary /run/systemd/system/ardents-endpoint.service
with User/Group=ardents-endpoint, Type=exec, RemainAfterExit=no, ExitType=main,
Restart=no, RestartMode=normal, RuntimeMaxSec=150s and ExecStart pointing to that
exact binary with -test.run=^TestInstalledTextWorkerLifecycle$, -test.v and
-test.timeout=2m. The service must be initially inactive, have no drop-ins, and
be loaded from that exact temporary path. The binary is a qualification driver,
not the protected product Endpoint command.

Supply independently recorded ARDENTS_TEXT_LIFECYCLE_SHA256 and
ARDENTS_TEXT_LIFECYCLE_UNIT_SHA256 to make text-worker-lifecycle-check as root.
The runner checks its exact binary/unit and platform before starting only that
declared unit, preserves the exact invocation journal, requires both role tests to have run
and passed, and waits for an inactive Endpoint with MainPID=0. No-tests success
and intermediate deactivation are refused.
Absent digests, installed prerequisites or unverified results are failures,
never passing skips. Retain failed attempts and complete source/dirty-tree,
artifact, OS/kernel/package, driver and unit inventories outside Git.

The shared runner also has one explicit tree profile, invoked only by
text-worker-tree-check with its independent hostile artifact digest and exact
TestInstalledTextWorkerHostileTree receipts. See the sibling profile README.
