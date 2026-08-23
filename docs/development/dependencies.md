# Dependency register

Every runtime dependency must be entered here before it is added to `go.mod`.
The entry must name the need, owner, exact module, reviewed version, license,
maintenance and security signals, alternatives considered, and removal plan.

## Current runtime dependencies

The maintained product-shaped Modules use the Go standard library, the
Windows-only `golang.org/x/sys/windows` surfaces described below, and the exact
OHTTP closure owned by `internal/naming/resolution`. The product promotion is selected by
[R-047](../research/records/r-047-stage-6-query-hiding.md) and ADR-0014; the
original experiment selection is recorded by
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
| `go.opentelemetry.io/otel` | `v1.39.0` | Apache-2.0 | OHTTP tracing types |
| `go.opentelemetry.io/otel/trace` | `v1.39.0` | Apache-2.0 | OHTTP tracing Interface |
| `golang.org/x/crypto` | `v0.51.0` | BSD-3-Clause | selected cryptographic support closure |
| `golang.org/x/net` | `v0.55.0` | BSD-3-Clause | BHTTP HTTP support; raised from vulnerable `v0.48.0` |
| `golang.org/x/sys` | `v0.45.0` | BSD-3-Clause | Windows owner-only DACL enforcement and platform atomic replacement support |
| `golang.org/x/text` | `v0.39.0` | BSD-3-Clause | BHTTP normalization; raised from vulnerable `v0.32.0` |

**Need and owner:** RFC 9458 is the accepted external-first Private Resolution
shape. `internal/naming/resolution` owns the maintained product OHTTP/CIRCL Adapter.
No other product Module imports this closure. A change repeats R-047/R-026
instead of allowing its cryptographic configuration to drift.

**Stage 5 Windows ACL historical record and remaining owners:** the removed
`internal/bridge`, Stage-5 evidence generator, and R-090-retired
`internal/lab/blockedverify` formerly used this Windows ACL path. The retained
platform-specific owners use `golang.org/x/sys/windows` on Windows to apply a
protected DACL granting the current process owner full control and nobody else.
The module is a direct platform-specific product dependency. It avoids a
child PowerShell process during Invite import and avoids first-party `unsafe`.
Unix builds retain the standard-library permission implementation.
`x/sys` is the Go project's maintained, tagged operating-system support module;
the selected version has the existing checksum/license review, passes the
repository's offline build/tests and reachable vulnerability scan, and the
remaining callers use no cgo or first-party `unsafe`.

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

**Removal plan:** the complete closure leaves only when both the product private
resolution Adapter and Gate C Adapter are removed. A changed version or
dependency graph repeats R-047 and R-026; an unremediated reachable
high/critical vulnerability, unacceptable license, offline-build failure, or
broken role split selects `stop`, not a fork.

## Selected Stage 7 release-verifier closure

Status: **accepted for S7.1 integration on 2026-08-20.** R-049 selects the
following exact reviewed module closure. It enters `go.mod` only with the real
Release Decision package, behavior tests, non-test caller, and package-map entry.

