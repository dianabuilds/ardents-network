# Installed text-worker network exchange

Run `make text-worker-network-check` only on the dedicated preinstalled Ubuntu
24.04 x86-64 qualification host. This uses the installed lifecycle profile's
root operator, separate ardents-endpoint account, systemd255, cgroupv2, polkit,
root-owned ordinary worker artifacts, canonical manifest and accepted sockets.
Never select an operational Endpoint or the hostile substitute worker.

Build outside Git using the selected Go toolchain, Linux amd64 and CGO_ENABLED=0:

    go build -trimpath -buildvcs=false -o WORKER ./cmd/ardents-text
    go test -c -tags text_worker_installed -trimpath -buildvcs=false -o ENDPOINT ./internal/endpoint

Preserve the prior worker, manifest, qualification binary and temporary unit
before installing the exact candidate. Independently record hashes; install
binaries root:root0555 and update the canonical manifest's ordinary worker digest.
The root:root0644 /run/systemd/system/ardents-endpoint.service uses User/Group
ardents-endpoint, Type=exec, RemainAfterExit=no, ExitType=main, Restart=no,
RestartMode=normal, RuntimeMaxSec=960s and this exact command:

    /usr/lib/ardents/qualification/endpoint.test -test.run=^TestInstalledTextWorkersReadTargetThroughJoinedNetwork$ -test.v -test.timeout=15m

Load that exact inactive unit without drop-ins. Supply the shared runner's
ARDENTS_TEXT_LIFECYCLE_SHA256 and ARDENTS_TEXT_LIFECYCLE_UNIT_SHA256 from the
independently pinned candidate. The runner verifies artifacts and environment,
starts one invocation, collects its exact journal, and requires terminal inactive
success with MainPID0. It separately requires the root test, both Carrier tests
and all eight nested cases exactly once: empty, reference64KiB, maximum4MiB and elapsed refresh over
TCP/TLS and QUIC. No-tests success, missing cases, failure or missing prerequisite
cannot pass. The fifteen-minute test bound and sixteen-minute systemd watchdog are
qualification limits; each exchange retains its own accepted runtime bounds.

Each case launches real installed confined Reader and Publisher workers through
Endpoint verification, INIT/readiness and scoped Grants; uses actual publication,
Introduction, JOIN, Service authentication and worker protocols; compares the
complete immutable body; and joins worker/context retirement. Snapshot publication
and withdrawal traverse the actual local Administration socket and separately
authorized retained owner. Reading uses the real AAI3 Connection owner, installed
launcher and local RESULT projection. Startup caller completion precedes the read.
Publication, Link presentation and reading invoke the manifest-pinned ordinary
`ardents-text publish`, `link` and `read` commands as separate trusted UI processes.
The Link passes only through owned pipes, never argv or diagnostics. The complete
plain-text command output must equal the fixture body, including empty and maximum
size cases. Each command has a finite deadline and joined process/pipe completion.
Withdrawal still uses the real local Administration client directly.
Accepted Network State,
and Endpoint setup remain explicit test fixtures. Service Authority and Instance
provisioning use actual installed `ardents-custody` and `ardents` commands. This is not ordinary command adoption, complete 300s/60s acceptance,
full hostile-host qualification, full recovery qualification or acceptance of
issues #56–#58. Retain source identity, all failed attempts, independent hashes,
unit/manifest inventories and invocation journal outside Git.

The refresh case retains an installed Publisher for the real 300-second schedule,
observes a fresh slot/key and higher published revision, then checks the recorded
predecessor acceptance deadline is no later than its signed expiry or 60 seconds
after the actual first verified ACK transition recorded by its publication owner. Joined retirement and key erasure must be observed within two
seconds after that deadline; this observation allowance grants no extra acceptance.
It then performs the same real local read and withdrawal. Neither clocks nor
scheduler fields are advanced. This adds an executable qualification requirement;
building the test binary alone is not evidence that the elapsed trial passed.

The installed network profile additionally requires root-owned mode0555
`/usr/lib/ardents/qualification/ardents` and
`/usr/lib/ardents/qualification/ardents-custody`, built from the same candidate.
Record their independent SHA-256 values in the exact temporary Endpoint unit as
`Environment=ARDENTS_TEXT_ENDPOINT_COMMAND_SHA256=...` and
`Environment=ARDENTS_TEXT_CUSTODY_COMMAND_SHA256=...`. The test verifies each
installed file against its pin before execution. Preserve and restore these files
along with the other candidate artifacts. Missing binaries or pins invalidate
this profile; they never select a fixture fallback.

For each case, Custody creates a distinct Service Authority using a no-echo PTY.
The Instance command creates its public request, whose file and independently
presented digest are checked before Custody issues the public response. The
Instance command accepts that response; only then does the network fixture open
the accepted non-exporting binding. No test-side signature or direct Accept
supplies the Service Credential in this installed profile. This additional
command requirement must be executed on the new candidate; older successful
profile receipts do not cover it.