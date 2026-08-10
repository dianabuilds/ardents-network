# Carrier Lab preflight

The preflight is the first maintained vertical project slice. It prepares and
verifies the controlled environment required by R-013 before any routing
experiment can run. It does not implement routing or a network protocol.

It answers the environment-readiness part of
[R-004](../research/records/r-004-routing-rendezvous-families.md) using the
frozen inputs from
[R-013](../research/records/r-013-carrier-lab-technology-candidates.md) and the
Go foundation selected by
[R-014](../research/records/r-014-language-runtime-candidates.md).

## Prerequisites

- a host that can run Linux `amd64` containers (Windows, macOS, or Linux);
- Bash, Git, Docker client and daemon, and a local Go 1.26 toolchain used only
  to launch maintained bootstrap orchestration; on Windows, use Git Bash for
  the shell entrypoint;
- the already downloaded `go1.26.5.linux-amd64.tar.gz` archive with SHA-256
  `5c2c3b16caefa1d968a94c1daca04a7ca301a496d9b086e17ad77bb81393f053`;
- the already present Ubuntu image manifest
  `sha256:7b202b0e2e0028c6250f5fcf41d04df492d145a1654c6995a6553f0c1f6f1960`.

Native-candidate tooling smoke additionally requires the 12 exact Ubuntu
`.deb` files named in `carrier-lab/tools.lock`, together in one external
directory containing no other entry, and the locked Ubuntu base already
present in the local image store. Preparation may use the network as an
explicit separate operation; verification, build, and run never do.

Missing inputs are a failed preflight. The thin script sets offline Go launcher
controls and invokes `internal/preflight`; it does not own Docker lifecycle,
cleanup, or evidence policy. The Go Module does not pull an image, download Go,
install a tool, accept a mutable image tag, or use the host-built launcher as
success evidence. The candidate and verifier are rebuilt with the supplied
pinned archive before a passing verdict is possible.

## Reproducible Carrier Lab image

### Development and qualification workflow

Ordinary development is container-free: `make quick-check`, `make check`, the
pre-commit hook, and the `quality` CI workflow compile and test Go without
building or running a Docker image. A code test never triggers Carrier Lab
image construction.

Docker is an explicit qualification boundary after the source tree is frozen:

1. finish source, architecture, documentation, and Go tests;
2. run `make check` and make no further source edits;
3. verify external inputs, build each required final image once with
   `--no-cache --pull=false --network=none`;
4. pass immutable image IDs to the required smoke commands;
5. retain only bounded evidence and update its durable record.

The smoke controllers always use Compose with `--no-build --pull never`; they
cannot rebuild an image. A later edit to maintained code, tests, Docker/Compose,
Make, hooks, or CI inputs invalidates the embedded qualification-source receipt,
but does not trigger an automatic rebuild. Documentation-only evidence updates
do not. Qualification waits until the next source freeze. `--no-cache` belongs
to this final evidence boundary, not to the edit/test loop.

`carrier-lab/Dockerfile` is one multi-target build definition. Its shared
builder and the `application` and `tooling` runtime targets all use the exact
R-013 Ubuntu digest. The Go archive stays outside the repository
and ordinary build context. BuildKit receives its containing directory as the
named `go_archive` file context; the Dockerfile mounts only the expected archive
read-only and verifies its SHA-256 before extraction. Build and module caches
are tmpfs mounts, and every build `RUN` has `--network=none` with
`GOTOOLCHAIN=local`, `GOPROXY=off`, `GOSUMDB=off`, and `CGO_ENABLED=0`.

The `bootstrap` command computes the qualification-source digest and passes it
as the required `CARRIER_LAB_SOURCE_SHA256` build argument. A manual build that
omits that receipt fails closed; ordinary development does not build this image.

The final stage is the pinned Ubuntu runtime plus
`/usr/local/bin/carrier-lab`; it contains no source tree, Go toolchain, or build
cache and defaults to numeric user `65532:65532`. The preflight independently
builds the same binary twice through the Dockerfile and pinned verifier path,
requires matching SHA-256 values, and records `image.base_image_id`,
`image.carrier_lab_image_id`, and `binary.sha256` in the canonical manifest.
The image is disposable Carrier Lab input, not a release artifact.

## Two-role isolation smoke

The `isolation` profile in `carrier-lab/compose.yaml` defines exactly two
disposable roles, `alpha` and `beta`, on one Compose-internal `adjacency`
network. There is no default,
external, host, or published-port path. Each role runs as numeric user
`65532:65532` with a read-only root filesystem, all capabilities dropped,
`no-new-privileges`, bounded CPU, memory, PIDs, and tmpfs, one read-only role
configuration, and one separate writable evidence bind. Neither role receives
the repository, Docker socket, another role's evidence, or topology data beyond
its one allowed peer.

