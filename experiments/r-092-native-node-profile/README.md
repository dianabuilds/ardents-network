# R-092 native Node profile baseline

This disposable baseline exercises one exact `ardents-interactive-route-v1`
mTLS 1.3 leg and reciprocal `route.LegBinding` exchange before echoing synthetic
opaque bytes. It answers only the preliminary protocol/function question in
[R-092](../../docs/research/records/r-092-native-node-operating-profile.md).
It is not a Node listener, capacity measurement, operating-profile selection,
or Route Qualification result.

Run from the repository root:

```sh
go run experiments/r-092-native-node-profile/main.go -connections 1 -payload 65536
```

The program generates ephemeral synthetic Ed25519 certificates and loopback
addresses; it writes no state, credentials, captures, or generated artifacts to
the repository. Its JSON output records elapsed time plus Go allocation and
goroutine deltas. Those are diagnostic baseline observations only. A selection
run must execute on the R-092 Ubuntu reference host and retain raw OS CPU/RSS/
FD/socket, pressure, drain, withdrawal, and cleanup evidence outside Git.

Falsification: absent TLS 1.3/ALPN, an unverified peer key, a nonreciprocal
binding, a byte mismatch, or any worker that fails to join is a failed baseline.
A local two-leg, 4,096-byte sanity run passed; no number emitted by this program
authorizes a native Node admission limit.
