# R-092 native Node profile baseline

## Question

Can one disposable process exercise an exact `ardents-interactive-route-v1`
mTLS 1.3 leg, reciprocal `route.LegBinding`, bounded opaque-byte carriage,
withdrawal, cancellation, sampling, and joined cleanup strongly enough to serve
as a preliminary oracle for the later real Rendezvous duty?

## Hypothesis

One exact `ardents-interactive-route-v1`
mTLS 1.3 leg and reciprocal `route.LegBinding` exchange before echoing synthetic
opaque bytes can meet the declared identity, byte, withdrawal, cancellation,
sampling, and cleanup checks. Any listed falsifier rejects that baseline. Even
a pass selects no capacity or operating profile. It answers only the
preliminary protocol/function question in
[R-092](../../docs/research/records/r-092-native-node-operating-profile.md).
It is not a Node listener, capacity measurement, operating-profile selection,
or Route Qualification result.

## Run

Run from the repository root. The explicit file list keeps this disposable
executable out of the maintained module's normal build graph:

```sh
go run experiments/r-092-native-node-profile/main.go \
  experiments/r-092-native-node-profile/role_carriage.go \
  experiments/r-092-native-node-profile/linux_sampler.go \
  -scenario baseline -connections 1 -payload 65536

go run experiments/r-092-native-node-profile/main.go \
  experiments/r-092-native-node-profile/role_carriage.go \
  experiments/r-092-native-node-profile/linux_sampler.go \
  -scenario role-carriage -capacity 2 -payload 65536 -hold 10s \
  -sample-interval 1s -timeout 30s

go run experiments/r-092-native-node-profile/main.go \
  experiments/r-092-native-node-profile/role_carriage.go \
  experiments/r-092-native-node-profile/linux_sampler.go \
  -scenario role-cancellation -capacity 2 -payload 65536 -hold 10s \
  -sample-interval 1s -timeout 30s
```

For a decision-bearing reference-host campaign, retain the command transcript,
`go version`, `uname -srmo`, the binary SHA-256, and unmodified JSON stdout in
an external evidence directory. For example, on the declared Ubuntu host:

```sh
evidence_dir="$(mktemp -d)"
git rev-parse HEAD | tee "$evidence_dir/source-commit.txt"
go version | tee "$evidence_dir/go-version.txt"
uname -srmo | tee "$evidence_dir/uname.txt"
go build -trimpath -o "$evidence_dir/r092-role-carriage" \
  experiments/r-092-native-node-profile/main.go \
  experiments/r-092-native-node-profile/role_carriage.go \
  experiments/r-092-native-node-profile/linux_sampler.go
sha256sum "$evidence_dir/r092-role-carriage" | tee "$evidence_dir/binary.sha256"
"$evidence_dir/r092-role-carriage" -scenario role-carriage -capacity 2 \
  -payload 65536 -hold 10s -sample-interval 1s -timeout 30s \
  | tee "$evidence_dir/drain.json"
"$evidence_dir/r092-role-carriage" -scenario role-cancellation -capacity 2 \
  -payload 65536 -hold 10s -sample-interval 1s -timeout 30s \
  | tee "$evidence_dir/cancellation.json"
```

## Evidence and falsification

The program generates ephemeral synthetic Ed25519 certificates and loopback
addresses; it writes no state, credentials, captures, or generated artifacts to
the repository. Its JSON output records elapsed time plus Go allocation and
goroutine deltas. `role-carriage` also records runtime/OS/architecture/kernel/
CPU/RAM host identity, its own executable SHA-256, and raw per-interval Linux
process RSS, file descriptor, socket, and CPU-tick samples. The executable
digest in JSON must equal the external `sha256sum` record. Those still are not
the full-profile observations required for selection. A selection run must
execute on the R-092 Ubuntu reference host and retain raw OS CPU/RSS/FD/socket,
pressure, drain, withdrawal, and cleanup evidence outside Git.

`role-carriage` is a synthetic, bounded pressure injection: it carries exactly
`capacity` simultaneous reciprocal TLS/LegBinding legs, withdraws the synthetic
listener before a subsequent dial, holds the admitted legs, then drains and
joins every worker. `role-cancellation` follows the same admission/withdrawal
setup but cancels the carried legs and requires every client and server worker
to join within two seconds. Both sample Linux process RSS/CPU/FD/socket state
through the final post-cleanup observation. They do **not** demonstrate an
actual Node admission decision before a kernel accepts a TCP socket, real
resource pressure, a production listener, a capacity selection, or a product
claim.

## Result

Absent TLS 1.3/ALPN, an unverified peer key, a nonreciprocal
binding, a byte mismatch, a listener that remains reachable after withdrawal,
missing Linux samples on the selected host, non-zero post-cleanup sockets, or
any worker that fails to join is a failed baseline. A local two-leg, 4,096-byte
sanity run passed; no number emitted by this program authorizes a native Node
admission limit.

## Disposition and next campaign

The Product Owner selected Rendezvous as the first native duty on 2026-08-24.
This harness remains a disposable protocol/cleanup baseline; extending its echo
server until it resembles a Node would create a second implementation and is
rejected. A separate, explicitly bounded
[Rendezvous tracer](../r-092-rendezvous-tracer/README.md) now owns the disposable
three-reservation/pairing/pump feasibility evidence without changing this
baseline's claim. The decision-bearing follow-up starts only after the maintained
candidate owns real handshake, unmatched-leg, active-pair, queue, pressure, and
terminal reservations.

On the exact NET-01A host, sweep pair capacity from one and double until the
first safe refusal or falsifier. Retest the last passing and next higher points
against healthy full-duplex, stalled TLS, unmatched leg, slow reader,
backpressure, reset/half-close, churn, saturation, `PROTECT`, `DRAIN`, assignment
change/expiry, listener failure, `SIGTERM`, and abrupt-loss cells. Retain five
10-minute runs for every decision-bearing sustained cell and all failures. The
complete oracle and selection rule live in R-092; this README supplies no
capacity or resource threshold by itself.

A throwaway interactive reservation model was also run on 2026-08-24. It found
that handshake identity must remain unknown until authenticated LegBinding and
that `PROTECT`/full pair capacity must close other pre-admission work. The result
is promoted into R-092; its TUI and model were deleted rather than retained as a
simulator, test oracle, or second Rendezvous implementation.