Both Compose profiles receive only one immutable image ID, one controller-owned
run root, and one run ID. Mount paths are fixed children of that root inside the
Compose definition; callers do not pass a separate host path for every role.

The deep `internal/harness` Module behind `compose-smoke` is the sole lifecycle
controller. It accepts an
immutable local image ID, creates the fixed data-only role configurations,
starts the contour without building or pulling, waits for both readiness
records, inspects the actual containers and internal network, and requires each
role to complete two authenticated run-local identity exchanges with only its
declared neighbor. It retains one bounded `compose-smoke.json` summary and
removes the transient role evidence, run directory, containers, network, and
volumes. Cleanup is repeated and checked after both success and failure.

`--fault controller-stop` interrupts the controller immediately after both
roles are ready. It must exit non-zero while still recording
`cleanup_complete=true` and leaving no project resources. This is the required
stop-path check, not a successful smoke result.

Only a controller running natively on Ubuntu 26.04 `x86-64` records
`classification: official`. Windows, macOS, WSL, Docker Desktop, another Linux
distribution, and another Ubuntu release always record
`classification: development`. The Product Owner accepted the verified Docker
Desktop development result for the iteration-3 isolation gate on 2026-08-09;
this does not reclassify it as official evidence or change later R-013 gates.
The contour proves only this exact harness isolation
and cleanup behavior. It is not a selected topology, runtime, transport,
routing system, protocol, privacy result, anonymity result, decentralization
result, or production claim.

## Direct TLS measurement control

`direct-control` is the R-013 measurement-floor control, not a Route and never
a fallback. The `internal/directcontrol` Module creates one ephemeral Ed25519
Target root, one active
Ed25519 Instance leaf, and one inactive wrong-leaf fixture under the same root.
It launches the same `carrier-lab` binary as separate User and Service tracer
processes. The User accepts only the exact active leaf under the preconfigured
Target root. TLS is fixed to 1.3 with X25519, a constant
`carrier.invalid` SNI, constant `carrier-lab-direct/1` ALPN, no session cache,
no tickets, no resumption, and no early data path.

The positive case exchanges a fresh 32-byte canary and a 64 KiB deterministic
incompressible payload as one ordered opaque stream, and exposes bytes to the
result only after complete verification. Two mandatory negatives run
sequentially: a wrong Instance leaf under the valid Target root, and a separate
fault-proxy that changes one encrypted TLS record without receiving Target,
Instance, canary, payload, naming, discovery, or topology data. Both cases must
produce an explicit failure with `application_bytes_verified=false`.

The retained `direct-control.json` records the binary SHA-256, exact process
exit outcomes, TLS properties, per-process elapsed/heap/goroutine observations,
case verdicts, and final cleanup. Private keys, role configs, canary, payload,
and transient role evidence remain in the owned run directory and are removed.
The summary always states `direct_relationship_disclosed=true` and
`route_fallback=false`. Passing it provides no Route, privacy, anonymity,
decentralization, availability, or production claim.

## Native candidate tooling supply and smoke

R-025 supplies the laboratory tools without changing the unprivileged
Application image. `carrier-lab/tools.lock` names every external package and
runtime binary by version and SHA-256. The `tooling` target in the shared
`carrier-lab/Dockerfile` verifies the exact directory contents, package control
identity, architecture, package hashes, and final `tc`/`tcpdump`/libpcap
hashes, then uses `dpkg-deb --extract`; it never runs APT, `dpkg --install`, or
maintainer scripts.

The `tooling` profile in `carrier-lab/compose.yaml` has two unprivileged
synthetic tracer roles, two namespace-sharing shaping sidecars, and one capture
sidecar on one internal network. Tracers have no effective capabilities. Each
shaper runs as a
laboratory-only root user with all capabilities dropped and only `NET_ADMIN`
added; capture uses the same contour with only `NET_RAW`. Root is required here
because Docker clears the added effective capability when the fixed numeric
non-root user execs the tool. All roles retain a read-only root filesystem,
`no-new-privileges`, no published port, no external network, bounded resources,
and only their dedicated external mounts.

