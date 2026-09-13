# Installed text-worker recovery smoke

Run `make text-worker-recovery-check` only on the dedicated preinstalled Ubuntu
24.04 x86-64 qualification host. This is a bounded P7 process smoke profile,
not the complete NET-14 recovery, percentile, impairment, traffic-accounting or
P10 restart matrix.

The profile uses the installed lifecycle driver, separate `ardents-endpoint`
account, systemd 255, cgroup v2, polkit, root-owned ordinary worker artifacts,
the canonical manifest and accepted sockets.

Build outside Git using Go 1.26.8, Linux amd64 and `CGO_ENABLED=0`:

    go build -trimpath -buildvcs=false -o WORKER ./cmd/ardents-text
    go build -trimpath -buildvcs=false -o ENDPOINT_COMMAND ./cmd/ardents
    go build -trimpath -buildvcs=false -o CUSTODY_COMMAND ./cmd/ardents-custody
    go test -c -tags text_worker_installed -trimpath -buildvcs=false -o ENDPOINT ./internal/endpoint

Preserve the prior worker, manifest, commands, qualification binary and
temporary unit before installing the exact candidate. Independently record all
hashes. Install the candidate binaries root:root mode 0555 and update the
canonical manifest's ordinary worker digest. The root:root mode 0644
`/run/systemd/system/ardents-endpoint.service` uses `User=ardents-endpoint`,
`Group=ardents-endpoint`, `Type=exec`, `RemainAfterExit=no`, `ExitType=main`,
`Restart=no`, `RestartMode=normal`, `RuntimeMaxSec=360s` and this exact command:

    /usr/lib/ardents/qualification/endpoint.test -test.run=^TestInstalledTextWorkersRecoverAcceptedRequestAcrossJoinedNetwork$ -test.v -test.timeout=5m

The unit supplies the independently recorded command digests through
`ARDENTS_TEXT_ENDPOINT_COMMAND_SHA256` and
`ARDENTS_TEXT_CUSTODY_COMMAND_SHA256`. Supply the runner's
`ARDENTS_TEXT_LIFECYCLE_SHA256` and `ARDENTS_TEXT_LIFECYCLE_UNIT_SHA256` from
the independently pinned candidate. The shared runner verifies the artifacts,
environment, inactive unit and exact invocation; requires terminal inactive
success with `MainPID=0`; and requires the root test plus the TCP/TLS and QUIC
subtests exactly once. A missing case, prerequisite, pin or test is a failure,
not a skip.

Each Carrier case creates and accepts a Service Instance through the installed
`ardents` and `ardents-custody` commands, launches real confined Reader and
Publisher workers, and opens the maintained publication, Introduction, JOIN,
Service TLS and native Connection path. The Publisher Endpoint interrupts the
initial Route only after its installed worker has supplied the complete
512-byte document request. The same installed worker receives the exact
65,523-byte body through one matching fresh protected Route; the test rejects a
duplicate request, reused capsule digest, extra successful replacement, token
reuse, retained Introduction exchange, live Grant or populated/retained worker
cgroup. A callback already racing completion may enter once more only if it
returns cancellation without another successful digest or token spend. The
reported elapsed time covers interruption through validated result,
terminal control, worker protocol completion and joined worker cleanup and must
not exceed the five-second single-episode smoke bound.

Retain the source identity, dirty-tree digest, full artifact and unit inventory,
all failed attempts, complete invocation journal and host facts outside Git.
The logged useful bytes and derived useful bitrate are diagnostic only. This
profile has one episode per Carrier and no controlled link manifest or
authoritative directional network counters, so it cannot establish NET-14 p95,
the NET-14V byte/bitrate bounds, full Route Qualification, P10 restart safety or
issue #59 acceptance by itself.
