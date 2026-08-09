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

Missing inputs are a failed preflight. The thin script sets offline Go launcher
controls and invokes `internal/preflight`; it does not own Docker lifecycle,
cleanup, or evidence policy. The Go Module does not pull an image, download Go,
install a tool, accept a mutable image tag, or use the host-built launcher as
success evidence. The candidate and verifier are rebuilt with the supplied
pinned archive before a passing verdict is possible.

## Reproducible Carrier Lab image

`Dockerfile.carrier-lab` is a two-stage build whose builder and runtime both
use the exact R-013 Ubuntu digest. The Go archive stays outside the repository
and ordinary build context. BuildKit receives its containing directory as the
named `go_archive` file context; the Dockerfile mounts only the expected archive
read-only and verifies its SHA-256 before extraction. Build and module caches
are tmpfs mounts, and every build `RUN` has `--network=none` with
`GOTOOLCHAIN=local`, `GOPROXY=off`, `GOSUMDB=off`, and `CGO_ENABLED=0`.

An equivalent direct development build is:

```sh
docker build --no-cache --pull=false --network=none \
  --build-context go_archive=/absolute/directory/containing-the-go-archive \
  --file Dockerfile.carrier-lab \
  --tag ardents-carrier-lab:development .
```

The final stage is the pinned Ubuntu runtime plus
`/usr/local/bin/carrier-lab`; it contains no source tree, Go toolchain, or build
cache and defaults to numeric user `65532:65532`. The preflight independently
builds the same binary twice through the Dockerfile and pinned verifier path,
requires matching SHA-256 values, and records `image.base_image_id`,
`image.carrier_lab_image_id`, and `binary.sha256` in the canonical manifest.
The image is disposable Carrier Lab input, not a release artifact.

## Two-role isolation smoke

`compose.carrier-lab.yaml` defines exactly two disposable roles, `alpha` and
`beta`, on one Compose-internal `adjacency` network. There is no default,
external, host, or published-port path. Each role runs as numeric user
`65532:65532` with a read-only root filesystem, all capabilities dropped,
`no-new-privileges`, bounded CPU, memory, PIDs, and tmpfs, one read-only role
configuration, and one separate writable evidence bind. Neither role receives
the repository, Docker socket, another role's evidence, or topology data beyond
its one allowed peer.

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
a fallback. The harness creates one ephemeral Ed25519 Target root, one active
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

## Native candidate prerequisite stop

The native C-5/C2 scenario is not implemented. R-013 requires actual link
shaping and per-link packet captures, but the pinned offline runtime has no
`tc` or capture tool and no reviewed content-addressed tool artifact is
registered. R-025 records the minimal supply decision needed before code may
resume. Direct TLS success cannot substitute for the C-5/C2 topology, its C2
Introduction Path, controlled network, role-view evidence, mandatory failures,
or mature Tor/Chutney comparison.

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
