# Dependency register

Every runtime dependency must be entered here before it is added to `go.mod`.
The entry must name the need, owner, exact module, reviewed version, license,
maintenance and security signals, alternatives considered, and removal plan.

## Current runtime dependencies

None. The project currently uses the Go standard library only.

## Development tools

| Tool | Version | Purpose |
|---|---:|---|
| Go | 1.26.x; CI and Carrier Lab pin 1.26.5 | compiler, formatter, tests, vet |
| Staticcheck | 2025.1.1 | additional correctness analysis |
| govulncheck | v1.1.4 | reachable Go vulnerability analysis |

`make tools-install` is the only documented installation command. Normal build
and quick-check targets never install or upgrade tools implicitly.

## Carrier Lab-only tool inputs

These tools are not product runtime dependencies and do not enter `go.mod` or
the `application` image target. They exist only in the disposable `tooling`
target of `carrier-lab/Dockerfile`.

| Tool | Version | Supplied by | License summary | Purpose |
|---|---:|---|---|---|
| iproute2 `tc` | 6.19.0 | Ubuntu `iproute2` 6.19.0-1ubuntu1.1 | GPL-2.0-only | fixed endpoint `netem` shaping |
| tcpdump | 4.99.6 | Ubuntu `tcpdump` 4.99.6-1 | BSD-3-Clause with historical notices | bounded link capture and readback |
| libpcap | 1.10.6 | Ubuntu `libpcap0.8t64` 1.10.6-1ubuntu1 | BSD-family and other permissive notices | packet socket/filter runtime |

The exact 12-file runtime closure, official URLs, versions, SHA-256 values,
license summaries, installed paths, and executable hashes are normative in
[`carrier-lab/tools.lock`](../../carrier-lab/tools.lock). R-025 records the
source and security review. The external `.deb` bundle is an explicitly
prepared input outside Git. Normal build and run use no package repository,
installer, maintainer script, or download fallback; a missing, extra, or
mismatched artifact fails closed.
