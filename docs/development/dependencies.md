# Dependency register

Every runtime dependency must be entered here before it is added to `go.mod`.
The entry must name the need, owner, exact module, reviewed version, license,
maintenance and security evidence, alternatives considered, and removal plan.
Existing entries are subject to the same continuing review as new selections.

## Review decisions and evidence reuse

Keep three decisions distinct. **Investigation** permits a bounded research
probe with declared inputs, privileges and external evidence; it grants no
maintained runtime use. **Design selection** fixes the exact component, use,
configuration and replacement path after source/support/advisory review and
architecture-sensitive checks. It is sufficient to specify implementation
tasks. **Candidate admission** additionally requires the actual changed source,
build/test closure, artifacts and relevant integration evidence. A selected
library is not an already qualified implementation. These are review terms,
not runtime identities or an additional issue-status system.

The design assistant owns the selection record; the implementing change owns
its exact closure and candidate checks. The Product Owner and Codex are the
available reviewers. Independent review is a separate claim-specific release
gate, never an assumed member of this workflow.

Reuse a previous source, license, support or non-applicability assessment only
when its source identity, supported branch, use, targets, privileges and stated
invalidation conditions still match. Link that evidence and inspect changes;
do not repeat a historical research campaign just because its identifier is
in this register. Recheck advisories for each integration or qualification
candidate and reassess changed facts. A document-only change that cannot alter
the build, behavior, dependency closure or an acceptance decision needs document
validation, not an unrelated execution campaign.

Evidence must be readable and reproducible. Record the exact command, outcome,
source/build identity, tool/database identity and scope. A missing temporary
capture cannot support a new admission by its former filename alone; recover
it from the declared evidence store or reproduce it. A policy review does not
start an automation or promise continuous staffed monitoring.

## Maintenance and vulnerability acceptance

