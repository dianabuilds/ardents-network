# Dependency register

Every runtime dependency must be entered here before it is added to `go.mod`.
The entry must name the need, owner, exact module, reviewed version, license,
maintenance and security signals, alternatives considered, and removal plan.

## Current runtime dependencies

The maintained product-shaped Modules use the Go standard library plus the
Windows-only `golang.org/x/sys/windows` ACL surface described below. Gate C
adds the following exact runtime closure to the maintained
`internal/lab/namedsite` laboratory Module. The OHTTP closure is selected by
[R-026](../research/records/r-026-private-resolution-adapter.md), while the bounded
external socket fault use is selected by [R-032](../research/records/r-032-h3-same-connection-recovery.md); the set must enter
`go.mod` as this reviewed set rather than as the vulnerable versions declared
by `openpcc/ohttp v0.0.80`.

| Module | Reviewed version | License | Purpose |
|---|---:|---|---|
| `github.com/openpcc/ohttp` | `v0.0.80`, commit `79bec89d804248df1a71a0f56c882b116579035d` | Apache-2.0 | RFC 9458 client and Gateway encapsulation |
| `github.com/openpcc/twoway` | `v0.0.73` | Apache-2.0 | request/response HPKE context used by OHTTP |
| `github.com/openpcc/bhttp` | `v0.0.73` | Apache-2.0 | RFC 9292 known-length HTTP encoding |
| `github.com/cloudflare/circl` | `v1.6.3` | BSD-3-Clause | reviewed HPKE implementation; raised from vulnerable `v1.6.1` |
| `github.com/quic-go/quic-go` | `v0.57.1` | MIT | QUIC varint implementation required by BHTTP |
| `github.com/cespare/xxhash/v2` | `v2.3.0` | MIT | tracing dependency closure |
| `go.opentelemetry.io/otel` | `v1.39.0` | Apache-2.0 | OHTTP tracing types; Gate C emits no external telemetry |
| `go.opentelemetry.io/otel/trace` | `v1.39.0` | Apache-2.0 | OHTTP tracing Interface |
| `golang.org/x/crypto` | `v0.51.0` | BSD-3-Clause | selected cryptographic support closure |
| `golang.org/x/net` | `v0.55.0` | BSD-3-Clause | BHTTP HTTP support; raised from vulnerable `v0.48.0` |
| `golang.org/x/sys` | `v0.45.0` | BSD-3-Clause | Windows owner-only DACL enforcement plus Linux/Windows atomic no-replace Stage 5 evidence publication; transitive Gate C operating-system support elsewhere |
| `golang.org/x/text` | `v0.39.0` | BSD-3-Clause | BHTTP normalization; raised from vulnerable `v0.32.0` |

**Need and owner:** RFC 9458 is the accepted external-first Private Resolution
shape. `internal/lab/namedsite` is the sole first-party owner of the OHTTP/CIRCL
Interface. No product Module imports the OHTTP/CIRCL portion of this closure.

**Stage 5 Windows ACL need and owner:** `internal/bridge`,
`internal/localroles`, `internal/lab/blockedentry`, and
`internal/lab/blockedverify` use only
`golang.org/x/sys/windows` on Windows to apply a
protected DACL granting the current process owner full control and nobody else.
The module was already pinned and reviewed in the Gate C closure; this makes
that exact version a direct platform-specific product dependency. It avoids a
child PowerShell process during Invite import and avoids first-party `unsafe`.
Unix builds retain the standard-library permission implementation.
`x/sys` is the Go project's maintained, tagged operating-system support module;
the selected version has the existing checksum/license review, passes the
repository's offline build/tests and reachable vulnerability scan, and the four
callers use no cgo or first-party `unsafe`.

**Stage 5 evidence-publication need and owner:**
`internal/lab/blockedentry` uses `unix.Renameat2(..., RENAME_NOREPLACE)` on
Linux and `windows.MoveFile` on Windows to publish one completed external
evidence directory without replacing an existing path. The standard-library
`os.Rename` may replace an existing empty directory on Unix, so it cannot meet
the immutable/no-replay publication contract. Other platforms fail closed.
This use is removed when the standard library exposes a portable atomic
no-replace directory rename; copying or check-then-rename remain rejected
because they expose partial evidence or a replacement race.

