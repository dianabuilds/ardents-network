# Dependency register

Every runtime dependency must be entered here before it is added to `go.mod`.
The entry must name the need, owner, exact module, reviewed version, license,
maintenance and security signals, alternatives considered, and removal plan.

## Current runtime dependencies

The maintained product-shaped Modules use the Go standard library, the
Windows-only `golang.org/x/sys/windows` surfaces described below, and the exact
OHTTP closure owned by `internal/naming/resolution` and
`internal/service/reachability`. ADR-0014 selects the
maintained private-resolution profile; the set must enter
`go.mod` as this reviewed set rather than as the vulnerable versions declared
by `openpcc/ohttp v0.0.80`.

| Module | Reviewed version | License | Purpose |
|---|---:|---|---|
| `github.com/openpcc/ohttp` | `v0.0.80`, commit `79bec89d804248df1a71a0f56c882b116579035d` | Apache-2.0 | RFC 9458 client and Gateway encapsulation |
| `github.com/openpcc/twoway` | `v0.0.80` | Apache-2.0 | request/response HPKE context used by OHTTP |
| `github.com/openpcc/bhttp` | `v0.0.80` | Apache-2.0 | RFC 9292 known-length HTTP encoding |
| `github.com/cloudflare/circl` | `v1.6.5` | BSD-3-Clause | reviewed HPKE implementation |
| `github.com/quic-go/quic-go` | `v0.61.0` | MIT | maintained QUIC v1 Carrier Adapter and QUIC varint closure required by BHTTP |
| `github.com/cespare/xxhash/v2` | `v2.3.0` | MIT | tracing dependency closure |
| `go.opentelemetry.io/otel` | `v1.45.0` | Apache-2.0 | OHTTP tracing types |
| `go.opentelemetry.io/otel/trace` | `v1.45.0` | Apache-2.0 | OHTTP tracing Interface |
| `golang.org/x/crypto` | `v0.55.0` | BSD-3-Clause | selected cryptographic support closure |
| `golang.org/x/net` | `v0.58.0` | BSD-3-Clause | BHTTP HTTP support |
| `golang.org/x/sys` | `v0.47.0` | BSD-3-Clause | Windows owner-only DACL/locking and registry enforcement plus platform atomic replacement support |
| `golang.org/x/text` | `v0.41.0` | BSD-3-Clause | BHTTP normalization |

**Need and owner:** RFC 9458 is the accepted external-first Private Resolution
shape. `internal/naming/resolution` owns the Namespace OHTTP/CIRCL Adapter and
`internal/service/reachability` owns the separately authenticated Target
descriptor adapter; neither is a general HTTP proxy. A change repeats R-047/R-026
instead of allowing its cryptographic configuration to drift.

**Windows platform use:** current platform-specific owners use
`golang.org/x/sys/windows` on Windows to apply a protected DACL granting the
current process owner full control and nobody else. The module is a direct
platform-specific product dependency. It avoids a child PowerShell process and
avoids first-party `unsafe`. Unix builds retain the standard-library permission
implementation.
`x/sys` is the Go project's maintained, tagged operating-system support module;
the selected version has the existing checksum/license review, passes the
repository's offline build/tests and reachable vulnerability scan, and the
remaining callers use no cgo or first-party `unsafe`.

**Maintenance and security review:** `openpcc/ohttp` has versioned releases, an
Apache-2.0 license, tests including RFC vectors and malformed inputs, and a
published security contact. Its selected tag predates three now-known reachable
dependency advisories, so the raised versions above are mandatory. On Go
1.26.6 the exact set passes checksums, upstream and independent role-view tests,
offline build/test with cgo disabled, and reachable `govulncheck`. The reachable
Go packages have no cgo files or `unsafe` imports. CIRCL contains optimized
assembly behind portable Go APIs; Ardents selects no custom cryptographic suite.

**Alternatives:** `chris-wood/ohttp-go` at commit `776f22a178b8` has a smaller
MIT/BSD closure and passes with CIRCL `v1.6.5`, but has no release
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

**QUIC Carrier use:** `internal/route` directly imports pinned
`github.com/quic-go/quic-go v0.61.0` for the maintained
`ardents-carrier-quic-v1` Adapter selected by ADR-0048. The module is pure Go,
MIT licensed, supports the repository toolchain, publishes security reporting
and advisories, and passed R-094's exact-version binary vulnerability, TLS peer
binding, LegBinding, timeout, cleanup, selective blocking, loss/reorder,
MTU-1280, NAT-rebinding, and separate-host checks. `golang.org/x/net/quic` was
rejected because upstream still describes it as work in progress; a first-party
QUIC implementation is forbidden cryptographic/protocol work; TCP-only cannot
exercise the required second Carrier seam. Remove this direct use if QUIC is
withdrawn as a maintained profile. Any version change repeats license,
advisory, MTU, cancellation, resource, and hostile-network qualification.

