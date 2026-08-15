# R-036 Camouflage Adapter feasibility harness

This disposable harness answers [R-036](../../docs/research/records/r-036-h3-camouflage-adapter.md):
can the two pinned candidates satisfy the same internal PT control seam and
carry the same opaque useful-work unit without DNS, fallback, privilege, or
residual resources?

It is research code, not an Ardents package, public protocol, or maintained
runtime dependency. Candidate source, dependencies, binaries, certificates,
state, raw logs, and evidence stay in an owned system-temporary directory.

## Hypotheses and falsifiers

- Standalone WebTunnel `v0.0.6` can use a numeric dial address, separate TLS
  name, and certificate pin to carry the workload with zero DNS queries.
- Lyrebird obfs4 `0.8.1` can carry the same workload with its generated `cert`
  and `iat-mode=0` configuration.
- Either candidate is rejected on a source/supply mismatch, reachable advisory
  forbidden by R-036, malformed control acceptance, non-loopback listener,
  privilege or descendant escape, DNS query, byte mismatch, deadline miss, or
  cleanup residue. There is no candidate fallback.

The exact structural limits and full falsification list are frozen in R-036.

## Environment and inputs

The run uses the source commits, Ubuntu image digest, Go archive, and build
flags recorded in R-036. The container is attached alone to an internal Docker
network at documentation address `192.0.2.3`, runs as UID/GID `65534`, drops all
capabilities, sets `no-new-privileges`, and receives only read-only candidate
binaries. A separate root helper joins only its network namespace with exactly
`NET_RAW`; its Linux `AF_PACKET` socket counts IPv4/IPv6 TCP or UDP packets
whose source or destination port is `53`, including common IPv6 extension
headers. Non-initial fragments, ESP, and overlong extension chains invalidate
the cell as ambiguous instead of supporting a zero count. It is an observer,
not a resolver.

The workload contains fresh random `32-byte` client/server canaries, one
nonce-bearing `512-byte` request, and a deterministic incompressible `64 KiB`
response. Only hashes and measurements are publishable. Raw PT output,
candidate arguments, certificates, keys, and addresses are secret evidence.

## Exact run

`run.ps1` requires Docker and the three already verified Linux binaries in the
external paths passed to it. It builds the harness with the repository Go
toolchain, runs `stdin`, `SIGTERM`, and `SIGKILL` cells for each candidate in
separate containers, and writes evidence only under the external
`-EvidenceRoot`.

```powershell
./experiments/r-036-h3-camouflage-adapter/run.ps1 `
  -Lyrebird C:\owned-temp\out\lyrebird `
  -WebTunnelClient C:\owned-temp\out\webtunnel-client `
  -WebTunnelServer C:\owned-temp\out\webtunnel-server `
  -EvidenceRoot C:\owned-temp\evidence
```

Before invoking it, reproduce the online preparation, module verification,
vendoring, two clean `--network none` builds, vulnerability scan, hashes, and
license/SBOM capture specified in R-036. The script deliberately does not fetch
source or dependencies.

All experiment Go files are root-module build-ignored. Run the control parser
tests explicitly with:

```powershell
go test ./experiments/r-036-h3-camouflage-adapter/control.go `
  ./experiments/r-036-h3-camouflage-adapter/workload.go `
  ./experiments/r-036-h3-camouflage-adapter/control_test.go
```

Cross-compile and run the complete parser suite, including the Linux-only raw
packet observer tests, in the same pinned offline image with:

```powershell
$files = Get-ChildItem experiments/r-036-h3-camouflage-adapter -Filter *.go
$env:GOOS = 'linux'; $env:GOARCH = 'amd64'; $env:CGO_ENABLED = '0'
go test -c -trimpath -o $env:TEMP\r036-harness-tests $files.FullName
docker run --rm --network none --cap-drop ALL `
  -v "${env:TEMP}\r036-harness-tests:/tests:ro" `
  ubuntu@sha256:7b202b0e2e0028c6250f5fcf41d04df492d145a1654c6995a6553f0c1f6f1960 /tests
```

## Evidence and result

Expected output is six `summary.json` files and six secret evidence directories,
one for each candidate/shutdown-rung cell. A summary records useful-work hashes,
three resource/traffic samples, DNS packet count, process observations,
requested and actual shutdown rung, residual checks, and hashes of the raw
control transcript and secret run manifest. The raw evidence is not committed.

The decisive 2026-08-16 campaign returned `pass` in all six cells. Every cell
used the same exact request and response hashes, observed all three injected
control flows (IPv4 UDP, IPv6 UDP, and IPv4 TCP; `16–20` captured packets) and
zero candidate DNS packets, and restored the process, state, FD, PID-namespace,
and goroutine baseline within `306 ms` or less.
R-036 records the measurements and selects standalone WebTunnel `v0.0.6`. A
future supply-preflight rejection remains a rejection even if an isolated
functional probe carries bytes.

## Limitations and disposition

This is a single-host feasibility exercise, not an R-037 blocked-network
campaign, performance qualification, anonymity measurement, or censorship-
resistance claim. Docker's internal network and synthetic TLS front are
controlled fixtures. Retain this small harness as reproducible evidence;
delete all generated artifacts and secret evidence after the record has been
reviewed.
