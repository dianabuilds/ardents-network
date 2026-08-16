# Stage 5 WebTunnel maintained Linux test

This test is controlled H3 evidence, not a public deployment or a
censorship-resistance claim. It consumes the two externally prepared R-036
Linux `amd64` binaries and performs no download or build repair.

From the repository root on a host with Go `1.26.6`, Docker Engine, the pinned
Ubuntu image already present, and the two binaries outside Git, run:

```powershell
.\scripts\test-stage5-webtunnel.ps1 `
  -ClientBinary C:\owned-temp\webtunnel-client `
  -ServerBinary C:\owned-temp\webtunnel-server
```

The script rejects either binary hash mismatch, cross-compiles the maintained
package test into a unique system-temporary directory, and runs it as UID/GID
`65534` on one Docker-internal `203.0.113.0/24` network. The candidate container has a
read-only root, a `32 MiB` no-exec tmpfs, no capabilities, and
`no-new-privileges`. The container is additionally capped at `2` CPUs,
`256 MiB`, `64` PIDs, and `128` FDs; the Adapter itself enforces a
`1 MiB`/`32`-entry state tree and the tests inspect the stricter per-candidate
FD/socket/process limits. Candidate binaries and frozen testdata are mounted
read-only. A separate short-lived observer joins only that network namespace
with `CAP_NET_RAW`; the candidate never receives it. Three IPv4/IPv6 UDP/TCP
port-53 positive-control flows prove observation before the candidate phase.
The observer then fails the suite on any candidate-phase port-53 packet or
ambiguous IPv4 fragment/IPv6 extension chain. The suite covers strict envelopes and PT v1 control, sanitized
client environment, one numeric SOCKS5 grant/refusal, pinned client/server
useful work through the standard-library TLS/HTTP front, three shutdown rungs,
repeated cleanup, process residue, candidate-state removal, zero candidate DNS,
and the exact numeric dial target. The script
removes its network and owned temporary directory on success or failure.

Pressure, traffic/resource time series, and the full C0-C6/hostile campaign
remain S5.3-S5.5 gates; this command does not claim them.