**Offline supply:** an explicit preparation step runs `go mod download` and
`go mod verify` outside the repository, then supplies a temporary vendor context
to a Docker build with `--network=none`. No vendor tree, module cache, generated
dependency, or Gateway key is committed.

**Removal plan:** the complete closure leaves only when the product private
resolution Adapter is removed. A changed version or
dependency graph repeats R-047 and R-026; an unremediated reachable
high/critical vulnerability, unacceptable license, offline-build failure, or
broken role split selects `stop`, not a fork.

## Current release-verifier closure

Status: **current maintained dependency.** R-049 selected the following exact
reviewed module closure; the current `internal/release` package, behavior
tests, non-test caller, and package-map entry now own it.

| Module | Proposed version | License | Purpose |
|---|---:|---|---|
| `github.com/theupdateframework/go-tuf/v2` | `v2.4.2`, commit `f5edbde31e5507f46db2069402dc38903fe6d9d4` | Apache-2.0 | TUF metadata and trusted-metadata workflow |
| `github.com/cenkalti/backoff/v5` | `v5.0.3` | MIT | transitive go-tuf module dependency; absent from the maintained package import path |
| `github.com/google/go-containerregistry` | `v0.21.9` | Apache-2.0 | signature/key conversion closure |
| `github.com/opencontainers/go-digest` | `v1.0.0` | Apache-2.0 | digest conversion closure |
| `github.com/secure-systems-lab/go-securesystemslib` | `v0.11.1` | MIT | maintained signing-verification support used by go-tuf metadata |
| `github.com/sigstore/protobuf-specs` | `v0.5.2` | Apache-2.0 | signature verification type closure |
| `github.com/sigstore/sigstore` | `v1.10.9` | Apache-2.0 | public-key verification adapter used by maintained Release verification and its behavior fixtures |
| `github.com/youmark/pkcs8` | `v0.0.0-20240726163527-a2c0da244d78` | MIT | PKCS#8 parsing closure required by Sigstore |
| `golang.org/x/crypto` | `v0.55.0` | BSD-3-Clause | raised cryptographic support closure |
| `golang.org/x/sys` | `v0.47.0` | BSD-3-Clause | raised platform support closure; already selected elsewhere |
| `golang.org/x/term` | `v0.45.0` | BSD-3-Clause | no-echo terminal secret input for `cmd/ardents-custody`; also present in the reviewed sigstore closure |
| `google.golang.org/genproto/googleapis/api` | `v0.0.0-20260819154853-08b0e4226688` | Apache-2.0 | protobuf API type closure |
| `google.golang.org/protobuf` | `v1.36.12` | BSD-3-Clause | signature protobuf runtime |

**Need and owner:** the Release Decision Module owns verification.
The maintained verification path imports go-tuf `metadata` and
`trustedmetadata`. The broader reviewed
updater closure remains the removal/review boundary but is not imported by
other production code. Release Decision receives bounded bytes, trusted root, exact target
identity, and artifact bytes. It constructs one trusted set, assigns the one
captured UTC `RefTime` before the first expiry check, then executes the standard
consecutive root, timestamp, snapshot, and top-level targets update methods.
The Module owns neither network
retrieval nor persistent cache, general repository/signing administration,
multi-repository maps, delegated targets, installation, or activation.
Ardents-owned durable `version + digest` floors for root, timestamp, snapshot,
and top-level targets are mandatory inputs. Before `release-accepted`, the owner
atomically publishes the candidate-verified consecutive root chain and floor
successors. No go-tuf cache or updater is constructed, so candidate cache remains
absent and can never become a watermark.

**Review evidence:** the completed selection measured exact source identities,
`108/108` TUF conformance, Windows/Linux
upstream tests, ten-run no-cgo resource tests, permissive-license inventory,
and the reachable scan. The raised three-module set preserved upstream and
profile tests. `govulncheck` reported no symbol or imported-package
vulnerability; its remaining module-only finding is the unimported and
unmaintained `x/crypto/openpgp` package. Integration repeats the complete root
module scan and stops on any reachable unpatched high/critical advisory.

**Alternatives and removal:** the DataDog legacy fork failed the reproducible
maintenance/conformance criterion; first-party TUF or cryptographic primitives,
distributor authority, and a hand-built generic threshold workflow are rejected.
The closure is removed with the Release Decision Module. A version, module,
surface, role, delegation, cache, or multi-repository change requires a new
dependency review and applicable ADR analysis.

**Offline supply:** integration adds checksums only after review, prepares the
module cache outside Git, verifies it online, and proves an offline no-cgo
build. No module cache, vendor tree, generated repository, key, or binary
belongs in the repository.

## Current Authority custody dependencies

