# R-092 — Rendezvous reservation and data-plane tracer

## Question

Can the selected first native duty join exactly one authenticated Initiator leg
and one authenticated Responder leg under finite handshake, waiting-leg, and
active-pair reservations, carry a bounded bidirectional transcript, and join all
work on drain across the available local/remote Docker hosts?

## Hypothesis

A disposable Rendezvous server with separate finite handshake, waiting-leg,
and active-pair reservations can satisfy every predeclared local/race/two-host
oracle without duplicate pairing, pre-admission work at pair saturation, or
owned residue. Failure of any cell rejects this reservation/pairing/pump shape;
a pass still selects no numeric capacity or supported Node profile.

## Scope

This is disposable, build-ignored research code. It is not `ardents-node`, a
maintained package, a selected capacity, or a supported Node profile. It uses
the maintained canonical `route.LegBinding` encoding, TLS 1.3, the native ALPN,
and deterministic synthetic identities. It creates no Ardents Authority,
Network State, Entry Invite, Service Target, or persistent credential.

The server owns three independent finite reservations:

- accepted sockets doing TLS and binding authentication;
- authenticated single legs waiting for the opposite side; and
- active two-leg pairs whose only data path is two bounded `io.CopyBuffer`
  pumps with fixed connection deadlines.

An Attachment ID may contain at most one Initiator and one Responder leg. A
certificate identity must equal the binding sender identity; the binding must
name the exact network, epoch, digest, Rendezvous identity, native role, and
deadline. Pairing never follows a client-selected transport or fallback.

## Predeclared matrix and falsifiers

| Case | Required result | Falsifier |
|---|---|---|
| Exact pair | One Initiator and one Responder with one Attachment ID exchange the exact 256 KiB transcript in both directions. | Partial/duplicate bytes, wrong digest, missing reciprocal binding, more than one active pair, or leaked work. |
| Wrong/duplicate side | A second leg with the same side and Attachment ID is rejected without displacing the first. | Duplicate-side pairing or mutation of the retained first leg. |
| Unmatched timeout | One authenticated leg expires and releases its waiting reservation. | Reservation remains after the declared deadline or is paired after expiry. |
| Pair full | While the sole pair slot is active, new accepted sockets are closed before TLS/binding work and no new waiting leg appears. | New cryptographic worker/waiting state, eviction of the active pair, or hidden retry. |
| Drain | Listener closes first, waiting and active legs are closed, every pump/handler joins inside five seconds, and final reservations are zero. | New admission after drain, a live socket/goroutine, or nonzero final reservation. |
| Separate hosts | Two local client containers reach one resource-limited remote server container through its public high TCP port and satisfy the exact-pair oracle. | Host-network/shared-volume shortcut, port/workload collision, transcript difference, missed resource limit, or residue after cleanup. |

Any unexpected result is retained and classified; it is not retried away. The
separate-host run uses a fresh high port, no host networking, read-only scratch
images, an unprivileged UID, no capabilities, `no-new-privileges`, and explicit
CPU/memory/PID ceilings. It must not alter the existing remote containers or
ports 80/443.

## Claim boundary

A passing tracer supports the reservation/pairing/pump shape and the ability to
exercise it on two real hosts. It does not select numeric capacity, prove the
NET-01A 2-vCPU/2-GiB profile, independent operators, anonymity, censorship
resistance, availability, production durability, or complete Route behavior.
Those require the maintained duty and R-092's reference-host campaign.

## Run

Build by naming every source file explicitly; all Go files use
`//go:build ignore`. The checked local matrix builds outside the repository,
runs the five predeclared cases as separate processes, refuses path reuse, and
removes only its exact temporary files:

```powershell
pwsh -NoProfile -File experiments/r-092-rendezvous-tracer/run-local.ps1
```

When the host supports Go's race detector, repeat the same matrix with:

```powershell
pwsh -NoProfile -File experiments/r-092-rendezvous-tracer/run-local.ps1 -Race
```

If the Windows cgo linker cannot build the race runtime, run the focused
Linux exact-pair race cell inside a pinned/local Go image:

```sh
sh experiments/r-092-rendezvous-tracer/run-race-linux.sh
```

The separate-host slice uses `Dockerfile.wan` to package the exact Linux binary
in a scratch image. Record the binary/image digests and checked server/client
JSON in R-092; do not retain generated binaries, image archives, credentials,
containers, or captures in Git.

## Captured local evidence — 2026-08-24

The first complete Windows process matrix passed without a retry:

- `exact-pair`: one successful pair, exact 262,144-byte transcript and SHA-256
  `743cdf7849dc6ffdc775371adb60313afb6ecb74e2a8ef22f63c8d419e0b36ec`
  in both directions, followed by zero reservations/connections;
- `duplicate-side`: one duplicate rejected while the first leg remained and
  then expired exactly once;
- `unmatched-timeout`: one waiting leg expired and released its reservation;
- `pair-full`: two new connections rejected before TLS while the existing pair
  completed exactly; and
- `drain`: one active held pair was terminated, all handlers/pumps joined, and
  final handshake, waiting, pair, and connection counts were zero.

This is process/concurrency evidence on the development host, not a Windows
Node profile or a capacity measurement.

The Windows `-Race` build did not produce a tracer binary: the current cgo
linker closure requested `-ldl`, which the installed MinGW linker could not
resolve. This is a pre-execution toolchain failure, not a passing or failing
Rendezvous case. The focused exact-pair cell was therefore rebuilt with Go
1.26.6's Linux race runtime inside the already-local `golang:1.26.6` image. It
passed with one exact pair, no race report, joined cleanup, and zero final
connections.

## Captured separate-host evidence — 2026-08-24

The Linux/amd64 tracer SHA-256 was
`13343c6562cbf1b33dd6eef811dacd980193f186413dc628dced085c3f9e9f19`;
its scratch image ID was
`sha256:710126c98253b554a9fc71a7867af39366561a10603f0a41b23f075fabdbf3ef`.
One server ran on the project-operated Ubuntu 22.04.5 LTS Docker host and two
clients ran in separate Docker networks on Docker Desktop. All three containers
were read-only, UID 65532, capability-free, `no-new-privileges`, and capped at
128 MiB, 0.5 CPU, and 64 PIDs. They shared no volume or host network.

Both clients authenticated the exact server and reciprocal LegBinding, and
each reported the exact 262,144-byte digest, FD `7→7`, goroutines `3→3`, and
joined cleanup. Initiator elapsed time was 861 ms and Responder 567 ms. The
server observed peak handshakes `2`, waiting legs `1`, active pairs `1`, one
successful pair, and final handshake/waiting/pair/connection counts all zero;
it exited zero without OOM. These two single-client elapsed values are not a
latency or performance comparison.

After capture, the checked server/client containers, two client networks,
remote listener, scratch images, transfer archive, and generated binaries were
removed. Existing remote workloads and ports 80/443 were unchanged.

## Disposition

Retain only while it supplies unique R-092 evidence. Delete it after the
maintained Rendezvous duty and its source-bound qualification suite supersede
the tracer.