| Module | Proposed version | License | Purpose |
|---|---:|---|---|
| `github.com/theupdateframework/go-tuf/v2` | `v2.4.2`, commit `f5edbde31e5507f46db2069402dc38903fe6d9d4` | Apache-2.0 | TUF metadata and trusted-metadata workflow |
| `github.com/cenkalti/backoff/v5` | `v5.0.3` | MIT | transitive go-tuf module dependency; absent from the maintained package import path |
| `github.com/google/go-containerregistry` | `v0.20.7` | Apache-2.0 | signature/key conversion closure |
| `github.com/opencontainers/go-digest` | `v1.0.0` | Apache-2.0 | digest conversion closure |
| `github.com/secure-systems-lab/go-securesystemslib` | `v0.11.0` | MIT | maintained signing-verification support used by go-tuf metadata |
| `github.com/sigstore/protobuf-specs` | `v0.5.0` | Apache-2.0 | signature verification type closure |
| `github.com/sigstore/sigstore` | `v1.10.6` | Apache-2.0 | public-key signature verification adapter used by go-tuf |
| `golang.org/x/crypto` | `v0.52.0` | BSD-3-Clause | raised cryptographic support closure |
| `golang.org/x/sys` | `v0.45.0` | BSD-3-Clause | raised platform support closure; already selected elsewhere |
| `golang.org/x/term` | `v0.43.0` | BSD-3-Clause | no-echo terminal secret input for `cmd/ardents-custody`; also present in the reviewed sigstore closure |
| `google.golang.org/genproto/googleapis/api` | `v0.0.0-20250825161204-c5933d9347a5` | Apache-2.0 | protobuf API type closure |
| `google.golang.org/protobuf` | `v1.36.11` | BSD-3-Clause | signature protobuf runtime |

**Need and owner:** the Release Decision Module is the sole owner. Its maintained
path imports only go-tuf `metadata` and `trustedmetadata`; the broader reviewed
updater closure remains the removal/review boundary but is not imported by
production code. The Module receives bounded bytes, trusted root, exact target
identity, and artifact bytes. It constructs one trusted set, assigns the one
captured UTC `RefTime` before the first expiry check, then executes the standard
consecutive root, timestamp, snapshot, and top-level targets update methods.
The Module owns neither network
retrieval nor persistent cache, repository/signing administration,
multi-repository maps, delegated targets, installation, or activation.
Ardents-owned durable `version + digest` floors for root, timestamp, snapshot,
and top-level targets are mandatory inputs. Before `release-accepted`, the owner
atomically publishes the candidate-verified consecutive root chain and floor
successors. No go-tuf cache or updater is constructed, so candidate cache remains
absent and can never become a watermark.

**Review evidence:** [R-049](../research/records/r-049-stage-7-release-verifier.md)
records exact source identities, `108/108` TUF conformance, Windows/Linux
upstream tests, ten-run no-cgo resource tests, permissive-license inventory,
and the reachable scan. The raised three-module set preserved upstream and
profile tests. `govulncheck` reported no symbol or imported-package
vulnerability; its remaining module-only finding is the unimported and
unmaintained `x/crypto/openpgp` package. Integration repeats the complete root
module scan and stops on any reachable unpatched high/critical advisory.

**Alternatives and removal:** the DataDog legacy fork failed the reproducible
maintenance/conformance criterion; first-party TUF or cryptographic primitives,
distributor authority, and a hand-built threshold workflow are rejected. The
closure is removed with the one Release Decision Module. A version, module,
surface, role, delegation, cache, or multi-repository change reopens R-049.

**Offline supply:** S7.1 must add checksums only after acceptance, prepare the
module cache outside Git, verify it online, and prove an offline no-cgo build.
No module cache, vendor tree, generated repository, key, or binary belongs in
the repository.

## Selected Stage 7 Ubuntu isolation runtime

Status: **accepted for the S7.6 Ubuntu Adapter under ADR-0016; native-host
qualification remains deferred where Docker cannot observe the fact.** R-052 freezes upstream bubblewrap
`v0.11.2` (`LGPL-2.0-or-later`) as the external-process candidate for
`ubuntu-bwrap-native-v1`. Each native qualification campaign must pin the exact
Ubuntu package/source, executable SHA-256, dynamic-library closure, build
options, and current advisory state. The Ubuntu 26.04 Docker development run
pins the same facts but cannot qualify native Ubuntu Desktop integration or
host containment that the container cannot observe. The executable must have setuid support disabled and
must carry no setuid/setgid bit or file capability.

