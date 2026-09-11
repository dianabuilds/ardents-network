# Installed text-worker escape matrix

Run `make text-worker-escape-check` only as root on the dedicated preinstalled
Ubuntu 24.04 x86-64 qualification host. This profile pins a separately built
`text_worker_escape` worker artifact and runs
`TestInstalledTextWorkerEscapeMatrix` through the same root-owned temporary
Endpoint driver and unit integrity checks as the lifecycle profile.

The artifact attempts IPv4 and IPv6 TCP, IPv4 UDP, DNS, a host file, host Unix
IPC, network-namespace creation and uid escalation before its normal inherited
attachment protocol. The host-side driver owns positive-control listeners and
sentinels; connection or packet observation is a failure. The installed worker
property verifier checks the effective systemd syscall filter, including the
selected `bpf` denial: a direct forbidden syscall may terminate the worker and
cannot provide a trustworthy in-process result.

Build the worker outside the repository with `go test -c -tags text_worker_escape
-trimpath -buildvcs=false -o OUTPUT ./internal/application/textdocument`, install
it root:root mode 0555 at `/usr/lib/ardents/text-worker-root/ardents-text`, update
only the root-owned canonical artifact manifest with its digest, and supply that
digest as `ARDENTS_TEXT_ESCAPE_WORKER_SHA256`. Build and pin the driver/unit as
documented by the sibling lifecycle profile. Retain inventories and all failed
and successful host logs outside Git. Passing this profile is installed P6/P7
evidence, not whole-host, general Application, Route or release qualification.
