# H4-2 local Docker full-system emulator

## Question

Does the complete current C-2 system classify a held Service Connection after
the test abruptly removes its product Rendezvous fault domain, and do both
maintained Carrier Profiles preserve the same authenticated C-2 contract
without hidden fallback on the constrained Linux profile?

## Run from Windows Docker

```powershell
make qualification-h4-2-local-emulator
```

The runner cross-builds the current test, product, and fixture bytes into one
temporary directory outside the repository. It mounts only those bytes
read-only in one disposable `golang:1.26.6` Linux container with no external
network, 1 vCPU, 1 GiB memory, and 256 PIDs. The campaign starts the actual current fixture roles and four
`ardents-node` commands as separate processes. It waits for the Publisher
Application and User Endpoint acknowledgements that the same exact C-2 route
is established, then hard-stops the Rendezvous process and requires the
existing route to finish with classified terminal results. It then runs the
same complete in-process C-2 journey over TCP/TLS and QUIC, verifies signed
State projection and rejection of Carrier Profiles, and checks that a failed
QUIC attempt never contacts an available TCP listener and that a failed TCP
attempt never sends a packet to an available QUIC/UDP socket. It also verifies
that the QUIC listener returns pending work before authentication completes so
Node owns the finite admission reservation. The Carrier cells also require a
healthy QUIC lane to survive more than its negotiated idle timeout, classify
unknown/timeout/unauthorized failures without exposing a transport retry, and
use a nonzero abort for failed post-open binding authentication. Temporary test state
stays under the container's `/tmp`; Docker removes the container and the runner
removes its outside-repository artifact directory after the test exits.

## Scope

This is the selected local full-system emulator. It proves the exact C-2
composition and simulated fault semantics of the current source on Linux. It
does not prove an independent machine, physical host or provider outage,
public-path failure/recovery, capacity, or availability. The checked run on
2026-08-27 passed every cell and repeated quic-go's 208-KiB to 416-KiB UDP
receive-buffer warning versus 7 MiB requested; that warning is retained as a
reason not to infer throughput or a supported capacity profile.