`internal/lab/blockedverify` uses `unix.Flock` or Windows `LockFileEx` plus the
platform atomic replace/write-through operations to serialize and durably
replace its external consumed-run registry. The lock is advisory and released
by the operating system after a verifier crash; the single registry state file
is recovered from any uncommitted temporary successor. This use is removed
when the standard library provides equivalent cross-process file locking and a
durable atomic replacement primitive on both maintained platforms.

**Maintenance and security review:** `openpcc/ohttp` has versioned releases, an
Apache-2.0 license, tests including RFC vectors and malformed inputs, and a
published security contact. Its selected tag predates three now-known reachable
dependency advisories, so the raised versions above are mandatory. On Go
1.26.5 the exact set passes checksums, upstream and independent role-view tests,
offline build/test with cgo disabled, and reachable `govulncheck`. The reachable
Go packages have no cgo files or `unsafe` imports. CIRCL contains optimized
assembly behind portable Go APIs; Gate C selects no custom cryptographic suite.

**Alternatives:** `chris-wood/ohttp-go` at commit `776f22a178b8` has a smaller
MIT/BSD closure and passes after CIRCL is raised to `v1.6.3`, but has no release
and declares its implementation/API experimental. First-party OHTTP/PIR, local
lookup, direct/DNS/HTTP resolution, alternate Namespace, and cached-success
fallback are rejected.

For Windows ACL enforcement, an external PowerShell/`icacls` subprocess breaks
the no-process import contract, while raw first-party system calls require the
forbidden `unsafe` surface. Default inherited ACLs are not owner-only and fail
closed security requirements. Remove the direct use when the Go standard
library exposes equivalent protected-DACL construction and inspection; a
version change repeats dependency, license, advisory, Windows behavior, and
offline-build review.

**Offline supply:** an explicit preparation step runs `go mod download` and
`go mod verify` outside the repository, then supplies a temporary vendor context
to a Docker build with `--network=none`. No vendor tree, module cache, generated
dependency, or Gateway key is committed.

**Removal plan:** the complete closure leaves with the Gate C OHTTP Adapter. A
changed version or dependency graph repeats R-026; an unremediated reachable
high/critical vulnerability, unacceptable license, offline-build failure, or
broken role split selects `stop`, not a fork.

## Stage 5 external process runtime

The H3 Camouflage Adapter owns exactly two Linux `amd64` executables built from
standalone WebTunnel `v0.0.6`: client SHA-256
`de581c8dd36193bb4168aee840406294af406bf8187817c10ac2bcd9464fd120`
(`7,690,615` bytes) and server SHA-256
`5fe32f8ab736ed54fc66027775761084e68f0e1ec9b5fea7c3417c6617255336`
(`5,899,325` bytes). `internal/camouflage` is their sole first-party owner.
They are external processes, never linked into the root module, and add no
`go.mod` requirement.

**Source and license:** the signed upstream tag `v0.0.6` resolves to commit
`d729fde1f38357dcefa2a751eb4752e9ca78f910`, tree
`fe82090e6a523d1b05d602f934fda515354c8cf5`, canonical archive SHA-256
`aee07ac29c683e86d25cd5e02f046628da724f1f14197baeeebe2b6e70afa13c5`,
and MIT license. Its sole dependency is goptlib at signed tag `v1.6.0`, resolving
to commit `f4bb5dd5725833bd880347b8fbaf60522ed0a710`, tree
`ada1d375537ffa240b9d0eb91be505f9eb80c60b`, canonical archive SHA-256
`d239acc63dfbb1ad6929e6701fbe379b9b3380d3d7c954d99e5fa748b5c49af7`,
CC0-1.0 dedication, module sum
`h1:KD9m+mRBwtEdqe94Sv72uiedMWeRdIr4sXbrRyzRiIo=`, and `go.mod` sum
`h1:70bhd4JKW/+1HLfm+TMrgHJsUHG4coelMWwiVEJ2gAg=`. The sorted nine-file
vendor manifest has aggregate SHA-256
`e4a886933f85ba87137d011fd66dd20fa122dce32af208f249d5bf17f76bf64a`.

