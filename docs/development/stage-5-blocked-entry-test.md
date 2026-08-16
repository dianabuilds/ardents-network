# Stage 5 blocked-entry tracer

S5.3 keeps the accepted Stage 4 Route and publisher authentication unchanged
while replacing only the Client-to-Initiator entry channel with the pinned
WebTunnel carrier. It is a deterministic development tracer, not the final
R-037 campaign or a general censorship-resistance claim.

Run the Windows host driver with the two externally built, R-036-pinned Linux
binaries:

```powershell
.\scripts\test-stage5-webtunnel.ps1 `
  -ClientBinary C:\outside-repository\webtunnel-client `
  -ServerBinary C:\outside-repository\webtunnel-server
```

The PowerShell file is only a thin launcher. The directly runnable Go live test
verifies both binary hashes, builds the current product image, owns the full
Docker lifecycle, and gives Endpoint, Bridge, Initiator, Introduction,
Rendezvous, Responder, and Publisher separate processes and network namespaces
on two internal networks. Each product role receives a distinct owner-only
input tree; among maintained product roles candidate material exists only at
Endpoint and Bridge, and no Route, Service, or Application process receives
another role's private key. The isolated `fault-one` harness fixture also mounts
the pinned server solely to inject the declared C4 fault. The two
Application processes use only shared Unix IPC in `network_mode: none`; the
Stage 4 data socket remains a raw byte stream and a derived owner-only local
control socket optionally carries the sole bounded `ASRS` Result for the
maintained tracer. A Stage 4 peer that opens only the data socket remains valid
and receives the legacy trailing Result; Endpoint admission never requires the
optional connection. The maintained Application waits on both, so an early
classified failure interrupts an incomplete workload without being mistaken
for Application bytes. Every process uses a read-only
image, UID/GID `65532`, dropped capabilities, finite CPU/memory/PID/FD limits,
and owned temporary state. Candidate children enforce the same unprivileged
identity before executing, including when an owning process starts as root.
Only the isolated test observers run as root with their one declared
`NET_RAW` or `NET_ADMIN` capability. A separate observer runs in every
networked role namespace and fails on candidate TCP/UDP port-53 traffic,
ambiguous transport headers, or undeclared data paths. Explicit IPv6 link-local
multicast control is classified separately and cannot satisfy a required path.
For every cross-boundary episode they consume atomic path manifests, drain
pre-cell packet queues, count all IPv4/IPv6 TCP/UDP packets against exact
source/IP/port targets at all six network boundaries, require the exact
front/server loopback target inside the exclusive Bridge namespace, bind and
record the one expected ephemeral WebTunnel-client SOCKS target, and reject a
second or otherwise undeclared loopback, network, proxy, or fallback target.

The maintained short cells cover the frozen repetition floors. C0, C1, and C2
share the same product commands, four-position Route, authenticated Service
Target, Application byte streams, and exact ordinary Initiator at
`172.31.20.11:4601`:

- C0 in `20/20` cross-namespace episodes through the ordinary entry, with no
  Bridge or Adapter process started;
- C1/C2 in `20/20` cross-namespace production-command episodes by carrying the
  same exact Target through the pinned WebTunnel profile. A separate
  `NET_ADMIN` policy process installs a blackhole for that exact ordinary
  Initiator, the Endpoint proves the block for `3 s`, and only then consumes the
  authenticated Bridge transition. C2 additionally sends raw-carrier and PT
  control shapes to the same declared Bridge address and port and requires the
  TLS/HTTP boundary to reject both before allowing the WebTunnel profile; it
  also runs the uninformed/informed probe process in its own namespace. Both
  cells carry a fresh connection canary, the canonical
  nonce-bearing 512-byte HTTP request, the deterministic incompressible 64-KiB
  response, their exact end-to-end digests, and both explicit Connection
  Results;
- C3 in `5/5` product-command episodes: all four durable contacts hit the two
  policy-blackholed Bridge addresses, exhaust in fixed ordinal order, and never
  dial the ordinary Initiator endpoint;
- C4 in `5/5` product-command episodes. Slot zero closes the first connection
  and stalls the second before readiness; slot one uses the real pinned server
  and fails Route authentication after readiness by an `8 s` stall and then
  truncation. Verified cleanup precedes every next ordinal and the fourth
  result is durable exhaustion;
- the recovery-parent cell in `5/5` integrated episodes: a live Service
  Connection runs in separate `ardents-service`, `ardents-stream-app`, and
  `ardents-route` processes. The harness terminates the live ordinary
  Initiator after the Application's product-side last-byte event, the first
  real WebTunnel contact stalls to offset `8 s`, candidate cleanup finishes
  before `14 s`, the unchanged terminal episode and zero later Bridge ordinals
  are verified, and the Application receives `abrupt connection loss` before
  offset `15 s` with zero residue. The exact `15 s` episode retains a fixed
  `10 ms` proposal-publication reserve; both last-byte and result-publication
  timestamps come from the product processes rather than Docker-log arrival;
- C5 in `20/20` external-namespace four-request episodes with valid-TLS
  missing/wrong paths plus malformed TLS and HTTP probes; and
- C6 in `20/20` external-namespace disclosed-path episodes as an explicit
limitation: a probe that knows the numeric endpoint, TLS name, and secret
  path receives a WebTunnel-origin response and can distinguish the front.

Every C1-C4 and recovery transition, contact, cleanup, and terminal offset is
derived from that episode's single `CLOCK_MONOTONIC` manifest start. Wall-clock
timestamps remain diagnostic only.

Run the three maintained matrices directly when individual evidence is needed:

```powershell
go test -tags=live ./tests/live/network -run '^TestBlockedEntryCommandsAcrossNamespaces$' -count=1 -timeout=90m
go test -tags=live ./tests/live/network -run '^TestBlockedEntryNegativeCommandsAcrossNamespaces$' -count=1 -timeout=20m
go test -tags=live ./tests/live/network -run '^TestBlockedEntryRecoveryParentCommandsAcrossNamespaces$' -count=1 -timeout=15m
```

The host must supply the two R-036-pinned binaries through
`ARDENTS_WEBTUNNEL_CLIENT` and `ARDENTS_WEBTUNNEL_SERVER`. Their accepted
SHA-256 values are `de581c8dd36193bb4168aee840406294af406bf8187817c10ac2bcd9464fd120`
and `5fe32f8ab736ed54fc66027775761084e68f0e1ec9b5fea7c3417c6617255336`.

The tracer satisfies the R-037 short-cell repetition floors, but not impairment,
capacity, sustained-transfer, pressure, hostile-evidence, or
independent-verifier gates. Those remain S5.4/S5.5 work and must not be inferred
from a green S5.3 run.