Status: **current maintained dependency under ADR-0021.** Password-derived
Authority Custody uses `golang.org/x/crypto/argon2` from module
`golang.org/x/crypto v0.55.0` (BSD-3-Clause). Other maintained cryptographic
owners select the same module version; integration must produce one shared
exact root-module version, never parallel copies.

`internal/custody` is the only maintained Argon2 caller. It uses only
`argon2.IDKey` with the fixed v1 profile and passes the derived
32-byte key to Go 1.26 standard-library `crypto/aes` and
`cipher.NewGCMWithRandomNonce`. No other
Argon2 variant, dynamic parameter negotiation, signing primitive, password
store, DPAPI/Secret Service wrapper, cgo, or `unsafe` is selected. The current
[release, update, and Authority Custody reference](../technical/release-update-custody.md)
owns the maintained boundary.

`cmd/ardents-custody` is the separate interactive Adapter. It imports only
`golang.org/x/term` to reject a non-terminal descriptor and read a password
without echo. It accepts no
password from arguments, environment, configuration, nor a stream shared with
Application data; it exposes no decrypted material.

Before a supported custody handoff,
integration must run official exact-version Argon2id vectors, the fixed 256 MiB
resource profile, license/source identity, and reachable-advisory checks.
Weakest-native-host performance remains a separate Qualification gate. Removing
password-derived custody removes these callers. A version/profile/surface change
requires a new dependency review and applicable ADR analysis. ADR-0067 retires
the former release-seed and fixed State-genesis Argon2 callers as historical
ceremony implementations.

## Current qualification-only dependencies

Status: **test-only; absent from every product binary and enrollment
inventory.** The artifact-native custody process test imports one PTY harness
so it can drive the real terminal-only `ardents-custody` executable on Windows
and Unix without adding a password flag, environment variable, shared input
stream, first-party `unsafe`, or fixture custody command.

| Module | Reviewed version | License | Purpose |
|---|---:|---|---|
| `github.com/aymanbagabas/go-pty` | `v0.2.3`, commit `b1081175e7d78aa5e2fd02f88bcbc0af4e280039` | MIT | Cross-platform test PTY; Windows uses the native ConPTY API and Unix uses a normal PTY |
| `github.com/creack/pty` | `v1.1.24` | MIT | Unix-only transitive PTY implementation |
| `github.com/u-root/u-root` | `v0.16.0` | BSD-3-Clause | Declared transitive terminal support closure; not imported by the Windows process-test build |

**Need and owner:** `tests/e2e/service` owns the dependency. The supported
custody product deliberately accepts secrets only from a real no-echo terminal,
while Windows `os/exec` cannot attach a native child to ConPTY through its
portable public API. `go-pty` exposes one `io.ReadWriteCloser` plus an
`exec.Cmd`-like command boundary on both selected development platforms. The
test waits for each exact custody prompt before writing a fixed test-only
password and asserts that the terminal transcript never contains that secret.
It executes the exact built `ardents-custody` and `ardents` files; it neither
imports `internal/custody` nor constructs a credential response itself.

**Maintenance, security, and distribution review:** the current tagged
`v0.2.3` release was published on 2026-05-17 from a GitHub-verified commit; its
repository has current Windows-specific fixes and documents Windows ConPTY and
Unix PTY support. The module is pre-v1 and has no declared security policy, so
it is not accepted into a runtime or shipped artifact. Its first-party test
caller imports no `unsafe` or cgo; the dependency's platform implementation may
use operating-system primitives internally. MIT and BSD-3-Clause permit the
test-only source dependency. `go mod verify`, the exact process test, Windows
package dependency inspection, `govulncheck`, and artifact dependency checks
must pass before integration. Product manifests and archives must prove that
none of these modules contributes a file or import edge to a shipped binary.

**Alternatives and removal:** an ordinary pipe is intentionally rejected by
the custody binary and would weaken the product contract. The available MSYS
`script` PTY is not a Windows console handle, while combining unrelated MSYS
and Git-for-Windows `winpty` runtimes did not provide a stable native terminal.
A first-party ConPTY wrapper would require forbidden `unsafe`; the lower-level
Windows-only `github.com/UserExistsError/conpty` would still need a separate
Unix dependency and more local process orchestration. Remove `go-pty` when Go's
standard `os/exec` exposes a supported cross-platform PTY/ConPTY attachment or
when the exact terminal ceremony is moved to an equally reproducible external
artifact qualification runner. Any version or runtime use repeats this review.

## Development tools

| Tool | Version | Purpose |
|---|---:|---|
| Go | 1.26.x; CI pins 1.26.6 | compiler, formatter, tests, vet |
| Staticcheck | 2025.1.1 | additional correctness analysis |
| govulncheck | v1.1.4 | reachable Go vulnerability analysis |

`make tools-install` is the only documented installation command. Normal build
and quick-check targets never install or upgrade tools implicitly.
