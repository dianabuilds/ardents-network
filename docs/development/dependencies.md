# Dependency register

## Go and HTTP security renewal

Reviewed 2026-10-09 after the fresh repository vulnerability gate found the
2026-10-08 advisories in Go 1.27.1 and x/net v0.59.0. Select the fixing Go
1.27.2 patch and x/net v0.60.0 before further integration. This supersedes the
current compiler/container pins below, while historical receipts retain their
actual compiler. It changes no domain authority, native platform admission,
Carrier set or persisted grammar. The earlier Installation process-cut receipts
are evidence of their original artifacts; renewed artifacts require fresh gates
and native evidence before candidate acceptance.

Primary sources accessed 2026-10-09: [Go release history](https://go.dev/doc/devel/release#go1.27.0),
[Go security policy](https://go.dev/doc/security/policy),
[x/net source](https://go.googlesource.com/net/+/refs/tags/v0.60.0),
and the reviewed Go database records
[GO-2026-6617](https://pkg.go.dev/vuln/GO-2026-6617),
[GO-2026-6613](https://pkg.go.dev/vuln/GO-2026-6613),
[GO-2026-6612](https://pkg.go.dev/vuln/GO-2026-6612),
[GO-2026-6611](https://pkg.go.dev/vuln/GO-2026-6611),
[GO-2026-6610](https://pkg.go.dev/vuln/GO-2026-6610),
[GO-2026-6608](https://pkg.go.dev/vuln/GO-2026-6608),
[GO-2026-6607](https://pkg.go.dev/vuln/GO-2026-6607),
[GO-2026-6605](https://pkg.go.dev/vuln/GO-2026-6605),
[GO-2026-6604](https://pkg.go.dev/vuln/GO-2026-6604), and
[GO-2026-6603](https://pkg.go.dev/vuln/GO-2026-6603).
Their affected ranges name Go 1.27.2 and, where applicable, x/net v0.60.0 as
fixing versions. These findings are fixed by version selection rather than
waived on severity or conservative call-graph traces. Other imported-package,
module-only and build/tool findings still require their own current assessment.

Go's supported 1.27 branch retains its BSD license and security fix path;
upstream supports the two most recent major branches. x/net retains its
Go-authors BSD license and the same Go security reporting/fix path. Existing
HTTP/TLS consumers keep ownership; engineering owns compiler and tool renewal.
The x/net fixing tag is
commit `18ece0ce30bc35fa81fe72028bf309bb3ff3f4a5`, with module sum
`h1:79p50tfZlm0J9YfoDsSi639qSXNGVwEzOPLCxM2FsYU=` and go.mod sum
`h1:2DA/G1UfVbCpQPeWTmMPGY7Cs2PkBkwu743bVX5PIVg=`. Its Go 1.26 minimum and
x/crypto v0.57.0, x/sys v0.48.0, x/term v0.46.0 and x/text v0.42.0 requirements
match the existing graph; no new runtime module or HTTP protocol is selected.
Public module downloads retain proxy and sum.golang.org authentication.

The official Go 1.27.2 Bookworm image index is
`sha256:5cf287a799e6b94384bad13d16b14904c531f51ba65792237e122ce42b392f61`;
its Linux amd64 manifest is
`sha256:55395706e9703db746cc507abfc4eb2aea75918f8a8024e4848e2cb81004f5ad`,
from docker-library/golang commit
`8380885ec449224702b989d8792d2e1470a46e89`. Exact compiler, OS packages,
module/cache integrity and artifact scans remain build evidence. CI, registered
profile prerequisites, diagnostics image and tool-build guards follow this
patch. Install/rebuild quality and diagnostic parser tools only through explicit
`make tools-install`; its `GOTOOLCHAIN=go1.27.2` bootstrap uses the authenticated
Go toolchain mechanism. Normal checks keep `GOTOOLCHAIN=local` and install nothing.
The renewed candidate must reproduce support/closure review, source and binary
scans, full regression and applicable native startup/recovery; selection alone
does not make those checks pass. The failed fresh gate remains retained.


### Staticcheck compiler-format compatibility candidate

Reviewed 2026-10-09 after the Go 1.27.2 full gate reproduced
[upstream issue 1832](https://github.com/dominikh/go-tools/issues/1832).
Rebuilding Staticcheck 2026.2.1 with the new compiler does not update its
x/tools decoder, which accepts export versions only through 4; Go 1.27.2 emits
version 5. The required check remains enabled and its failure is retained.

Select a bounded development-tool compatibility candidate from
[upstream PR 1834](https://github.com/dominikh/go-tools/pull/1834), accessed
2026-10-09: `github.com/stefanb/go-tools` revision
`a0a7f6a7b6af6eaa66e22ff3a5f78725e27decf2`, authenticated module version
`v0.7.0-0.dev.0.20261008215554-a0a7f6a7b6af`, sum
`h1:3rKlGcVFB2S4ULUqeV8rT12r7zFyZiWuuN8QTDTXOsg=` and go.mod sum
`h1:xG1Q4n+mg3bKZrO6RWFA9jFlkqplflmDRHGeuexiLDs=`. This is an open
upstream proposal, not a released or upstream-accepted Staticcheck version.
The ten-file delta from v0.8.1 includes the already upstream nilness fixes and
their regression inputs, devel version metadata, CI, x/tools v0.51.0,
x/mod v0.41.0, x/sync v0.23.0 and the explicit local call classification that
replaces removed internal linknames. The decoder supports version 5. The
remaining source and BSD license are unchanged; source comparison and actual
analyzer tests, scans and both-platform gates must qualify this candidate.

Engineering owns this temporary downstream tool build and its replacement.
Build it only through explicit `make tools-install`, with its generated build
module and all caches outside the repository. The product module is unchanged.
The normal guard requires the exact replacement module/version/sum, x/tools
version and fixing compiler; a released v0.8.1 binary cannot satisfy that guard.
Retain actual build metadata, source checksums, behavior and vulnerability
receipts. Replace the candidate with an authenticated upstream fixing release
once reviewed; if this source cannot be maintained or qualified, it remains an
unfinished gate prerequisite. No analyzer disablement, older vulnerable compiler,
severity waiver or claimed independent validation is selected.

### Errcheck compiler-format compatibility build

Reviewed 2026-10-09 after the rebuilt diagnostic image's version-only doctor
passed but its actual errcheck invocation refused even an imported `fmt` package.
The selected upstream v1.20.0 and current upstream main still select x/tools
v0.44.0, whose export decoder supports only versions through 4. The fixing
compiler emits version 5; preserve that failed operational receipt.

Keep the unchanged MIT-licensed [errcheck v1.20.0 source](https://github.com/kisielk/errcheck/tree/v1.20.0),
module sum `h1:9rwHBNKzd4wkDWcROy3DvFGNqEPlkxBg305rvk7HabI=` and go.mod sum
`h1:O+f80MKNwX8Oor2jwgpeQ9An7uJm+hRSgT+h22knRJU=`. Build its diagnostic
command in a separate external module selecting the already reviewed x/tools
v0.51.0, with its x/mod v0.41.0 and x/sync v0.23.0 graph. This changes tool
dependencies, not errcheck source, its error-checking policy or product go.mod.
It is an engineering-owned compatibility build, not an upstream errcheck release.
Only explicit `make tools-install DIAGNOSTIC_TOOLS=1` installs it. The same
closed analyzer builder installs Staticcheck and errcheck; callers choose one
of those exact reviewed definitions, never an arbitrary module or command.

Require exact source/compiler/dependency metadata, upstream behavior tests,
actual positive and refusal invocations, current vulnerability review and the
real rebuilt image/doctor before acceptance. Diagnostic-tool guard must reject
the earlier v0.44.0 build. Replace the override with a reviewed upstream fixing
release when available; do not disable the analyzer or downgrade the compiler.

The unchanged upstream test suite also fails under Go 1.27.2: one failure-message
format uses `%q` for integer exit status, and the analyzer matrix requests the
removed `gotypesalias=0` setting. Preserve that failure. An independently compared
external source copy changes only that message to `%v` and runs all existing
analyzer fixtures with the selected compiler's default alias implementation;
production source and assertions remain unchanged. Its full race suite and
module verification pass. This is adapted upstream test evidence, not a pass of
the unchanged suite or qualification of the unavailable legacy alias mode.
Actual CLI controls separately accept checked errors and detect an unchecked
error, blank error assignment and unchecked type assertion. The rebuilt image's
eight Go tool binaries have current no-finding scan receipts; source integrity,
continued maintenance and the temporary override's removal remain separate
obligations. See [Go's GODEBUG history](https://go.dev/doc/godebug#go-127).

## Installation retained system-manager connection

Reviewed 2026-10-08 for the native Installation owner. Select
`github.com/godbus/dbus/v5` v5.2.2 (BSD-2-Clause) as the private D-Bus client
mechanism for an original Endpoint unit reference. Short-lived `busctl` clients
cannot retain that reference across fixed-file replacement: an inactive unit
can be collected and its new file loaded before the explicit daemon reload.
This library implements the already selected system-bus mechanism; it selects
no new manager profile, authority, runtime, or public protocol.

Primary evidence, accessed 2026-10-08: [upstream release](https://github.com/godbus/dbus/releases/tag/v5.2.2),
[security policy](https://github.com/godbus/dbus/blob/master/SECURITY.md),
[license](https://github.com/godbus/dbus/blob/v5.2.2/LICENSE), and
[source](https://github.com/godbus/dbus/tree/v5.2.2). Upstream supports security
fixes only in its latest release; the module proxy's `latest` query returned
v5.2.2. Maintenance and private reporting are on a volunteer best-effort basis,
with no separate support term for older patches. Recheck the latest supported
release and advisories at integration and replace an unsupported client.

The downloaded tag identifies commit
`a8ac15ba63645f02ffd57f4b443203279ab40b30`, module sum
`h1:TUR3TgtSVDmjiXOgAAyaZbYmIeP3DPkld3jgKGV8mXQ=`, and go.mod sum
`h1:3AAv2+hPq5rdnr5txxxRwiGjPXamgoIHgz9FPBfOp3c=`. Its Go 1.20 minimum fits
the selected toolchain. Its module graph requires x/sys v0.27.0; Ardents keeps
its existing reviewed v0.48.0. External Linux/Windows amd64 candidate probes
with Go 1.27.1 found no known vulnerability in their scanned package closure.
These probes are dependency-review evidence, not product binary or integration
acceptance; the complete owning artifact and repository gates remain required.

Owner: `installation/systemd`, through one private fixed-address connection
and the original manager's unique bus identity. No shared singleton, environment
address discovery, cookie authentication, reconnect, exported object, or Unix
FD negotiation is selected. Root privileges permit the fixed RefUnit/UnrefUnit
operation; no credential, permission, plan or artifact bytes enter this client.
Installation retains generation/protection checks, Release proofs, original
caller and process quiescence. A unit reference grants no start or readiness.

Source review shows that client Close interrupts the connection but exposes no
reader join. The owning implementation must therefore bound authentication and
messages, own and join original transport I/O and cancellation, retain uncertain
reference acquisition through cleanup, and reject manager replacement and late
results. Closing a context alone is not accepted physical completion.

The pinned decoder allocates string and array storage from their inner declared
length before it has verified the remaining message bytes. Bounding only the
outer message is insufficient. The selected transport validates the complete
finite frame before exposing any bytes to that decoder: exact scalar header
variants, bounded scalar lengths within their original envelope, only empty or
single-string bodies, and no nested values or method calls. Native negative
controls include a small frame with a maximum inner string length, an array
variant and a declared oversized outer body. This is a scoped mitigation for
the selected private connection, not a claim that the upstream decoder is safe
for arbitrary peer messages or that vulnerability scanning proves it safe.

Alternatives: retaining a Python helper would introduce another product runtime;
a private D-Bus implementation would add protocol maintenance; accepting an
unexpected candidate configuration would change phase admission. The selected
client is confined behind systemd's native lifetime Interface. Removing it
requires replacing that physical mechanism and rechecking its lifecycle tests,
not changing Installation's journal, Release, or manager-profile contract.

## Successor finite inspection OpenTelemetry selection

Reviewed 2026-10-02 for issue 478. Select OpenTelemetry Go v1.47.0 API,
trace/metric APIs, SDK and SDK metric plus official OTLP HTTP trace/metric
exporters (Apache-2.0). Owner: cmd/ardents-next composition; Admission's ledger
imports only its public issuerprofile contract and the standard library.
Need: actual vendor-neutral traces and metrics with
one explicitly selected local Collector. No logs SDK, auto-instrumentation,
global propagation or remote export is selected. Resource attributes are fixed;
no host/environment resource detection. Instrument values have finite outcome
categories; permission/participant bytes never reach exporters.

Primary evidence, accessed 2026-10-02:
[v1.47.0 release](https://github.com/open-telemetry/opentelemetry-go/releases/tag/v1.47.0)
and [upstream security policy](https://github.com/open-telemetry/opentelemetry-go/security/policy),
plus [official Go exporters](https://opentelemetry.io/docs/languages/go/exporters/).
Upstream supports only its latest minor version. v1.47.0 adds a maintained
HTTP response-size option, fixes metric aggregation precision and a periodic
reader race, and retains the Go 1.25 minimum compatible with this toolchain.
No periodic reader is selected. Exact module checksums, source use, full transitive
closure and vulnerability checks remain required before candidate acceptance.

Alternative: a private telemetry encoding would create protocol maintenance;
backend-specific instrumentation would couple the product to storage. Selected
official SDK/exporters avoid both. Removal: command composition owns all OTel
imports and can replace the exporter without changing permission verification.
Export must be bounded and explicitly loopback-local; failure must not change
inspection outcome. No network/privacy qualification follows from selection.

Selected module inventory (Go module sums are pinned in go.sum):

| Module | Version | Use |
|---|---|---|
| `go.opentelemetry.io/otel` | v1.47.0 | Attribute API and API foundation |
| `go.opentelemetry.io/otel/metric` | v1.47.0 | Counter and histogram API |
| `go.opentelemetry.io/otel/trace` | v1.47.0 | Trace API closure |
| `go.opentelemetry.io/otel/log` | v1.47.0 | API foundation closure; no log SDK or log exporter selected |
| `go.opentelemetry.io/otel/sdk` | v1.47.0 | Explicit resource and trace provider |
| `go.opentelemetry.io/otel/sdk/metric` | v1.47.0 | Manual metric collection |
| `go.opentelemetry.io/otel/exporters/otlp/otlptrace` | v1.47.0 | Trace export foundation |
| `go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp` | v1.47.0 | Bounded HTTP trace export |
| `go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp` | v1.47.0 | Bounded HTTP metric export |
| `go.opentelemetry.io/proto/otlp` | v1.11.1 | OTLP schema and test packet decoding |
| `go.opentelemetry.io/auto/sdk` | v1.2.1 | API transitive closure; no instrumentation selected |
| `github.com/cenkalti/backoff/v5` | v5.0.3 | Exporter retry implementation; retry disabled |
| `github.com/cespare/xxhash/v2` | v2.3.0 | SDK attribute grouping |
| `github.com/go-logr/logr` | v1.4.4 | OTel diagnostic closure |
| `github.com/go-logr/stdr` | v1.2.2 | OTel diagnostic adapter closure |
| `github.com/google/uuid` | v1.6.0 | Resource SDK closure |
| `github.com/grpc-ecosystem/grpc-gateway/v2` | v2.31.0 | Generated OTLP schema closure |
| `google.golang.org/genproto/googleapis/rpc` | v0.0.0-20260928230214-8a89bd6388cc | Generated RPC messages |
| `google.golang.org/grpc` | v1.84.0 | Generated OTLP service closure; no gRPC transport selected |

This inventory does not replace source, license and advisory acceptance. The
HTTP exporters' generated schema includes gRPC dependencies even though this
command uses HTTP only; removing them requires changing the selected exporter,
not pretending the transitive dependency is absent.

The initial closure scan found package advisory GO-2026-6443 in gRPC 1.83.1.
Select the fixing 1.83.2 patch before acceptance rather than waive the finding:
[Go advisory](https://pkg.go.dev/vuln/GO-2026-6443) and
[upstream release](https://github.com/grpc/grpc-go/releases/tag/v1.83.2), accessed
2026-10-02. The module-only x/crypto OpenPGP finding GO-2026-5932 remains subject
to the existing scoped non-applicability record; it is not introduced by OTel.

Reassessment on 2026-10-03: the current Go advisory still reports gRPC v1.84.0
under GO-2026-6443. Do not infer clearance from the earlier patch selection.
`govulncheck -show verbose ./...` finds no reachable affected symbol; binary
scans of canonical Windows/Linux amd64 `ardents-next` do retain
`transport.http2Server.HandleStreams` and exit 3. Those failed scans are retained.
The [advisory](https://pkg.go.dev/vuln/GO-2026-6443), accessed 2026-10-03, requires
xDS server routing. The selected command has only outgoing OTLP HTTP exporters;
`go list -deps -test ./cmd/ardents-next` under Windows/Linux amd64, each with
CGO_ENABLED=0/1, includes neither gRPC xDS nor OpenPGP. No maintained first-party
source imports gRPC or constructs a gRPC server. The exact command import gate
and real HTTP exporter tests constrain this use. This is scoped non-applicability
for those four command closures, not a clean artifact scan or a general gRPC
safety claim. Reassess on an exporter, import, target, toolchain or module change.
`go mod verify` passed; the selected `internal/transport/http2_server.go` also
contains an early missing-authority rejection (SHA256
`6ff2da17e276ba22a782dffe467ec32b664895d12faccfbb5fc1ec40d59dcf6c`).
Reproduce with the closure command above, canonical `go build -trimpath
-buildvcs=false`, and `govulncheck -mode=binary -show verbose` on each artifact;
keep complete logs and artifact/source hashes outside Git.

Reassessment on 2026-10-09 preserves those earlier failed binary receipts.
The current [GO-2026-6443 record](https://pkg.go.dev/vuln/GO-2026-6443), modified
2026-10-06, closes its 1.84 development interval at
`1.84.0-dev.0.20260825144003-d5a41119e0e3`; selected release v1.84.0 is outside
every current affected interval. The new canonical Go1.27.2 participant and
tagged startup binaries still retain `http2Server.HandleStreams`, but current
verbose binary scans report no affected symbols or imported packages. Their
module-only OpenPGP finding remains separate: complete Linux/amd64
`text_worker_installed` dependency/test closures with CGO off/on have no OpenPGP
or gRPC xDS imports and no package/dependency errors. Normal-tag Windows/Linux
closures were separately rechecked. This establishes scoped advisory
applicability for those exact artifacts and closures, not general dependency
safety or installed qualification; changed source, tags, compiler, versions or
advisory ranges require renewed assessment.

Updated closure releases were inspected on 2026-10-02:
[OTLP schema v1.11.1](https://github.com/open-telemetry/opentelemetry-proto-go/releases/tag/v1.11.1),
[gRPC v1.84.0](https://github.com/grpc/grpc-go/releases/tag/v1.84.0),
[gateway v2.31.0](https://github.com/grpc-ecosystem/grpc-gateway/releases/tag/v2.31.0).
Downloaded immutable module source licenses identify OTel, auto/sdk, generated
Google messages, gRPC and stdr as Apache-2.0; backoff and xxhash as MIT; UUID and
grpc-gateway as BSD-3-Clause. Module integrity is checked with `go mod verify`;
version and source sums belong to go.mod/go.sum. Quiet stable stdr, UUID and
xxhash versions remain the latest available versions in `go list -m -u`; this
does not assert a separately guaranteed support term. The composition owner
rechecks upstream fixes and advisories at integration and replaces the exporter
if this closure becomes unmaintained. None of these modules receives permission
bytes, authority material or user-selected file paths through instrumentation.

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

ADR-0078/0081 and the current [Admission contract](../technical/private-admission.md)
select the closed token construction. CIRCL ordinary blindrsa uses only
SHA384PSSDeterministic at its exact registered holder/receiver/issuer owners;
opaque blinding State is never serialized and no other CIRCL scheme or private
cryptographic fork is admitted. Route capsule uses the selected standard-library
HPKE mechanism; it does not restore the removed OHTTP/Naming importer.

The current compiler is Go 1.27.2, with the fixing x/net v0.60.0 closure and
exact compatibility tool builds specified above. Sigstore v1.11.0 and
go-containerregistry v0.22.1 replace their earlier selections. `go.mod` and
`go.sum` pin the full present closure; table entries below identify the same
versions, never an alternate build baseline. Changed artifacts need fresh
support, source/tool/binary scan and applicable native/profile evidence.
Version selection and a successful scanner exit do not grant integration or
qualification.

Go 1.26.8/1.27.1 inventories, removed Naming/OHTTP call paths and their dated
scan results are retained in the [earlier register](../archive/development/dependencies-before-consolidation.md).
They cannot qualify today's compiler, imports, tools or artifacts. Current
scoped advisory evidence and all failed receipts remain with their exact source
identities above and at the unresolved finding boundary below.

## Current runtime dependencies

The module pins in this register describe the current root `go.mod`; they do
not themselves prove dependency admission or ongoing support. New use and any
version/import/configuration change retain the full acceptance rule above.
The removed OHTTP/Naming tree has no maintained accepting caller. OTel is a
separately reviewed finite command-composition use; it is not restored by that
historical dependency chain.

| Module | Present pin | Owned use / admission boundary |
|---|---|---|
| `github.com/cloudflare/circl` | `v1.6.5` | BSD-3-Clause; exact reviewed blindrsa token mechanisms, separate holder/receiver/issuer owners |
| `github.com/quic-go/quic-go` | `v0.63.0` | MIT; existing selected QUIC Carrier. Earlier v0.62.0 finding below remains unresolved; the newer pin is not proof of its closure |
| `github.com/godbus/dbus/v5` | `v5.2.2` | BSD-2-Clause; private original system-manager connection, reviewed above |
| `golang.org/x/crypto` | `v0.57.0` | BSD-3-Clause; selected support closure and purpose-scoped Custody Argon2 |
| `golang.org/x/sys` | `v0.48.0` | BSD-3-Clause; native platform observations, ACL/locking and atomic file mechanisms |
| `golang.org/x/term` | `v0.46.0` | BSD-3-Clause; separate interactive no-echo Custody adapter |
| `golang.org/x/net` | `v0.60.0` | BSD-3-Clause; reviewed network/HTTP support closure, fixing renewal and exact source identities above |
| `golang.org/x/text` | `v0.42.0` | BSD-3-Clause; transitive Go text support in the reviewed library graph; version required by the current x/net closure |

The present QUIC pin differs from the earlier v0.62.0 reviewed profile. This
consolidation records that fact and preserves the open exact-profile finding;
it selects no fix, new probe, fork or gate waiver. Profile-specific renewed
admission evidence remains required under the same continuing acceptance rule.
The separately reviewed OTel, Release, test-only PTY and development tool
closures are enumerated at their owners below/above.

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

**Maintenance and security review:** exact current source, import, tool and
artifact closures require their own dated support/advisory assessment.
Earlier OHTTP and Go/compiler scan snapshots remain provenance in the archive.
CIRCL's optimized assembly behind portable APIs does not select custom
cryptography. First-party cgo and `unsafe` remain forbidden by project policy.

**QUIC Carrier use:** ADR-0048 retains both selected Carriers. Current adapters
pin v0.63.0 in `go.mod`; older v0.61.0/v0.62.0 trials cannot qualify that artifact.
Reproduce the supported-platform, exact TLS/ALPN/configuration, hostile-network,
MTU, cancellation, accounting and resource evidence required by the current
protocol and test profiles. The open local finding below is retained with its
original observations, source and admission blocker. TCP-only cannot satisfy
the selected second Carrier boundary; a first-party QUIC implementation is
unselected protocol/cryptographic work.

### Existing QUIC finding: return-path reply budget

**Finding ID:** `ARDENTS-QUIC-2026-09-21-RETURN-PATH-REPLY-BUDGET`.
This is a locally recorded component observation, not a CVE, GHSA, upstream
report, proof of novelty, or demonstrated attack against an Ardents deployment.
It concerns `quic-go v0.62.0`, with module checksum
`h1:ZHDjCk5OacATwGvs8PWE97CTvX7AqZiVoW7++ZOXTf8=` and `go.mod` checksum
`h1:RAro2j2yN9a9EiPACLHT9IB2NXCvGQmmo/alT0yYI0w=`. The upstream source
identity is immutable release tag `v0.62.0`, annotated-tag object
`738877626f361538cf07bc4c40cef79483a3ddbf`, peeled commit
`793f74d8e03368c5aded128af6f48d21dbb47f73`; the release and source were
rechecked on 2026-09-23.

**Observed evidence and limit:** retained Windows and Linux loopback,
user-space NAT-mapping probes on Go 1.26.8 report a 32-byte inbound event and
a 1200-byte path-probe reply while the old mapping is retained. Their original
nonzero exits remain evidence rather than being converted into passes. The
evidence establishes that result for the probe fixture and source inspection;
it does not establish kernel-NAT behavior, source-address spoofing,
third-party reflection, sustained rate or lifetime, a CVE-class exploit, or
behavior in the exact Ardents TLS/ALPN/listener profile. In particular, the
actual Linux OOB send path and the full admitted profile combination were not
measured.

The local evidence is retained outside Git at
`C:\Users\vitek\AppData\Local\Temp\ardents-handover-model-11a26ed48d104b1a826aa64fd2e1ea9c`.
It is identified, rather than replaced by an inference, by:

- `quic-applicability-note.md` SHA-256
  `9890cd1a2fc3eca600e557d216ea002c60aa8aaefae7d0e63641d49feca94cf7`;
- `finding-disposition-and-g10-lanes.md` SHA-256
  `6226691653199302de7551d0b5f7ad413b6d3928fa1168855f0766436753bbd6`;
- `quic-nat-probe/inputs.json` SHA-256
  `b04da0ed3675d31184e2564a8ae37981dbd37264add8bea2e7f9204309bdc42c`,
  with retained Windows result SHA-256
  `0a2539282e89204a01735aa3da7fdba96738080ba6e153677abcf9877b73b97b`
  and Linux old-map-dropped result SHA-256
  `0eeb48547b21cbcd91f38a89f0d5884fb0976c09c29d4063e659513a57cfa3f0`;
- `quic-egress-probe/inputs.json` SHA-256
  `892ceb4fc280634bc40f81b70bb6d8647ca3b21dd7fd17e0066aea13b04c592d`.

These hashes and the pinned module/source identities make the repository
record independently inspectable; the temporary directory is not the only
basis for this disposition. Its raw captures are provenance, not a current
qualification result.

**Admitted-use disposition at `dev@6134408ff561db83ff4f98f2aad2f1ef11198701`:**
the affected server use remains unresolved. `ListenClosedSharedCarrier` is
selected by the forwarding, resolution, introduction, join, and issuer Node
listeners; `ListenClosedRoleCarrier` is selected by the credential token
listener. When either selects QUIC, it is the server-side path covered by this
finding's source inspection. The live server callers and the three route
adapter files were checked at that commit; their SHA-256 identities are
recorded in the issue evidence.

`OpenClosedNodeCarrier` and `OpenClosedRoleCarrier` are outgoing-only uses.
For this finding's claimed reflector role, source inspection supports a
bounded non-applicability argument: the observed new-remote-address handling
is server-side. It does not qualify client receive accounting, general QUIC
safety, or a future client migration feature. TCP/TLS selections do not
execute the QUIC path. QUIC fixture tests exercise the adapters but neither
turn the server disposition into qualified use nor replace the missing
profile-specific measurement. The removed `ListenNodeCarrier` is not a current
server caller.

**Current disposition:** the unresolved applicability/qualification of the
affected QUIC server use blocks its admission under the maintenance and
vulnerability acceptance rule. Removing a historical caller, a quiet advisory
scan, or passing adapter tests does not close that blocker. No fix, replacement,
private fork, upstream report, or network experiment is selected by this
record.

**Upstream/advisory review (accessed 2026-09-23):** the official release and
GitHub advisory records were checked. The published identifiers
`GHSA-vvgj-x9jq-8cj9`/`CVE-2026-40898`, `GHSA-g754-hx8w-x2g6`/`CVE-2025-64702`,
`GHSA-47m2-4cr7-mhcw`/`CVE-2025-59530`, and
`GHSA-j972-j939-p2v3`/`CVE-2025-29785` describe other defects and their
published affected ranges end before v0.62.0. They do not identify this local
finding. This review is not an assertion that no other advisory exists or that
the component is free of vulnerabilities.

The disposition must be revisited if the module, checksum, source, Go version,
QUIC configuration, server caller, path-migration behavior, platform/backend,
or an upstream/advisory record changes. Closure requires a preserved,
exact-artifact profile measurement that covers each admitted QUIC server
listener on its supported platform and TLS/ALPN/configuration, demonstrates
the selected mitigation or enforceable non-applicability boundary, and records
the relevant bounded-rate/lifetime and cleanup result. A subsequent dependency
or architecture decision must then satisfy this document's normal review and
qualification requirements.

**Offline supply:** an explicit preparation step runs `go mod download` and
`go mod verify` outside the repository, then supplies a temporary vendor context
to a Docker build with `--network=none`. No vendor tree, module cache, generated
dependency, or Gateway key is committed.

**Removal plan:** a dependency leaves after its last maintained caller and
registered-use obligation are retired or replaced. QUIC is still required by
the selected closed Carrier set; removed private-resolution callers do not
remove that obligation. A changed version or
dependency graph repeats the affected current-owner checks. An exploitable or unresolved
vulnerability of any severity, loss of support, unacceptable license,
offline-build failure or broken role split stops admission under the
[acceptance rule](#maintenance-and-vulnerability-acceptance). The selection
does not authorize a first-party fork.

## Current release-verifier closure

Status: **current maintained dependency.** R-049 selected the following exact
reviewed module closure. Maintained `internal/release` and separately isolated
`internal/successor/release` verification owners consume the reviewed library
mechanisms under their own behavior tests, callers and exact package-map imports.

| Module | Present pin | License | Purpose |
|---|---:|---|---|
| `github.com/theupdateframework/go-tuf/v2` | `v2.4.2`, commit `f5edbde31e5507f46db2069402dc38903fe6d9d4` | Apache-2.0 | TUF metadata and trusted-metadata workflow |
| `github.com/cenkalti/backoff/v5` | `v5.0.3` | MIT | transitive go-tuf module dependency; absent from the maintained package import path |
| `github.com/google/go-containerregistry` | `v0.22.1` | Apache-2.0 | signature/key conversion closure |
| `github.com/opencontainers/go-digest` | `v1.0.0` | Apache-2.0 | digest conversion closure |
| `github.com/secure-systems-lab/go-securesystemslib` | `v0.11.1` | MIT | maintained signing-verification support used by go-tuf metadata |
| `github.com/sigstore/protobuf-specs` | `v0.5.2` | Apache-2.0 | signature verification type closure |
| `github.com/sigstore/sigstore` | `v1.11.0` | Apache-2.0 | public-key verification adapter used by maintained Release verification and its behavior fixtures |
| `github.com/youmark/pkcs8` | `v0.0.0-20240726163527-a2c0da244d78` | MIT | PKCS#8 parsing closure required by Sigstore |
| `golang.org/x/crypto` | `v0.57.0` | BSD-3-Clause | raised cryptographic support closure |
| `golang.org/x/sys` | `v0.48.0` | BSD-3-Clause | raised platform support closure; already selected elsewhere |
| `golang.org/x/term` | `v0.46.0` | BSD-3-Clause | no-echo terminal secret input for `cmd/ardents-custody`; also present in the reviewed sigstore closure |
| `google.golang.org/genproto/googleapis/api` | `v0.0.0-20260928230214-8a89bd6388cc` | Apache-2.0 | protobuf API type closure |
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
[the 2026-09-07 source check](../archive/research/records/r-150-common-protection-baseline.md#dependency-baseline-observation)
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
`golang.org/x/crypto v0.57.0` (BSD-3-Clause). Other maintained cryptographic
owners select the same module version; integration must produce one shared
exact root-module version, never parallel copies.

`internal/custody` is the only maintained Argon2 caller. It uses only
`argon2.IDKey` with the fixed v1 profile and passes the derived
32-byte key to the selected Go standard-library `crypto/aes` and
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
| Go | 1.27.2 | compiler, formatter, tests, vet |
| Staticcheck | v0.8.1 with exact PR1834 compatibility source above | additional correctness analysis; temporary reviewed build candidate |
| govulncheck | v1.8.0 | reachable Go vulnerability analysis |
| deadcode | v0.50.0 (`golang.org/x/tools`) | reachability analysis for reviewed production code and test-only code |

`make tools-install` is the only documented installation command. Normal build
and quick-check targets never install or upgrade tools implicitly.

## Local diagnostic tools

The engineering diagnostics owner is [local-diagnostics.md](local-diagnostics.md).
The image uses Go 1.27.2 and the digest-pinned official Debian Bookworm Go image
`sha256:5cf287a799e6b94384bad13d16b14904c531f51ba65792237e122ce42b392f61`.
The existing accepted quality tools retain their reviewed pins; normal gates do
not install additional tools. Explicit `make tools-install DIAGNOSTIC_TOOLS=1`
adds **Delve v1.27.2**, module `github.com/go-delve/delve/cmd/dlv`, MIT, solely
inside the local development environment. It is not a product import or shipped
artifact. Standard Go runtime profiles/traces and a stdlib-only private Unix
Interface avoid a new runtime SDK, telemetry exporter or backend dependency.

Primary review accessed 2026-09-30:
[Delve immutable v1.27.2 release](https://github.com/go-delve/delve/releases/tag/v1.27.2),
[upstream changelog](https://github.com/go-delve/delve/blob/master/CHANGELOG.md),
[MIT license](https://github.com/go-delve/delve/blob/v1.27.2/LICENSE),
[Go diagnostics](https://go.dev/doc/diagnostics), and
[Docker service limits](https://docs.docker.com/reference/compose-file/services/).
The upstream release and follow-up activity provide a maintained update path;
release notes/changelog cover Go 1.26 module-data support and subsequent debugger
fixes. This is not a claim that an old patch has independent long-term support.
Reassess new release fixes and advisories before renewing the image. Delve can
read/write target memory and ptrace; run only in the explicitly selected isolated
profile without host PID/network, credentials or production authority roots.
Debugger TCP serving is not selected. Go/Delve telemetry is disabled in the image.

Debian-installed strace, iproute2 (ss/tc), tcpdump, procps and Graphviz provide
syscalls, namespace socket/fault/packet evidence and local profile rendering.
Use Debian's signed security repositories; actual versions and full native
package closure are captured by `dpkg-query -W` inside the image inventory.
That build is identified by its actual image ID, because package updates make
a rebuild a distinct environment. Tool module closure/compiler is captured
with `go version -m`; module integrity is checked during build. Before admission,
run the relevant behavior/race tests and vulnerability inspection of the exact
tool artifact; retain findings and any scoped non-applicability evidence. Image
presence or a clean scanner alone does not establish continued upstream support.

The Product Owner and Codex maintain this local tool environment. Update via an
explicit reviewed image rebuild and the installation target. Remove Delve when
interactive source/memory debugging is no longer needed; remove native tools
when their recipes are retired. Prefer standard Go pprof/trace/objdump for
ordinary diagnosis; GDB/eBPF/kernel instrumentation is not selected merely
because it could be useful. No new dependency enters the root product module.

The same explicit diagnostics installation adds **errcheck v1.20.0**, module
`github.com/kisielk/errcheck`, MIT, as an advisory unchecked-error/type-assertion
analyzer. Primary [release notes](https://github.com/kisielk/errcheck/releases/tag/v1.20.0)
and [source/license](https://github.com/kisielk/errcheck/tree/v1.20.0), accessed
2026-09-30, identify its maintained update path and Go >=1.25/tooling update.
Its actual binary/compiler/module closure is inventoried and inspected with
Delve. The static diagnostic operation retains findings and a failing exit;
findings require owner/call-path review and are not automatically product bugs.
It neither replaces official gates nor becomes a runtime import. Remove it if
Staticcheck/vet cover its needed checks. Explicit rebuild/update ownership is
the same local tool owner as Delve.

The first inspected Delve 1.27.0 candidate was rejected after binary analysis
reported GO-2026-6238 in cilium/ebpf 0.11.0. The selected upstream Delve 1.27.2
source `d116177dd925e085ba5dd340f8c644f4a1501a3b` raises that dependency to the
fixed 0.22.0; module integrity is `h1:dhcjFjiVoQI53uwsG7EqYiMhdvFuywsBZx4KDOK+sYE=`.
See the [upstream fixing release](https://github.com/go-delve/delve/releases/tag/v1.27.2)
and [Go advisory](https://pkg.go.dev/vuln/GO-2026-6238), accessed 2026-09-30.
Keep the rejected scan with the corrected candidate evidence; no exemption is
used for its presence in the earlier artifact.

Exact errcheck 1.20.0 binary review reported module-only GO-2026-6180 and
GO-2026-6179 for x/mod 0.35.0, with no affected imported packages or symbols.
The vulnerable packages are x/mod/sumdb and x/mod/sumdb/tlog; inspect the exact
source closure with `go list -deps .` inside errcheck's pinned module, and the
binary with `go tool nm /go/bin/errcheck`. Neither package participates in this
analyzer executable. That older binary's subprocess used Go 1.26.8; this is historical evidence,
not admission of the current compatibility build. The current exact compiler
and analyzer dependency closure are specified above and require renewed scans. This scoped
non-applicability is invalidated by a changed tool version, imported closure,
Go toolchain, or invocation that admits a sumdb implementation. Retain verbose
binary scan and closure evidence; module-only findings are not erased by a
successful scanner exit.

The explicit Linux tool installation also installs PowerShell 7.6.6 (MIT)
from the [upstream immutable release](https://github.com/PowerShell/PowerShell/releases/tag/v7.6.6),
accessed 2026-09-30, using SHA256
`ddbc4a2d113bbd46d283cfedcbcd117a70caefd7673f41f2b4e0000badf103bc`.
It supplies the maintained architecture script tests, not a product dependency.
The release uses .NET 10.0.401; Debian native dependencies and Python 3 come
from signed security repositories and are included in the environment inventory.
Rebuild on upstream/security fixes; validate the actual required scripts.

### Prebuilt standard Go diagnostic parsers

The local diagnostics owner uses the selected Go 1.27.2 source packages
`cmd/pprof` and `cmd/trace`, with the Go distribution license, to parse explicitly
selected private profiles offline. Explicit `make tools-install` builds them into
`GOBIN` or `GOPATH/bin`; `DIAGNOSTIC_PARSERS_ONLY=1` installs only this pair. They
are not new module imports, product binaries or a separate upstream selection.
Their source/support/security owner is the existing pinned Go toolchain above;
rebuild and revalidate both on each toolchain change. The image/doctor inventory
records actual compiler/build metadata. Remove this installation when private
bundle format validation is retired. No symbolization, selected executable,
remote URL, HTTP parser service or artifact execution is enabled by this use.
Per-file deadline, process-group cancellation and discarded-output caps belong
to the local diagnostics owner. Native/parser prerequisite failure blocks that
profile instead of triggering implicit compilation or a passing skip.

Successor `admission/issuance` consumes the same reviewed CIRCL v1.6.5 Blind RSA signer
through its sole additional import blindsign/blindrsa. Admission confirmations
gate private use; journal reopen rechecks stored responses with that signer.
No scheme, version, opaque blinding-state serialization or private fork changes.
Acceptance repeats client finalize/stdlib PSS verification and current gates.

Successor Nodeidentity and signed issuer-profile provisioning add no third-party
runtime dependencies. Ed25519 and PKCS#8 parsing use the selected Go standard
library; existing CIRCL token signing version and scheme remain unchanged.
