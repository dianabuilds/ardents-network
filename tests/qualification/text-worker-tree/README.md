# Installed hostile text-worker tree

This selected profile replaces only the dedicated qualification host's worker
executable with a separately pinned adversarial Go test artifact. Root records
and preserves the ordinary worker and artifact manifest before replacement and
restores them after evidence capture. It uses the same socket/service templates,
immutable root, DynamicUser, namespace/seccomp/resource properties and Endpoint
launch, INIT/readiness, opaque Grant and cgroup cleanup code.

Build outside the repository using the declared Go toolchain and Linux amd64:

    go test -c -tags text_worker_hostile -trimpath -buildvcs=false -o HOSTILE ./internal/application/textdocument
    go test -c -tags text_worker_installed -trimpath -buildvcs=false -o ENDPOINT ./internal/endpoint

The hostile artifact's parent accepts only the existing worker role arguments.
It audits the actual inherited descriptors, creates a child and grandchild
using ordinary Go self-exec, then runs the real worker exchange. Descendants
retain the accepted stream and ignore SIGTERM. Their local readiness pipe is
created after the inherited-authority audit. They deliberately refuse ordinary
parent cleanup; the installed Endpoint must terminate the whole cgroup. A
one-minute descendant watchdog limits a broken test and is never the passing
cleanup oracle. Descendant modes do not exist in the production command.

Use the dedicated Ubuntu prerequisites and independent Endpoint binary/unit
pins from the sibling installed lifecycle profile. The temporary test unit
selects exactly TestInstalledTextWorkerHostileTree with -test.v and a two-minute
binary deadline. Additionally supply ARDENTS_TEXT_HOSTILE_WORKER_SHA256 for the
root-owned executable at /usr/lib/ardents/text-worker-root/ardents-text, and
update only its digest in the root-installed canonical artifact manifest.
Run make text-worker-tree-check as the qualification operator. No client or
Application can select this artifact or enable a qualification runtime switch.

Each role test launches a victim tree and an independent Publisher sibling.
Kernel observations require the exact three-process lineage, matching worker
UID, inherited NoNewPrivs/seccomp/empty capabilities and live TERM-ignoring
children. After victim Close, the original pinned cgroup must be empty/removed
and no original live process may remain; PID start times distinguish reuse.
The sibling must retain its exact invocation and Grant and return its immutable
snapshot through the real local worker protocol. Its owner is then revoked and
its complete tree must also be joined and collected. A fresh replacement job
then loses its attachment: the real parent exits on EOF while descendants
retain it. The test observes original-parent exit and manager-owned cgroup
cleanup before calling Endpoint Close to revoke and finish that job.

The fixed document request is an explicitly local stream fixture. This result
covers hostile-tree cleanup and sibling survival, not authenticated Route
admission, hostile sibling escape, every abrupt-crash fault, network/syscall
escape or full P6/P7/#56 acceptance. Run the profile against the ordinary worker
as a negative environment control: absence of actual hostile descendants must
fail. Preserve failures, exact source/artifact hashes, manager journal and all
platform/package inventories outside Git. A profile PASS requires both role
receipts and terminal successful Endpoint status; missing prerequisites or
no-tests exit zero never pass.