The future Application Isolation Ubuntu Adapter is the sole owner. It supplies
one exact argument/environment/mount manifest, runs bubblewrap inside the R-051
cgroup/pidfd resource tree, and accepts only the inherited Broker descriptor and
declared context/runtime handles. Bubblewrap is never linked into `go.mod`,
downloaded at runtime, granted Authority access, or treated as the evidence
verdict. Installed may declare the reviewed package after acceptance; Portable
only preflights an already present matching host dependency and otherwise
returns `isolation-unsupported` while direct/generic use remains available.

R-052 selected bubblewrap instead of first-party namespace/mount/seccomp
machinery, setuid mode, a privileged daemon, or a kernel driver. Removing the
Ubuntu claim-bearing profile removes this process dependency without changing
the shared Application Broker or generic/direct Adapters. Any version/source/
build/capability/policy change reopens R-052 and repeats F-cell qualification.

## Selected Stage 7 Authority-envelope dependency

Status: **accepted for S7.2 integration under ADR-0021.** R-053 selects
`golang.org/x/crypto/argon2` from module
`golang.org/x/crypto v0.52.0` (BSD-3-Clause) as the sole non-standard-library
cryptographic dependency for password-derived Authority Custody. R-049 already
selects the same module version in its release-verifier closure; integration
must produce one shared exact root-module version, never parallel copies.

The Authority Custody Module is the sole caller. It uses only
`argon2.IDKey` with the fixed v1 profile and passes the derived 32-byte key to Go
1.26 standard-library `crypto/aes` and `cipher.NewGCMWithRandomNonce`. No other
Argon2 variant, dynamic parameter negotiation, signing primitive, password
store, DPAPI/Secret Service wrapper, cgo, or `unsafe` is selected. The
[R-053 record](../research/records/r-053-stage-7-authority-recovery.md) and
[exact Authority Custody specification](stage-7-authority-custody-spec.md) own vectors,
resource bounds, envelope rules, and removal.

`cmd/ardents-custody` is the separate interactive adapter and imports only
`golang.org/x/term` to reject a non-terminal descriptor and read one password
without echo for active-record verification. The adapter neither accepts a
password from arguments, environment, configuration, nor a stream shared with
Application data; it does not expose decrypted Authority material.

The disposable logic prototype first used the current root module's `v0.51.0`
to test format/state coherence, then repeated the full 64 MiB sequence in a
temporary module with exact `v0.52.0` at commit
`a1c0d9929856c8aba2b31f079340f00578eda803` and checksum
`h1:RMs7fP2rXdep0CftQlK8Uf+kibLm7qkCcradZWYz988=`. Both passed, but scheduled
development-host integration must still run official exact-version Argon2id
vectors, the fixed 256 MiB resource profile, license/source identity, and
reachable-advisory checks before the S7.2 handoff. Weakest-native-host
performance remains a separate qualification gate. Removing password-derived custody removes this caller; if
R-049 is also rejected/removed, the module can return to the independently
justified root version. A version/profile/surface change reopens R-053.

## Historical Stage 5 external process record

The former H3 Camouflage Adapter owned exactly two Linux `amd64` executables built from
standalone WebTunnel `v0.0.6`: client SHA-256
`de581c8dd36193bb4168aee840406294af406bf8187817c10ac2bcd9464fd120`
(`7,690,615` bytes) and server SHA-256
`5fe32f8ab736ed54fc66027775761084e68f0e1ec9b5fea7c3417c6617255336`
(`5,899,325` bytes). R-080 deleted `internal/camouflage` and active runners;
the identities below are C4 historical provenance, not a current external
process dependency or a `go.mod` requirement.

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
graph with the exact R-036 commands, then records the source, module, vendor,
license, SBOM, advisory, and recipe hashes. Two clean `--network none` builds
run with `CGO_ENABLED=0`, `GOOS=linux`, `GOARCH=amd64`, `GOPROXY=off`,
`GOSUMDB=off`, `-mod=vendor`, `-trimpath`, `-buildvcs=false`, and an empty build
ID against `./main/client` and `./main/server`; both builds must reproduce the
binary hashes above. At runtime the image verifies those hashes before startup
and cannot fetch or repair an input. Source, vendor, caches, binaries, keys,
and build evidence stay outside Git.

