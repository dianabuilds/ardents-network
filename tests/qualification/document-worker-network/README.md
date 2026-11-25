# Installed document worker network exchange

Run `make text-worker-network-check` on the dedicated preinstalled Ubuntu 24.04
x86-64 qualification host with systemd 255, cgroup v2, polkit, the fixed
`ardents-endpoint` account and the [ordinary document worker inventory](../worker-lifecycle/README.md).
Build the exact candidate outside Git with the selected Go toolchain,
`GOOS=linux`, `GOARCH=amd64` and `CGO_ENABLED=0`.

    go test -c -tags document_worker_installed -trimpath -buildvcs=false -o "$candidate/execution-command.test" ./internal/endpoint
    go build -trimpath -buildvcs=false -o "$candidate/ardents-next" ./cmd/ardents-next
    go build -trimpath -buildvcs=false -o "$candidate/ardents-custody" ./cmd/ardents-custody
    go build -trimpath -buildvcs=false -o "$candidate/ardents-text" ./internal/application/document/worker

Install the first three independently pinned artifacts root-owned at
`/usr/lib/ardents/qualification` with mode 0555. Install the ordinary document
worker and its canonical manifest as specified by the lifecycle profile.
Preserve any prior qualification artifacts for restoration. Load the pinned
inactive `/run/systemd/system/ardents-endpoint.service` with no drop-ins and
`RuntimeMaxSec=960s`. Use the ordinary lifecycle unit properties and this
single entrypoint.

    /usr/lib/ardents/qualification/execution-command.test -test.run=^TestInstalledDocumentCommandsThroughJoinedNetworkTCP$ -test.v -test.timeout=15m

Provide the independent `ARDENTS_RECOVERY_INSTANCE_SHA256` and
`ARDENTS_RECOVERY_CUSTODY_SHA256` pins to the unit and the root runner.
Provide `ARDENTS_TEXT_LIFECYCLE_SHA256` and `ARDENTS_TEXT_LIFECYCLE_UNIT_SHA256`
to the root runner. It verifies the original invocation and requires terminal
success, no live MainPID and one PASS receipt each for the root, TCP/TLS and
all four cases. Begin with sufficient real time in the original token hour for
all exchanges and at least sixteen minutes remaining at the refresh case.

Each case independently obtains a Service Credential through the installed
Instance command and Custody with a no-echo PTY. The pinned ordinary CLI uses
the actual local Administration and Connection sockets with the genuine
successor Publisher and Reader. The Link travels through an owned pipe. The
profile compares the complete immutable document for empty, 64 KiB and 4 MiB,
then checks authenticated withdrawal and Link refusal. The fourth case compares
the full 64 KiB document after real elapsed refresh, a verified replacement
Descriptor and refusal of a still-signed predecessor.

The [Publication timing profile](../publication-timing/README.md) exercises
original creation and ACK timing brackets and actual pair overlap on the same
owners. The Instance package's recipient lifetime test directly verifies joined
key erasure. The installed document profile uses signed Network provisioning
fixtures and one manager. Retain source identity, failures, independent pins,
unit/manifest inventories and invocation logs outside Git.
