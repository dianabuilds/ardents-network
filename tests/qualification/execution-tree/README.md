# Installed Execution hostile tree

This component profile drives the genuine new Execution runtime, local
authority, Job, operation and fixed-worker cleanup. It runs as the original
non-root Endpoint MainPID under a separately pinned temporary qualification
unit. It grants no installed product startup, Service readiness or network
authority. Local owner authorization is an explicit fixture.

Build outside the repository with the selected Linux amd64 toolchain:

    go test -c -tags document_worker_hostile -trimpath -buildvcs=false -o HOSTILE ./internal/application/document/worker
    go test -c -tags document_worker_installed -trimpath -buildvcs=false -o RUNTIME ./internal/domain/execution/runtime

Use the admitted Ubuntu24.04/systemd255/cgroup-v2 environment and fixed units
from the sibling lifecycle profile. Install RUNTIME root:root mode0555 at
/usr/lib/ardents/qualification/execution-runtime.test. The independently
pinned temporary Endpoint unit must select exactly TestInstalledExecutionHostileTree,
with -test.v and -test.timeout=2m. It must initially be inactive and have no
drop-ins. This test binary is a qualification driver, not the product Endpoint.

Root separately pins HOSTILE at the canonical fixed Text executable path,
updates its exact digest in the root-installed artifact manifest and supplies
ARDENTS_TEXT_HOSTILE_WORKER_SHA256. The artifact uses the real Text handshake
and creates its TERM-ignoring child and grandchild before READY. No product
argument or Application message selects this artifact. Preserve and restore
the ordinary artifact on a dedicated qualification host.

Supply ARDENTS_TEXT_LIFECYCLE_SHA256 and ARDENTS_TEXT_LIFECYCLE_UNIT_SHA256,
then run make execution-tree-check as root. The runner independently checks
all pins, platform, original invocation and exact executed connection and
administration receipts. It requires terminal successful qualification status;
missing tests, missing descendants and no-tests exit zero fail.

On each local surface, an independent Publisher retains its original Worker,
Job and operation while the victim tree joins. The Administration case also
retains a separate Connection operation. Kernel cgroup event and proc
descriptors verify the original process lineage, distinct identities and
TERM-ignoring descendants. After victim retirement the original Publisher
returns its exact immutable document through the fixed Text worker using a
local request and terminal fixture. Closing the Publisher Session revokes
its provenance and joins the worker. The manager must collect the original
units after the sibling trees and their operations join.

A fresh invocation loses its original attachment. The test holds its
operation join while independently observing that the original parent and
cgroup retire after EOF. After release the Execution owner must report the
original peer-loss result, join cleanup and expose a collected worker unit.

Run against the ordinary worker as a negative environment control: absent
hostile descendants must fail. The descendant watchdog and test timeout never
prove join. Keep failures, exact source/artifact/unit pins, manager journal and
platform inventory outside Git. Containers/WSL are explicitly labelled
component evidence. The snapshot result covers the fixed worker with a local
stream fixture; authenticated Service, logical recovery and whole-host
qualification have their own profiles.