The Go archive identity comes from the official Go downloads JSON
`https://go.dev/dl/?mode=json&include=all`, accessed 2026-08-15, and was
cross-checked against the downloaded 66,890,545-byte archive as recorded in
R-036. Stage 5's versioned supply lock repeats that accepted identity; it does
not introduce a second source claim. The qualifying builder recipe is
`tests/live/stage5-final/go-builder.Dockerfile`; its SHA-256 is stored beside
the archive hash in the supply lock and embedded into the builder and product
receipts. A separately generated deterministic module-cache archive contains
the complete `go mod download all` graph; its hash is locked and embedded by
the same recipe, while the cache itself remains outside Git. Preparation
verifies the exact builder base ancestry and recipe/archive/module-cache labels
and receipt files before the offline product build. R-091 retired the former
module-cache generator; this Stage 5 supply text remains historical provenance
and cannot be activated by a current command.

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
R-080 removed Stage 5 Camouflage's binary owner and live runners without
changing Route, Service Connection, or Application interfaces.

The historical native Carrier Lab candidate used the Go 1.26 standard-library
`crypto/hpke`, `crypto/tls`, `crypto/x509`, and `crypto/ecdh` implementations.
They add no module dependency. First-party cryptographic primitives, cgo, and
`unsafe` remain forbidden; replacing a standard implementation requires the
normal dependency review before `go.mod` changes.

## Development tools

| Tool | Version | Purpose |
|---|---:|---|
| Go | 1.26.x; CI pins 1.26.5 | compiler, formatter, tests, vet |
| Staticcheck | 2025.1.1 | additional correctness analysis |
| govulncheck | v1.1.4 | reachable Go vulnerability analysis |

`make tools-install` is the only documented installation command. Normal build
and quick-check targets never install or upgrade tools implicitly.

## Historical Carrier Lab tool inputs

R-091 retired these inputs and their executable consumer. The details below are
C4 provenance for R-013/R-025, not current dependencies or build inputs.

These tools were not product runtime dependencies and did not enter `go.mod` or
the historical `application` image target. R-091 deleted their disposable
`tooling` target together with `lab/carrier/Dockerfile`.

| Tool | Version | Supplied by | License summary | Purpose |
|---|---:|---|---|---|
| iproute2 `tc` | 6.19.0 | Ubuntu `iproute2` 6.19.0-1ubuntu1.1 | GPL-2.0-only | fixed endpoint `netem` shaping |
| tcpdump | 4.99.6 | Ubuntu `tcpdump` 4.99.6-1 | BSD-3-Clause with historical notices | bounded link capture and readback |
| libpcap | 1.10.6 | Ubuntu `libpcap0.8t64` 1.10.6-1ubuntu1 | BSD-family and other permissive notices | packet socket/filter runtime |

The exact 12-file runtime closure, official URLs, versions, SHA-256 values,
license summaries, installed paths, and executable hashes remain in the
accepted [R-025 record](../research/records/r-025-carrier-lab-tool-supply.md).
The external `.deb` bundle was an explicitly prepared input outside Git. The
former build and run used no package repository,
installer, maintainer script, or download fallback; a missing, extra, or
mismatched artifact fails closed.

## Historical Carrier Lab external reference inputs

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

[R-025](../research/records/r-025-carrier-lab-tool-supply.md) retains the exact
Tor 13-package closure, Chutney archive, wheels, source locations, and SHA-256
identities. The selected Chutney revision predates its optional SSH
launcher and restricted-discovery dependencies, so Paramiko, rpyc, and Python
`cryptography` are deliberately absent. Online preparation is explicit; the
experiment verifies and consumes the prepared directory without downloading.
