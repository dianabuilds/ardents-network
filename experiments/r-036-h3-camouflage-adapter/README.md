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
binaries. A local DNS trap is installed through Docker's resolver setting.

The workload contains fresh random `32-byte` client/server canaries, one
nonce-bearing `512-byte` request, and a deterministic incompressible `64 KiB`
response. Only hashes and measurements are publishable. Raw PT output,
candidate arguments, certificates, keys, and addresses are secret evidence.

## Exact run

`run.ps1` requires Docker and the three already verified Linux binaries in the
external paths passed to it. It builds the harness with the repository Go
toolchain, runs the candidates in separate containers, and writes evidence only
under the external `-EvidenceRoot`.

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

## Evidence and result

Expected output is one `summary.json` and one secret evidence directory per
candidate. `summary.json` records useful-work hashes, timings, DNS count,
process/resource observations, shutdown rung, and the hash of the raw control
transcript. The raw evidence is not committed.

Until the missing observer, forced-shutdown, and residual checks in
R-036 are implemented, the summary verdict is deliberately `incomplete` even
when `useful_work_verified` is true.

The actual 2026-08-15 result and immutable binary hashes are recorded in R-036
after the run. A future supply-preflight rejection would remain a rejection even
if an isolated functional probe happened to carry bytes.

## Limitations and disposition

This is a single-host feasibility exercise, not an R-037 blocked-network
campaign, performance qualification, anonymity measurement, or censorship-
resistance claim. Docker's internal network and synthetic TLS front are
controlled fixtures. Retain this small harness as reproducible evidence;
delete all generated artifacts and secret evidence after the record has been
reviewed.