The fixed smoke applies `delay 40ms rate 100mbit limit 1000` to each endpoint
egress, producing the R-013 80 ms round-trip floor. tcpdump captures exactly 12
packets on alpha's `eth0` and TCP port 37002. It reads the pcap back itself,
checks the run-local synthetic marker, records the pcap hash and size, and
deletes the raw file. The canonical summary can become `passed` only after
tool identity, the complete `delay`/`rate`/`limit` contract on both qdiscs,
capture, the exact five-container Compose project, the exact two tracer
attachments, both reciprocal `ObservedPeer` values, raw deletion, and repeated
resource cleanup all pass. `--fault capture-start` must fail while still
proving cleanup.

An offline build after explicit input preparation is:

```sh
VERIFY_OUTPUT="$(go run ./cmd/carrier-lab tooling-verify \
  --lock "$PWD/carrier-lab/tools.lock" \
  --bundle /external/absolute/tool-bundle \
  --repository-root "$PWD")"
printf '%s\n' "$VERIFY_OUTPUT"
SOURCE_SHA256="$(printf '%s\n' "$VERIFY_OUTPUT" | sed -n 's/^Qualification source SHA-256: //p')"
test "${#SOURCE_SHA256}" -eq 64
docker build --no-cache --pull=false --network=none \
  --build-arg CARRIER_LAB_SOURCE_SHA256="$SOURCE_SHA256" \
  --build-context go_archive=/external/absolute/go-archive-directory \
  --build-context tool_bundle=/external/absolute/tool-bundle \
  --file carrier-lab/Dockerfile \
  --target tooling \
  --iidfile /external/absolute/tooling-image.id .
docker build --pull=false --network=none \
  --build-arg CARRIER_LAB_SOURCE_SHA256="$SOURCE_SHA256" \
  --build-context go_archive=/external/absolute/go-archive-directory \
  --build-context tool_bundle=/external/absolute/tool-bundle \
  --file carrier-lab/Dockerfile \
  --target application \
  --iidfile /external/absolute/application-image.id .
```

The source is frozen before these two builds. The first build verifies and
extracts the tool closure and builds the one Carrier Lab binary. The second
target reuses that content-addressed build stage; it does not repeat package
preparation. Tests and scenario attempts reuse the two immutable IDs. Rebuild
only after a qualification-source file changes, never once per test.

`tooling-verify` is the mandatory pre-build gate: as well as checking the exact
external bundle, it resolves the base reference from the repository lock and
requires `docker image inspect` to find that repository digest locally. A
missing base therefore fails before Docker receives the build request; there
is no implicit pull or alternate-base fallback in the documented flow. The
verifier prints the qualification digest; the explicit build argument embeds
that digest, and smoke independently recomputes it.

`tooling-smoke` takes the resulting immutable `sha256:` image ID and the same
derived session/run path contract as `compose-smoke`. Before Compose starts,
the controller inspects the image's locked-base and target labels, hashes its
embedded lock and executable with network disabled, and compares the embedded
source-snapshot digest with the current build input. A syntactically valid but
unbound `sha256:` image is rejected. Retained `tooling-manifest.json` contains
no host path or address. Its build receipt binds the final image ID, exact base
reference, lock digest, source digest, and executable digest; the remainder
records tool identities, effective capability masks, bounded qdisc state, pcap
hash/size, exact peer/isolation assertions, classification, status, and
cleanup. The separate `tooling-verdict.json` binds that manifest by SHA-256;
the controller re-reads and verifies the binding before it returns. Later
mutation is thus detectable and cannot preserve the canonical verdict
identity.

### Native C-5/C2 development smoke

The `native` profile uses the same Compose file and the same two image targets;
there is no additional Dockerfile, Compose overlay, package cache, or generated
container source in Git. Eleven unprivileged application roles form 11 exact
adjacent internal networks. Eleven `NET_ADMIN` sidecars apply the fixed qdisc,
and ten `NET_RAW` sidecars produce 11 link captures (User Interior owns the two
distinct outgoing links). Each application role receives the same read-only
lifecycle control mount. No role receives a host, external, or Docker-control
network.

Run one development smoke from a source-frozen pair as follows:

```sh
RUN_ID="native-$(date -u +%Y%m%dT%H%M%SZ)"
TEMP_ROOT="${TMPDIR:-/tmp}"
SESSION_ROOT="$TEMP_ROOT/ardents-carrier-lab-preflight-session.$RUN_ID"
mkdir -m 700 "$SESSION_ROOT"
go run ./cmd/carrier-lab native-run \
  --repository-root "$PWD" \
  --session-root "$SESSION_ROOT" \
  --temp-root "$TEMP_ROOT" \
  --run-id "$RUN_ID" \
  --application-image "$(cat /external/absolute/application-image.id)" \
  --tool-image "$(cat /external/absolute/tooling-image.id)"
```

