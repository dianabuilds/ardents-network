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
```

The program generates ephemeral synthetic Ed25519 certificates and loopback
addresses; it writes no state, credentials, captures, or generated artifacts to
the repository. Its JSON output records elapsed time plus Go allocation and
goroutine deltas. On Linux it also records raw before/after process RSS, file
descriptor, and CPU-tick snapshots; those are not the per-second full-profile
observations required for selection. A selection run must execute on the R-092
Ubuntu reference host and retain raw OS CPU/RSS/FD/socket, pressure, drain,
withdrawal, and cleanup evidence outside Git.

`role-carriage` is a synthetic, bounded pressure injection: it carries exactly
`capacity` simultaneous reciprocal TLS/LegBinding legs, withdraws the synthetic
listener before a subsequent dial, holds the admitted legs, then drains and
joins every worker. It samples Linux process RSS/CPU/FD/socket state at the
requested interval. It does **not** demonstrate an actual Node admission
decision before a kernel accepts a TCP socket, real resource pressure, a
production listener, a capacity selection, or a product claim.

Falsification: absent TLS 1.3/ALPN, an unverified peer key, a nonreciprocal
binding, a byte mismatch, a listener that remains reachable after withdrawal,
missing Linux samples on the selected host, or any worker that fails to join is
a failed baseline. A local two-leg, 4,096-byte sanity run passed; no number
emitted by this program authorizes a native Node admission limit.
