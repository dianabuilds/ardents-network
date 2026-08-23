# R-092 native Node profile baseline

This disposable baseline exercises one exact `ardents-interactive-route-v1`
mTLS 1.3 leg and reciprocal `route.LegBinding` exchange before echoing synthetic
opaque bytes. It answers only the preliminary protocol/function question in
[R-092](../../docs/research/records/r-092-native-node-operating-profile.md).
It is not a Node listener, capacity measurement, operating-profile selection,
or Route Qualification result.

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

Falsification: absent TLS 1.3/ALPN, an unverified peer key, a nonreciprocal
binding, a byte mismatch, a listener that remains reachable after withdrawal,
missing Linux samples on the selected host, non-zero post-cleanup sockets, or
any worker that fails to join is a failed baseline. A local two-leg, 4,096-byte
sanity run passed; no number emitted by this program authorizes a native Node
admission limit.