Before Compose starts, `native-run` verifies both image IDs, target/base labels,
the current qualification-source digest, the identical embedded binary digest,
and the tooling image's exact embedded lock. It then requires all roles and
sidecars to become ready, inspects the topology and effective capability
contract, runs the separate C2 Introduction and C-5 data paths, authenticates
the exact ephemeral Instance, verifies 64 KiB of protected Application bytes,
checks all 11 captures for forbidden cleartext markers, collects bounded role
and tool summaries, removes raw captures, and removes all owned Compose and run
state. Failure logs are retained in bounded form before cleanup.

The retained `native-run.json`, `native-roles/*.json`, and
`native-tools/*.json` are development evidence outside the repository. A pass
requires the image receipt, exact topology, bounded privileges, complete qdisc
state, all link captures, role views, endpoint authentication, ordered bytes,
marker absence, raw-capture deletion, and resource cleanup together. Package
tests separately fail closed for a wrong Instance, a modified TLS record,
invitation replay and wrong profile/run/Rendezvous binding, oversized or
unknown frames, invalid Introduction state, and Rendezvous cancellation.

The same immutable images also run the fixed process-failure smoke; no rebuild
is allowed between the positive and failure cases:

```sh
go run ./cmd/carrier-lab native-run \
  --repository-root "$PWD" \
  --session-root "$SESSION_ROOT.failure" \
  --temp-root "$TEMP_ROOT" \
  --run-id "$RUN_ID-failure" \
  --application-image "$(cat /external/absolute/application-image.id)" \
  --tool-image "$(cat /external/absolute/tooling-image.id)" \
  --fault rendezvous-process
```

After the User verifies the first 16 KiB protected Application chunk, the
controller sends `SIGKILL` to the actual Rendezvous container. Both endpoints
must record an explicit failure within 15 seconds, the User may retain exactly
that authenticated prefix, neither endpoint may accept the complete stream,
and cleanup remains mandatory. `failure_smoke_passed` is development evidence,
not a timed recovery or availability result.

`development_smoke_passed` closes only the implementation slice. It is not the
R-013 comparative verdict: the official Ubuntu 26.04 runner, 20 setup attempts,
timed bidirectional streams, Direct/C-3 measurements, the full workload form of
the process-failure condition, and later Tor/Chutney reference remain a separate
experiment run.

## Run

```sh
bash ./scripts/preflight.sh --go-archive /absolute/path/go1.26.5.linux-amd64.tar.gz
```

The host operating system is evidence, not a success condition. The pinned
container must be Ubuntu 26.04 `linux/amd64`; Docker is the portability seam.

An optional `--seed` accepts letters, digits, dot, underscore, and hyphen. Each
run receives one unique session below the system temporary directory. The Go
verifier derives and validates `RunDir` and `EvidenceDir` from that
`SessionRoot` and `RunID`; callers cannot supply unrelated output paths. Go
build and module caches stay inside the disposable run directory. The
repository is mounted read-only and the verifier container has no network.

## Retained evidence

After the pinned Go verifier starts, the command prints the absolute evidence
directory and retains exactly four canonical summary files outside the
repository:

- `preflight-manifest.json`: inputs, source revision/dirty state, platform,
  tool versions, digests, parameters, stages, monotonic elapsed times, status,
  failure reasons, and final cleanup state;
- `verdict.json`: status and the manifest SHA-256;
- `report.md`: short human-readable result and non-claims;
- `cleanup.json`: machine-readable cleanup result.

The run directory, caches, extracted toolchain, temporary credentials, sockets,
and owned containers, networks, and volumes are removed on success and failure.
Cleanup is performed twice to prove idempotence. `preflight_checks_passed` is
only an intermediate state; the canonical verdict becomes `passed` only after
the finalizer has verified complete cleanup. A cleanup failure makes the
verdict fail and the command exit non-zero.

An error before the pinned Go verifier starts retains only
`bootstrap-failure.json` with schema
`carrier-lab-bootstrap-failure/v1`. It is bootstrap diagnostics, not a
canonical preflight manifest, verdict, or cleanup record.

## Outcomes and non-claims

`passed` means only that the exact pinned environment was verified, the evidence
was written, and all owned ephemeral resources were removed. `preflight_failed`
includes a stable reason code such as missing input/tool, unsupported platform,
digest mismatch, unsafe workspace, stage failure, or cleanup failure.

This is maintained project code but still disposable laboratory behavior. It
provides no compatibility, privacy, anonymity, security, availability,
decentralization, or production claim. It executes no Route, Node role,
Introduction, topology, TLS/HPKE flow, Service Connection, naming, discovery,
public bootstrap, Application Interface, SDK, Windows client, or production API.