[NET-30](../product/functional-map.md#common-protection-requirement) requires
maintained dependencies and no known exploitable vulnerability in admitted use.
This covers direct and transitive dependencies, the Go toolchain and standard
library, and build, release, CI and test tools. Selected native libraries,
operating-system packages and container inputs belong in the relevant build or
environment inventory with the same review. A test-only dependency has a
different exposure and authority boundary; absence from product binaries does
not exempt the environment that executes it.

### Evidence for continued support

Each selection or renewal records dated primary evidence from upstream for
the project and the exact chosen release or supported branch: maintenance
status, security reporting and fix delivery, applicable end-of-support policy,
compatibility with our toolchain and targets, and material unresolved security
findings. A maintained project does not automatically support every old version.
A quiet stable library is not automatically abandoned; popularity, recent
commits or the absence of a security-policy file alone do not settle the review.
The selected version need not be the newest when its branch remains supported
and the relevant fixes are available in that version. Inspect newer patches
on that branch before selecting a new candidate; explain retention of an older
patch when the changes can affect the admitted use. An upstream project or
major branch being supported does not mean every superseded patch receives
separate fixes.

Record exact source identity and integrity checks, license, full dependency
closure, actual imports or executable inputs, privileges and exposed data,
the responsible Ardents owner, review date and update/removal path. Review
upstream and advisory evidence for the packages we use, including compiler and
build-time exposure that a runtime call graph cannot represent. A replacement
or supported fork requires the normal dependency and architecture review;
maintaining a private cryptographic fork is not an assumed team capability.

### Known vulnerabilities and applicability

Assess every known advisory or discovered security finding for the admitted
versions and uses, regardless of severity. Each finding must be either fixed
in the selected version or supported by a reviewed non-applicability argument:
the version is outside the affected range, the affected code is absent from
the applicable build/execution closure, or the exploit prerequisites are
enforceably impossible throughout the supported use being assessed.

For each non-applicability decision, record:

- advisory/finding ID, primary source and review date, exact component version,
  source/build identity and responsible owner;
- affected code and attack prerequisites, our import/call/input/privilege paths,
  and the specific boundary that prevents exploitation;
- all covered platforms, architecture, build tags, toolchain, configuration and
  runtime or build/test roles; any uncovered case remains unresolved;
- reproducible inspection and, where prevention depends on our behavior, a
  regression test of the enforcing boundary, with analysis limitations;
- invalidation conditions, including a dependency, import, feature, privilege,
  configuration, platform or advisory change.

A default-disabled feature, the lack of a public exploit, a low severity score,
an upstream issue marked closed or a clean scanner result is insufficient
evidence by itself. In particular, an attacker must not be able to enable or
reach a supposedly excluded path. Reachability analysis can miss dynamic
behavior; examine the affected mechanism and the scanner's limits.

An exploitable finding or unresolved applicability blocks integration and
release qualification of the affected use. A component without a credible
maintenance and security-fix path also fails admission, even if no advisory is
listed. Update, replace or remove the dependency and its affected functionality
through the responsible owner, then repeat the relevant qualification. A
non-applicability record is not permission to suppress or bypass a failing
repository check. Security response and owner-controlled adoption retain their
existing authority boundaries; this rule creates no remote shutdown power.

### Revalidation through delivery

Establish the dependency inventory and support evidence during the protection
workstream, and maintain them with ordinary changes. Before integration,
admission of a system-test candidate and release qualification, recheck the
exact closure against current advisory data and the applicable support
evidence. Record the scan time, database source/freshness, tool versions,
build configuration, inspected findings and evidence location. An unavailable
database or cache of unestablished freshness is not a passing fresh check.

A new advisory, loss of upstream support or change to a decision's assumptions
requires reassessment even when `go.mod` is unchanged. The responsible owner
tracks remediation through the existing issue ledger. Its priority reflects
exposure and impact; severity does not waive the acceptance condition. The
one-human-and-Codex team must be able to sustain this update and replacement
work; dependency selection cannot assume another maintenance or audit team.

The existing `make vuln` and `make check` provide the configured Go
reachability gate. The broader support, applicability, build/tool and artifact
reviews above are required evidence, not a claim that all are already automated
or complete. The successor
[dependency disposition map](privacy-anonymity-map.md#dependency-disposition-map)
covers retained pins, selected closed uses and their update/removal obligations.
Design selection does not replace actual candidate admission. [Testing policy](testing.md#dependency-security-evidence) owns how
their coverage is stated. Historical selection receipts and a successful scan
do not establish the absence of unknown vulnerabilities or future safety.

`govulncheck -json` is evidence input: inspect the complete findings and scanner
configuration, not only its exit code. A successful JSON-mode exit may include
findings. Source-with-tests, cross-target source inspection and binary scans
have different coverage; none substitutes for execution on a required platform.
Known imported-package or module-only findings still receive the scoped
applicability decision above. Preserve unsuccessful attempts and corrections.

## Selected successor design uses

[ADR-0078](../adr/0078-select-common-split-circuit-privacy.md) selects an
[architecture](../technical/common-privacy-architecture.md) built on maintained
Go TLS/HPKE and a publicly verifiable blind-token family. The TLS component
experiment is byte-cost evidence, not full protocol or security acceptance.
[ADR-0081](../adr/0081-select-closed-protected-service-contract.md) selects
CIRCL v1.6.5 ordinary blindrsa SHA384PSSDeterministic for the exact
[admission construction](../technical/private-admission.md), beyond its retained
HPKE use. The [R-152 assessment](../research/records/r-152-closed-scheme-contract.md)
records dated source/support/license/advisory review, exact three-package plus
standard-library closure, checksum verification, upstream tests and the composed
probe. Its owner is internal/route/credential. Do not import other CIRCL schemes
or serialize opaque blinding State. A changed use or upstream/support/advisory
fact invalidates that evidence; update or replace within this owner, never
maintain a private cryptographic fork.

Go 1.26.8 is the required successor build baseline on the supported 1.26 line.
`go.mod`, CI, active qualification containers and their declared prerequisites
pin 1.26.8; historical component evidence retains its actual 1.26.6 identity.
Every changed candidate still runs fresh source, test, tool, advisory and
artifact checks on that selected patch. Ubuntu/systemd package inventory and
built-artifact inspection also remain candidate evidence. No documentation
decision marks those checks already passed. Tools are installed only through
make tools-install.

### Go 1.26.8 successor-baseline admission evidence

Candidate evidence captured 2026-09-08 on Windows/amd64 with `CGO_ENABLED=1`,
no build tags, `GOENV=off`, `GOTOOLCHAIN=local`, and the exact
  `go1.26.8` binary selected by the build environment. Its SHA-256 was
  `21761eceb9302062c9623fb699f332c8c7fe000f15f70efe8da01a2cfbbc16b9`.

- `go mod verify` and `go mod tidy -diff` passed. The direct selected module
  identities were CIRCL `v1.6.5`
  (`h1:O64F26HEqNhznd/hrC5KZXVKYuKM2rx4deZDTc4ihQA=`), quic-go `v0.62.0`
  (`h1:ZHDjCk5OacATwGvs8PWE97CTvX7AqZiVoW7++ZOXTf8=`), OHTTP `v0.0.80`
  (`h1:LsDWRCU55vfI+mes1zuMGGKDZ8MsgbnlEr9IDd6jG9Y=`), and x/crypto
    `v0.56.0` (`h1:GUh5Ii4J5jtcseSMiRqr1jXCNHoxjeV9Fmekc2oLy6Y=`).
    Its retained OHTTP closure was twoway `v0.0.80`
    (`h1:pojOC5jRtsN04/ZwzZM7FIgt0qGj/rxefb388Eb1jKU=`) and bhttp `v0.0.80`
    (`h1:Zq0FiWIZCOqzrpMBzrEV7J0Jrc+20n0rPWVXpuKqwPQ=`). The complete closure
    was 181 `go list -m -json all` records; its exact `go.mod` SHA-256 was
    `a5e05aceb2ec1fa5afbfd7a9ff657cfca7ff04de43095fee692af7bb48872026`
    and its exact `go.sum` SHA-256 was
    `5167264d35ddb70683f5b7a3fbc6cffac8fa0ec6cb15d72c1d4747a7976627a2`.
- `make tools-install` rebuilt the pinned Staticcheck 2025.1.1,
  govulncheck v1.1.4, and deadcode v0.48.0 binaries with Go 1.26.8;
  `tools-check` now rejects a tool binary built by any other Go patch. Their
  Windows/amd64 SHA-256 values were respectively
  `e25257acb31418dac0f491c4909df49e8a359793442281407b9411b93f80bede`,
  `6713173da52bd0b8b1e5960a6baa0eae5311f9011f52a2db77b9738fb39f2b54`,
  and `0fb70a59d2139e4267cf0252f53439887663d0fcf78d55d66eeb87fc4c5f28e7`.
  - CIRCL's exact `blindsign/blindrsa` package passed its upstream RFC-9474
    vector suite under Go 1.26.8 with
    `go test github.com/cloudflare/circl/blindsign/blindrsa -run '^TestVectors$' -count=1`.
    The compressed upstream fixture
    `test_vectors_rfc9474.json.gz` had SHA-256
    `374ba388a53cd9017aecff92afd2174188c7d5c7e8f0947d8af466043f134ff7`.
    RFC-9474's fixture supplies RSA integer inputs rather than an SPKI or key
    ID. The credential-owner baseline therefore separately constructs and
    parses the selected 346-byte RFC-9578 RSA-PSS SPKI, requires its SHA-256
    key ID to survive parse/re-encode unchanged, and rejects generic RSA SPKI.
    It uses a fresh 32-byte nonce with only `SHA384PSSDeterministic`, rejects a
    malformed length through CIRCL, and independently verifies its finalized
    signature with Go's `crypto/rsa.VerifyPSS`. No opaque blinding State is
    serialized and no other CIRCL scheme is admitted.
- `govulncheck -json ./...` used database `https://vuln.go.dev`, last modified
  `2026-09-02T19:12:04Z`, scanner v1.1.4 and Go 1.26.8. It reported only
    module-level `GO-2026-5932`. `go list -deps` and `go list -deps -test`
    found zero `golang.org/x/crypto/openpgp` packages in all four selected
    closures: Windows/amd64 with CGO disabled (367 production, 498 test
    packages), Windows/amd64 with CGO enabled (367, 498), Linux/amd64 with CGO
    disabled (370, 504), and Linux/amd64 with CGO enabled (371, 505). This
    non-applicability holds only for those four closures; a new import, target,
    build tag, x/crypto version, scanner database update, or advisory change
    requires reassessment.

The trusted snapshot/Administration client and Node forwarding command imports
were reassessed on 2026-09-09 with the same Go 1.26.8, scanner v1.1.4,
x/crypto v0.56.0 and database modification time above. A fresh
`govulncheck -json ./...` again reported only the module-level GO-2026-5932
finding. Fresh `go list -deps ./...` and `go list -deps -test ./...` found no
openpgp package in any normal-build closure: Windows/amd64 CGO off/on each had
374 production and 510 test packages; Linux/amd64 CGO off had 377/517 and CGO
on 378/518. The scoped non-applicability argument therefore still holds for
these changed closures. This does not cover additional build tags or qualify
the installed worker artifact; those boundaries retain their own checks.

This is source/tool evidence only. The required Ubuntu package inventory,
installed-artifact inspection, and platform execution remain separate candidate
and qualification evidence; Windows or cross-target analysis cannot replace
them.

The successor confidential control channels can replace OHTTP transport only
with the explicit new grammar and migration. Existing OHTTP imports and their
full closure remain subject to review for as long as any maintained use exists.
Removal is an owner change with dependency/compatibility evidence, not an
automatic consequence of selecting the architecture.

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
| `github.com/quic-go/quic-go` | `v0.62.0` | MIT | maintained QUIC v1 Carrier Adapter and QUIC varint closure required by BHTTP |
| `github.com/cespare/xxhash/v2` | `v2.3.0` | MIT | tracing dependency closure |
| `go.opentelemetry.io/otel` | `v1.45.0` | Apache-2.0 | OHTTP tracing types |
| `go.opentelemetry.io/otel/trace` | `v1.45.0` | Apache-2.0 | OHTTP tracing Interface |
| `golang.org/x/crypto` | `v0.56.0` | BSD-3-Clause | selected cryptographic support closure |
| `golang.org/x/net` | `v0.58.0` | BSD-3-Clause | BHTTP HTTP support |
| `golang.org/x/sys` | `v0.47.0` | BSD-3-Clause | Windows owner-only DACL/locking and registry enforcement plus platform atomic replacement support |
| `golang.org/x/text` | `v0.41.0` | BSD-3-Clause | BHTTP normalization |

**Need and owner:** RFC 9458 is the accepted external-first Private Resolution
shape. `internal/naming/resolution` owns the Namespace OHTTP/CIRCL Adapter and
`internal/service/reachability` owns the separately authenticated Target
descriptor adapter; neither is a general HTTP proxy. A change repeats the
affected current-owner conformance, dependency and observer checks. R-047/R-026
retain the selection evidence; they are not instructions to reopen the original
research or a second current specification.

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
`github.com/quic-go/quic-go v0.62.0` for the maintained
`ardents-carrier-quic-v1` Adapter selected by ADR-0048. The module is pure Go,
MIT licensed, supports the repository toolchain, and publishes security
reporting. R-140 verifies its exact source, checksum, Go 1.26.6 compatibility,
local TLS peer binding, LegBinding, timeout, cleanup, and no-fallback behavior.
The external selective-blocking, loss/reorder, MTU-1280, NAT-rebinding,
resource, and separate-host matrix must be rerun for the exact frozen C0
artifact; R-094's `v0.61.0` results do not carry forward. `golang.org/x/net/quic` was
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
dependency graph repeats the affected current-owner checks. An exploitable or unresolved
vulnerability of any severity, loss of support, unacceptable license,
offline-build failure or broken role split stops admission under the
[acceptance rule](#maintenance-and-vulnerability-acceptance). The selection
does not authorize a first-party fork.

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
| `golang.org/x/crypto` | `v0.56.0` | BSD-3-Clause | raised cryptographic support closure |
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
unmaintained `x/crypto/openpgp` package. This is dated selection evidence;
[the 2026-09-07 source check](../research/records/r-150-common-protection-baseline.md#dependency-baseline-observation)
records its current bounded applicability result. Integration repeats the
applicable scans and support review. Any exploitable or unresolved advisory,
regardless of severity, fails the
[acceptance rule](#maintenance-and-vulnerability-acceptance).

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
`golang.org/x/crypto v0.56.0` (BSD-3-Clause). Other maintained cryptographic
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

**Need and owner:** `tests/e2e/service` owns the dependency. The Node process tests and the explicitly tagged installed Endpoint qualification also use this test-only terminal ceremony to invoke the real Custody command; no maintained runtime imports it. The supported
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
Unix PTY support. The selected use is confined to the terminal test harness;
no runtime owner needs it. Its pre-v1 API and absence of a dedicated published
security policy require review of its actual maintenance/fix path, not an
automatic supported or abandoned verdict. It is not accepted into a runtime
or shipped artifact. Its first-party test
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
| Go | 1.26.8 | compiler, formatter, tests, vet |
| Staticcheck | 2025.1.1 | additional correctness analysis |
| govulncheck | v1.1.4 | reachable Go vulnerability analysis |
| deadcode | v0.48.0 (`golang.org/x/tools`) | reachability analysis for reviewed production code and test-only code |

`make tools-install` is the only documented installation command. Normal build
and quick-check targets never install or upgrade tools implicitly.