**Offline build and supply:** preparation uses the Ubuntu image
`ubuntu@sha256:7b202b0e2e0028c6250f5fcf41d04df492d145a1654c6995a6553f0c1f6f1960`,
official `go1.26.6.linux-amd64.tar.gz` SHA-256
`708effb774be8237570d0add163225abbdfaf4fca28b2611df167beba4feef89`,
and `govulncheck v1.1.4`. An owned empty-cache preparation downloads the module
graph, verifies it, creates the vendor tree, and scans the complete package
graph with the exact R-036 commands,
then records the source, module, vendor, license, SBOM, advisory, and recipe
hashes. Two clean `--network none` builds run with `CGO_ENABLED=0`,
`GOOS=linux`, `GOARCH=amd64`, `GOPROXY=off`, `GOSUMDB=off`, `-mod=vendor`,
`-trimpath`, `-buildvcs=false`, and an empty build ID against `./main/client`
and `./main/server`; both builds must reproduce the binary hashes above. At
runtime the image verifies those hashes before startup and cannot fetch or
repair an input. Source, vendor, caches, binaries, keys, and build evidence stay
outside Git.

**Maintenance and security:** R-036 found no reachable vulnerability with the
recorded database timestamp `2026-08-14T16:22:54Z`, no cgo or Go `unsafe`
import in WebTunnel/goptlib, and one small linked dependency. Cure53's 2024
assessment included WebTunnel but does not establish the pinned 2026 revision
or Ardents integration as safe. Before an integration campaign, the owner
repeats checksum, signature, license, SBOM, and current reachable-advisory
review; any changed source, dependency, toolchain, image, license, recipe, or
binary identity reopens R-036 rather than being upgraded in place.

**Alternatives and removal:** R-036 rejected a first-party camouflage protocol
and selected no Lyrebird/obfs4 fallback. Default WebTunnel discovery, public
DNS, ambient proxy configuration, and runtime download remain forbidden.
Removing Stage 5 Camouflage deletes both external binaries and the
`internal/camouflage` process owner without changing Route, Service Connection,
or Application interfaces.

The native Carrier Lab candidate uses the Go 1.26 standard-library
`crypto/hpke`, `crypto/tls`, `crypto/x509`, and `crypto/ecdh` implementations.
They add no module dependency. First-party cryptographic primitives, cgo, and
`unsafe` remain forbidden; replacing a standard implementation requires the
normal dependency review before `go.mod` changes.

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
target of `lab/carrier/Dockerfile`.

| Tool | Version | Supplied by | License summary | Purpose |
|---|---:|---|---|---|
| iproute2 `tc` | 6.19.0 | Ubuntu `iproute2` 6.19.0-1ubuntu1.1 | GPL-2.0-only | fixed endpoint `netem` shaping |
| tcpdump | 4.99.6 | Ubuntu `tcpdump` 4.99.6-1 | BSD-3-Clause with historical notices | bounded link capture and readback |
| libpcap | 1.10.6 | Ubuntu `libpcap0.8t64` 1.10.6-1ubuntu1 | BSD-family and other permissive notices | packet socket/filter runtime |

The exact 12-file runtime closure, official URLs, versions, SHA-256 values,
license summaries, installed paths, and executable hashes are normative in
[`lab/carrier/tools.lock`](../../lab/carrier/tools.lock). R-025 records the
source and security review. The external `.deb` bundle is an explicitly
prepared input outside Git. Normal build and run use no package repository,
installer, maintainer script, or download fallback; a missing, extra, or
mismatched artifact fails closed.

## Carrier Lab external reference inputs

The R-013 comparison uses Tor and Chutney only as a black-box laboratory
reference. They are not linked into the Go binary, included in either Carrier
Lab image, or selected as a product runtime foundation.

| Input | Reviewed version | License | Purpose |
|---|---:|---|---|
| Tor | Ubuntu 0.4.9.6-1 package closure; upstream 0.4.9.11 recorded | BSD-3-Clause | private onion-service reference |
| Chutney | revision `988fc372cc418fbecc60558fe27e75d07d76b996` | BSD-3-Clause | isolated local Tor network |
| typeguard | 4.3.0 | MIT | Chutney runtime validation |
| tomli-w | 1.2.0 | MIT | Chutney configuration writer |
| typing-extensions | 4.15.0 | PSF-2.0 | Python compatibility for Chutney/typeguard |

[`lab/carrier/reference.lock`](../../lab/carrier/reference.lock) is normative
for the exact Tor 13-package closure, Chutney archive, wheels, source locations,
and SHA-256 identities. The selected Chutney revision predates its optional SSH
launcher and restricted-discovery dependencies, so Paramiko, rpyc, and Python
`cryptography` are deliberately absent. Online preparation is explicit; the
experiment verifies and consumes the prepared directory without downloading.
