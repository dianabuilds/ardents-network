---
id: R-036
title: Which Camouflage Adapter and pinned implementation fits Horizon 3 Stage 5?
status: blocked
owner: product research
started: 2026-08-15
reviewed: 2026-08-15
---

# R-036 — Horizon 3 Camouflage Adapter

## Decision this unlocks

Freeze the exact candidate-neutral Camouflage Adapter contract and the comparison
of exactly Lyrebird obfs4 and standalone WebTunnel that must precede selection
of one pinned candidate for the maintained Horizon 3 Stage 5 slice.

This is source and contract research only. It cannot close R-036: R-033 requires
both candidates to be exercised before one is selected, while the controlling
Stage 5 gate forbids an Adapter experiment until R-036 is already `decided` and
an implementation brief is accepted. Source inspection cannot replace the
required useful-work, process, DNS, resource, shutdown, and cleanup evidence.

The Product Owner must explicitly change that ordering before this record can
advance. No dependency, runtime binary, Adapter experiment, package, command,
integrated campaign, or censorship-resistance claim is authorized here. R-033
and R-035 still need explicit acceptance; R-037 and a Stage 5 implementation
brief remain independent gates.

## Current contract

The authoritative inputs are:

- [Horizon 3 scope](../../product/scope.md#horizon-3--closed-test-network);
- [J-06 degraded/blocked-path journey](../../product/journeys.md#j-06--continue-through-degradation-or-recover-from-a-failed-path);
- [Stage 5 technical design](../../development/horizon-3-technical-design.md#stage-5--bridge-and-blocked-entry);
- [R-009 Bridge architecture](r-009-hostile-bootstrap-and-bridge-entry.md#bridge-entry-contract-for-option-c);
- [R-033 Stage 5 research map](r-033-h3-stage-5-research-map.md);
- [R-034 capacity sequencing](r-034-stage-4-bridge-capacity-sequencing.md);
- [R-035 transport-neutral Bridge state](r-035-h3-bridge-state.md);
- [R-023 useful-work contract](r-023-interactive-route-performance-budget.md#p3-d6b2b2b--controlled-application-payload-suite);
- [R-032 recovery ownership](r-032-h3-same-connection-recovery.md);
- [ADR-0005](../../adr/0005-route-domains-and-bounded-entry-exposure.md); and
- [Bridge-entry threat boundary](../../security/threat-model.md#bridge-entry).

The canonical glossary definitions are
[Bridge](../../../CONTEXT.md#bridge),
[Carrier Channel](../../../CONTEXT.md#carrier-channel),
[Entry Set](../../../CONTEXT.md#entry-set),
[Role Domain](../../../CONTEXT.md#role-domain), and
[Work Safety Lease](../../../CONTEXT.md#work-safety-lease).

The Route Module already owns authenticated Route selection and asks for one
endpoint-adjacent Carrier Channel. R-035 owns Invite validity, the Initiator
Bridge Entry Set, regime, exposure, contact order, retry, deadlines, and
exhaustion. The Adapter may receive only the already selected numeric Bridge
endpoint, one validated candidate configuration, one absolute parent deadline,
and one reserved resource lease. It returns one opaque ordered bidirectional
Carrier Channel or one internal classified failure. It does not receive or
select a Role Domain, Service Name, Service Target, opposite endpoint, Route,
retry, replacement, regime, or continuity policy.

R-037 remains owner of the integrated topology, censor profiles, sample counts,
candidate-independent CPU/memory/bandwidth thresholds, attempt/contact clocks,
and `pass|fail|invalid` verifier. R-036 fixes the per-Adapter structural limits,
startup and cleanup deadlines, identical useful-work input, and supply identity
that R-037 must use without relaxation.

## Protected claim and honest limitation

1. **Information:** the Adapter must not learn or emit the Service Name, Service
   Target, opposite endpoint, full Route, Application meaning, another Role
   Domain's membership, or another Isolation Context's state. Candidate secrets,
   dial addresses, and raw candidate logs stay outside ordinary diagnostics and
   publishable evidence.
2. **Adversary:** a censor that blocks an address or protocol, an unauthenticated
   probe, an informed probe holding the Invite, a malicious or stalled Bridge,
   malformed candidate process, compromised upstream artifact, and a crash or
   resource-pressure event that tries to escape bounds or cause fallback.
3. **Conditions:** an uncompromised Endpoint, authenticated current Bridge state,
   a preselected candidate campaign, exact pinned source and offline supply,
   R-037's accepted controlled profile, certificate/key material prepared out of
   band, and unchanged Route and Service Connection authentication.
4. **Measurement:** R-037 must observe the resolver and network boundaries,
   process tree, listeners, sockets, state directory, traffic, resources,
   control transcript, byte digests, shutdown, and residual resources.
5. **Limitation:** the Bridge, censor, TLS endpoint, and their observers may see
   Endpoint/Bridge addresses, TLS server name or obfs4-shaped traffic, timing,
   direction, duration, volume, retries, and blocking. An informed probe with an
   Invite may reproduce the candidate handshake. This decision claims neither
   invisibility, indistinguishability, anonymity against correlation,
   availability, nor resistance outside the exact R-037 profiles.

## Hypotheses

- **H1 — standalone WebTunnel:** a pinned standalone WebTunnel subprocess can
  carry the exact H3 stream through a protocol allow-list without Endpoint DNS,
  while keeping TLS name/certificate validation and web-front operation inside
  a bounded candidate configuration.
- **H2 — Lyrebird obfs4:** a pinned Lyrebird subprocess can provide stronger
  unauthenticated-probe rejection with fewer Bridge-side helpers, and its much
  larger multi-transport supply closure remains acceptable for one maintained
  H3 obfs4 profile.
- **H0 — stop:** neither candidate satisfies the same interface, offline supply,
  no-DNS/no-fallback, resource, shutdown, maintenance, and useful-work contract.

## Pre-registered evaluation criteria

The comparison is conjunctive. Convenience or throughput cannot compensate for
a failed knowledge, DNS, supply, privilege, cleanup, or fallback gate.

### User outcome, security, and availability

- Both candidates receive the same already selected Initiator Bridge and must
  yield either the same authenticated network, exact Target, Route shape,
  Service Connection, and ordered Application-visible bytes, or the same
  existing classified terminal Connection Result by the parent deadline.
- The Adapter may inspect only endpoint-adjacent transport metadata and opaque
  bytes. A raw control/log record containing an Invite secret, address, or
  candidate argument is secret evidence and never ordinary diagnostics.
- Address blocking must produce a bounded failure. No candidate may perform
  direct, ordinary-entry, DNS, proxy, alternate-candidate, shorter-Route, cached
  success, or weaker fallback.
- An unauthenticated active probe receives no Ardents-specific success signal.
  Informed-probe success after Invite disclosure is recorded as a limitation,
  not mislabeled as a pass.
- Every candidate-internal connection or handshake remains inside the one
  R-035 contact. It cannot obtain another contact, retry, Bridge, candidate, or
  deadline.

### Exact useful work and finite local envelope

Each candidate carries exactly the same R-023 interactive unit: fresh `32-byte`
connection canaries, the exact nonce-bearing `512-byte` request, and the exact
`64 KiB` verified incompressible response. The Adapter treats those bytes as an
opaque ordered bidirectional stream. R-037 owns repetitions, concurrency,
latency, goodput, traffic, CPU, and memory verdict numbers; it must apply the
same values to each applicable candidate profile and count failed latency as
infinity and failed goodput as zero under R-023.

One Adapter contact is limited to:

- one unprivileged candidate child process with zero Linux capabilities and
  zero descendants, one PT client method, one
  loopback-only ephemeral listener, and one live Carrier Channel;
- at most four candidate-owned live sockets at the Endpoint: the listener, both
  loopback connection endpoints, and one remote connection;
- one owned state directory with at most `32` filesystem entries and `1 MiB`
  total apparent bytes, no links, devices, nested mounts, or writes elsewhere;
- at most `64` startup control lines, `4096` bytes per line, and `64 KiB` total
  stdout plus stderr before readiness; and
- one `5 s` startup deadline and one `6 s` shutdown/cleanup deadline. Neither
  starts or extends the R-035/R-037 parent contact or attempt deadline.

The controlled Bridge fixture charges every candidate/helper process, listener,
socket, file, and byte. WebTunnel's server and TLS front therefore count even
though they are not Endpoint Adapter children. Its server is one unprivileged
child bound to loopback. Its TLS front is later authorized Stage 5 harness code
using the pinned Go standard library: it terminates the preprovisioned pinned
certificate, forwards only the exact Upgrade path to that loopback server, and
returns one bounded ordinary HTTP response for every other path. It adds no
third-party binary or runtime download and may not implement or modify the
WebTunnel protocol. Lyrebird's comparison fixture uses one unprivileged server
child and no web front. R-037 may set stricter CPU, memory, bandwidth, disk, and
process limits, but cannot omit helpers or increase the structural counts above.

### Supply, maintenance, operation, and accessibility

- Source identity is an exact tag, commit, Git tree, and the `go.mod`/`go.sum`
  bytes in that tree. Preparation records archive/module hashes, license texts,
  dependency graph, SBOM, and `govulncheck` result before any build is accepted.
- After one explicit preparation step outside the repository, the candidate
  must build with `CGO_ENABLED=0`, `-trimpath`, and network access disabled. The
  runtime image contains only the selected executable, required notices, and
  fixed system inputs. No candidate module enters Ardents `go.mod`.
- First-party cgo or `unsafe`, a candidate cgo requirement, unexpected process
  privilege/capability, mutable runtime download, unverified artifact, license
  incompatibility, reachable high/critical advisory, or offline-build failure
  selects H0. A source-only scan is not a substitute for that later locked
  supply verification.
- The maintained slice must fit one Product Owner and Codex. Public Invite
  acquisition, installer UX, external operators, public DNS discovery, and
  censored-region support are not silently assigned to unavailable staff.

## Exact candidate-neutral Adapter contract

This is a logical Module boundary, not a speculative exported Go interface or
public protocol. The accepted implementation brief must map it to cohesive
existing/new package ownership only after all Stage 5 gates close.

### 1. Pure candidate-envelope validation

At atomic Invite import, the campaign's preselected validator receives exactly
the opaque `candidate_envelope` and the selected Bridge identity. It performs no
clock read, DNS, dial, process start, file write, discovery, or fallback. It
returns one immutable `ValidatedAdapterConfig` or `adapter-config-invalid`.

The envelope's maximum length is `1024` bytes. Its implementation-brief
encoding must be canonical, versioned, reject unknown/duplicate/trailing fields,
and contain exactly one `adapter_profile_id`, one numeric global-unicast IPv4
address plus TCP port, and the selected candidate fields below. Loopback,
unspecified, multicast, link-local, private, IPv6, and non-canonical text forms
are invalid for this bounded comparison. The envelope contains no Role Domain, Entry Set,
regime, retry, deadline, Target, Route, proxy, alternate address, distributor,
or fallback instruction.

For Lyrebird obfs4 the candidate fields are exactly one canonical obfs4 `cert`
and `iat-mode=0`. The dial target is the envelope's numeric IP/port. Other
Lyrebird transports, `iat-mode` values, upstream proxies, logging flags, and
extra arguments are invalid.

For standalone WebTunnel the candidate fields are exactly one non-empty HTTPS
path, one TLS server name, and one SHA-256 certificate-chain pin. The Adapter
constructs the subprocess arguments as an HTTPS URL whose host is the envelope's
numeric IPv4 literal and explicit port, plus the separate `servername` and `cert`
values. Userinfo, query, fragment, HTTP, empty/root path, another address, and
unpinned TLS are invalid. The TLS name is only SNI/HTTP Host input and is never
resolved by the Endpoint.

### 2. One bounded `OpenCarrier` operation

The caller supplies one attempt/contact identity, validated config, absolute
parent deadline, cancellation signal, and already reserved resource lease.
`OpenCarrier` performs exactly this sequence:

1. create the per-contact owned state directory outside the repository;
2. launch the absolute-path pinned candidate with a sanitized environment;
3. negotiate PT version `1`, accept exact readiness, and reject terminal or
   malformed control output;
4. open one SOCKS5 connection to the reported loopback listener, passing only
   the validated numeric target and candidate arguments;
5. return that connected stream as the opaque Carrier Channel only after the
   SOCKS request is granted; and
6. retain ownership of child, pipes, listener observation, state, and cleanup
   until the channel is closed or the parent cancels.

There is no hidden retry in this interface. Candidate library packets and one
handshake are internal to the current contact; a failed `OpenCarrier` is
terminal for that contact and returns a bounded internal Adapter outcome to the
R-035 ledger.

The only child environment additions are:

```text
TOR_PT_MANAGED_TRANSPORT_VER=1
TOR_PT_CLIENT_TRANSPORTS=<exact selected method>
TOR_PT_STATE_LOCATION=<owned per-contact directory>
TOR_PT_EXIT_ON_STDIN_CLOSE=1
```

`TOR_PT_PROXY`, server variables, ambient proxy variables, candidate logging
flags, and transport lists are absent. The executable path is absolute; the
child has no network configuration or runtime-download authority.

Readiness requires, in order, `VERSION 1`, exactly one
`CMETHOD <selected-method> socks5 <loopback-ip>:<ephemeral-port>`, and one
`CMETHODS DONE`. `CMETHOD-ERROR`, `VERSION-ERROR`, `ENV-ERROR`, `PROXY-ERROR`,
missing/duplicate/conflicting required messages, another method, SOCKS version,
non-loopback or fixed listener, NUL, non-ASCII keyword, oversized line/transcript,
EOF, exit, or timeout before completion returns `adapter-control-invalid` or
`adapter-not-ready`. Bounded unknown PT keywords are ignored as the PT
specification requires; they cannot grant readiness. Stdout after readiness is
still drained and bounded by R-037. Stderr is drained to secret evidence only.

### 3. Idempotent shutdown and cleanup

Close/cancel first closes the Carrier Channel and child stdin, then waits `1.5 s`.
If the child remains after `1.5 s`, it sends `SIGTERM` and waits another `1.5 s`;
if it remains, it sends `SIGKILL` and allows `0.5 s` for reap. The remaining
`2.5 s` is reserved for pipe/socket closure, evidence hashing, directory removal,
and residual verification. The complete sequence is inside a fixed `6 s`
cleanup deadline and never extends its parent.

Cleanup closes pipes and owned sockets, records bounded secret-evidence hashes,
removes the owned candidate directory, and verifies zero owned processes,
descendants, listeners, sockets, files, queues, or timers. Close is idempotent.
Any residual or missed deadline returns `adapter-cleanup-failed`, invalidates the
campaign cell, and blocks another contact until the external harness has removed
the isolated fixture. Restart never restores a process or Carrier Channel.

## Pinned source and offline supply

No runtime artifact was downloaded or built for this record. Read-only source
checkouts lived under an owned system-temporary directory and are not supply
artifacts.

### O1 — Lyrebird obfs4 source identity

- upstream: `https://gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/lyrebird.git`;
- annotated OpenPGP-signed tag: `lyrebird-0.8.1`;
- commit: `0b10edbb61e0ca6fb70c7d57aeaabf315f1fade1`;
- Git tree: `e07f3f4d8b2266f29c67813fcb5c3b1cd46d8792`;
- commit date: `2026-01-15T13:37:52Z`; and
- build target: `./cmd/lyrebird`, with only `obfs4` enabled at runtime.

Its exact tag and complete 64-line declared module closure would have to be
archived and verified offline. No dependency may be raised ad hoc because doing
so creates an unreviewed source identity; a remediated upstream release would
repeat R-036.

### O2 — standalone WebTunnel source identity

- upstream: `https://gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/webtunnel.git`;
- annotated OpenPGP-signed tag: `v0.0.6`;
- commit: `d729fde1f38357dcefa2a751eb4752e9ca78f910`;
- Git tree: `fe82090e6a523d1b05d602f934fda515354c8cf5`;
- commit date: `2026-07-23T15:02:45+02:00`; and
- build targets: `./main/client` and `./main/server`.

The sole declared module dependency is goptlib `v1.6.0`, module sum
`h1:KD9m+mRBwtEdqe94Sv72uiedMWeRdIr4sXbrRyzRiIo=` and `go.mod` sum
`h1:70bhd4JKW/+1HLfm+TMrgHJsUHG4coelMWwiVEJ2gAg=`. Its upstream source identity
is commit `f4bb5dd5725833bd880347b8fbaf60522ed0a710` and Git tree
`ada1d375537ffa240b9d0eb91be505f9eb80c60b`, tagged `v1.6.0`.

The supply environment is the repository-pinned Ubuntu image
`ubuntu@sha256:7b202b0e2e0028c6250f5fcf41d04df492d145a1654c6995a6553f0c1f6f1960`
and official `go1.26.6.linux-amd64.tar.gz` with SHA-256
`5c2c3b16caefa1d968a94c1daca04a7ca301a496d9b086e17ad77bb81393f053`.
The online preparation, run outside this repository with empty owned caches,
must use the exact candidate tree and execute:

```sh
go mod download -json all
go mod verify
go mod vendor
govulncheck ./...
```

The scanner executable is repository-pinned `govulncheck v1.1.4`.

It then records SHA-256 for the canonical source archive, `go.mod`, `go.sum`,
every downloaded module `.mod`/`.zip`, vendor tree, license, SBOM, advisory
report, and build recipe. The offline build mounts that prepared source/vendor
tree read-only in the pinned Ubuntu image and executes the applicable command:

```sh
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOPROXY=off GOSUMDB=off \
  go build -mod=vendor -trimpath -buildvcs=false -ldflags=-buildid= \
  -o /out/lyrebird ./cmd/lyrebird

CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOPROXY=off GOSUMDB=off \
  go build -mod=vendor -trimpath -buildvcs=false -ldflags=-buildid= \
  -o /out/webtunnel-client ./main/client
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOPROXY=off GOSUMDB=off \
  go build -mod=vendor -trimpath -buildvcs=false -ldflags=-buildid= \
  -o /out/webtunnel-server ./main/server
```

A second clean offline build must reproduce each binary SHA-256. Generated
artifacts, module caches, keys, binaries, and SBOM remain outside Git. A changed
source, toolchain, base image, dependency, hash, license, or build recipe repeats
this gate. The comparison cannot begin until its immutable supply manifest is
accepted in the future authorized brief.

## Evidence plan

### Primary sources

Accessed 2026-08-15:

- [Tor PT specification](https://spec.torproject.org/pt-spec/specification.html),
  [IPC](https://spec.torproject.org/pt-spec/ipc.html), and
  [shutdown](https://spec.torproject.org/pt-spec/shutdown.html) for subprocess,
  readiness, SOCKS listener, control-line, and termination semantics;
- official Lyrebird `lyrebird-0.8.1` source, tag, licenses, `go.mod`, obfs4
  parser, client setup, logging, proxy, and shutdown code at the pinned identity;
- official standalone WebTunnel `v0.0.6` source, tag, MIT license, `go.mod`,
  README, client config/dial, TLS, HTTP Upgrade, logging, and shutdown code at
  the pinned identity;
- official goptlib `v1.6.0` source, signed tag, CC0 dedication, and module sums;
- [Tor WebTunnel design description](https://blog.torproject.org/introducing-webtunnel-evading-censorship-by-hiding-in-plain-sight/)
  for the protocol-allow-list comparison;
- [Tor WebTunnel deployment guide](https://community.torproject.org/relay/setup/webtunnel/)
  for reverse-proxy, domain, TLS, and operator requirements; and
- [Cure53 TTP-03 report](https://blog.torproject.org/code-audit-censorship-circumvention-tools/TTP-03-report.pdf)
  and [Tor audit summary](https://blog.torproject.org/code-audit-censorship-circumvention-tools/)
  for the 2024 WebTunnel audit scope and its limitations.

### Blocked conformance and integrated evidence

This record authorizes no experiment. The present gate order would allow the
comparison only after R-036 had already selected a candidate, contradicting the
required evidence order. If the Product Owner authorizes a narrow ordering
change, both candidates must first execute the source/supply preflight and the
candidate-neutral contract tests. They remain separate campaign configurations;
no run or failure switches candidates. Only the appended evidence and a later
explicit Product Owner decision may then select one maintained profile and mark
R-036 `decided`.

The disposable feasibility evidence must include exact source/binary hashes,
offline build transcript, licenses/SBOM/advisories, PT control transcript,
resolver trace, process tree, state manifest, listeners/sockets, resource and
traffic series, useful-work digests, injected malformed control cases, every
shutdown rung, and residual scan. R-037 owns the integrated blocked-entry cells
and independent verifier.

### Failure scenarios and falsification

Either candidate is falsified by any of:

- wrong/unverifiable source, tag, tree, module sum, binary hash, license, SBOM,
  offline build, cgo/unsafe/privilege result, or reachable high/critical advisory;
- public DNS query, system/ambient proxy use, runtime download, direct or
  ordinary-entry fallback, alternate endpoint/candidate, or non-loopback listener;
- malformed/oversized/ambiguous PT control accepted as ready, readiness after
  the startup deadline, more than one child/listener/channel, descendants, or
  state/write/socket escape;
- useful-work corruption, reordering, truncation, unauthorized success signal,
  Target/Route/profile substitution, or candidate-specific workload relaxation;
- address/protocol block, partial handshake, accept-then-stall, process crash,
  cancellation, or pressure resetting exposure, retry, contact, or deadline;
- shutdown/cleanup deadline miss or any residual owned resource; or
- an operator, DNS, certificate, distributor, audit, or maintenance dependency
  that the one-to-one H3 controlled fixture cannot actually supply.

For WebTunnel specifically, any Endpoint DNS packet or resolver request for the
TLS server name selects H0 for this revision. The Adapter may not patch the
protocol, substitute Lyrebird's embedded WebTunnel, or fall back to obfs4. For
Lyrebird specifically, selecting only the obfs4 method does not excuse the
complete linked supply closure, licenses, advisories, or dormant capability
surface from review and accounting.

## Findings

### Finding 1 — PT is a viable internal seam, not an Ardents protocol

**Sourced fact:** PT version 1 defines a parent-launched subprocess, environment
configuration, stdout readiness messages, a loopback SOCKS listener, and a
close-stdin/SIGTERM/force-kill shutdown sequence. It does not define Ardents
Bridge selection, Role Domains, Entry Sets, Route authentication, or continuity.

**Inference:** strict PT client-process mechanics fit below the Route only when
the Adapter validates one preselected configuration, accepts exactly one local
listener, and translates its output into one Carrier Channel. PT and SOCKS are
private implementation seams, not Application or public Ardents interfaces.

### Finding 2 — standalone WebTunnel can separate dial address from TLS name

**Sourced fact:** standalone WebTunnel `v0.0.6` parses an HTTPS URL, derives the
remote address from the URL host, accepts a separate `servername`, supports a
certificate-chain hash pin, dials the derived address, and uses the TLS server
name as SNI and HTTP Host. Its README's separate `addr` field is explicitly for
the Lyrebird version; the standalone client does not parse `addr`.

**Inference:** the H3 envelope must keep a numeric dial IP/port separate from the
TLS name and pin, while the Adapter serializes the numeric address into the
standalone URL. That removes public DNS as a design dependency. The later
resolver trace remains a hard falsifier because source inspection is not runtime
evidence.

### Finding 3 — the candidates address different censor profiles

**Sourced fact:** Tor describes obfs4 as fully encrypted/unrecognized traffic and
WebTunnel as WebSocket-like HTTPS that can coexist with an ordinary website.
Tor specifically reports that a deny-by-default protocol allow-list may reject
obfs4 while allowing WebTunnel-shaped HTTPS.

**Inference:** WebTunnel covers the harder Stage 5 protocol-allow-list profile;
obfs4 provides the stronger simple unauthenticated-probe handshake but remains
vulnerable to allow-list blocking. Neither survives known-address blocking or
an informed probe with the exact Invite. The candidates must not form a chain.

### Finding 4 — Lyrebird has materially greater supply and maintenance cost

**Measurement:** source-only inspection found 52 Lyrebird Go files and 64
declared module requirements at `lyrebird-0.8.1`. The executable registers a
multi-transport suite and its closure includes WebTunnel, Snowflake/WebRTC,
uTLS, AWS, Pion, DNS/mDNS, and other code even when H3 requests only obfs4.

**Measurement:** on 2026-08-15 the official Lyrebird repository exposed four
separate `[SECURITY]` dependency-update branches for the exact tag's
`filippo.io/edwards25519 v1.1.0`, `uTLS v1.6.7`, `x/crypto v0.33.0`, and
`x/net v0.35.0`. Those branches were not merged into a tagged remediated release.
This is upstream maintenance evidence, not a reachability verdict.

The immutable observed branch heads were, respectively,
`5ad22b6ae866d539bd5a5e54757b0a864ec24bb9`,
`0858c8b01645f60b867336a6f74ab97b3fbed804`,
`0bf8afedaa2b10de9adb1be7f9f705bef0ee6cad`, and
`983f735c277dd09808a76ac2c5fb62969beb6b4d`.

**Sourced fact:** Lyrebird's root is BSD-family, but linked obfs4 code imports
`internal/x25519ell2`, whose source states GPL-3.0-or-later and ships the GPLv3
text. Distribution therefore needs a complete license/notice/source-obligation
review; the root BSD notice alone is insufficient.

**Measurement:** the Lyrebird first-party Go tree had no exact `import "C"` or
`"unsafe"` import. That says nothing about its 64-requirement closure, which
remains a mandatory offline build and reachability check. The Cure53 TTP-03
scope lists WebTunnel but not Lyrebird, and no consulted official artifact binds
an independent audit to the exact `lyrebird-0.8.1` commit.

**Inference:** pinning 0.8.1 would either accept an unresolved security closure
or create an Ardents-specific dependency fork. Both violate the maintenance and
offline-supply gate for the current team.

### Finding 5 — standalone WebTunnel is the smaller maintained supply

**Measurement:** WebTunnel `v0.0.6` is the current upstream main commit at the
access date, contains 11 Go files, and declares only goptlib `v1.6.0`. Static
source inspection found no `import "C"` or Go `unsafe` import in WebTunnel or
goptlib. Its official remote exposed only that main head and no security-update
head at the access time; that is a maintenance signal, not a vulnerability
scan. WebTunnel is MIT; goptlib is CC0-1.0.

**Sourced fact:** Cure53's January–February 2024 assessment included WebTunnel
source in scope. Its published WebTunnel-related finding concerned Bridgestrap
SSRF when testing attacker-selected bridges, not the standalone transport path;
the report does not prove the 2026 pinned revision or Ardents integration safe.

**Sourced fact:** official deployment requires a TLS web front/reverse proxy and
normally a domain/certificate. The standalone client also logs SOCKS request
details to stderr and does not support an upstream proxy.

**Inference:** WebTunnel's client supply and removal surface are much smaller,
but Bridge-side TLS/reverse-proxy operation and secret log handling are real
costs. H3 can bound them with a preprovisioned numeric address, separate pinned
TLS identity, secret-only stderr, and complete helper accounting; none becomes a
public discovery service or permanent product trust root.

### Source-inspection reproduction

The source-only measurements used Windows 11 `amd64`, PowerShell
`5.1.26100.9168`, Git `2.45.1.windows.1`, and Go `1.26.6`. They did not build or
execute either candidate. Reproduce them in an owned system-temporary directory:

```text
git clone --branch lyrebird-0.8.1 --depth 1 https://gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/lyrebird.git lyrebird
git clone --branch v0.0.6 --depth 1 https://gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/webtunnel.git webtunnel
git clone --branch v1.6.0 --depth 1 https://gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/goptlib.git goptlib
git -C <repo> show -s --format="commit=%H tree=%T date=%cI" <tag>
rg --files <repo> -g "*.go"
git -C lyrebird show lyrebird-0.8.1:go.mod
rg -n '^\s*(?:import\s+)?(?:[._A-Za-z][A-Za-z0-9_]*\s+)?"(?:C|unsafe)"\s*(?://.*)?$' lyrebird webtunnel goptlib -g "*.go"
git ls-remote --heads https://gitlab.torproject.org/tpo/anti-censorship/pluggable-transports/lyrebird.git
```

PowerShell `(rg --files <repo> -g "*.go" | Measure-Object).Count` yields
Lyrebird `52` Go files, WebTunnel `11`, and goptlib `9`. PowerShell
`(git -C lyrebird show lyrebird-0.8.1:go.mod | Where-Object { $_ -match
'^\s+\S+\s+v\S+(?:\s+// indirect)?$' }).Count` yields `64` exact block-form
requirements in the pinned Lyrebird `go.mod`; this is the declared module list,
not a claim that every module is linked into the obfs4 call graph.
The exact import scan returned no match; Lyrebird's unrelated `unsafeLogging`
identifier is not a Go `unsafe` import. The exact tag
commits/trees, module sums, security-branch heads, commands, and results are
retained in this record; disposable checkouts and raw Git metadata are not
evidence artifacts and are removed after review.

## Options

| Option | Product/security fit | Performance, resources, and availability | Operations, maturity, supply, license, and DX |
|---|---|---|---|
| O1 — Lyrebird obfs4 `0.8.1` | Authenticated obfs4 handshake is strong against an uninformed probe, but random-looking traffic loses the protocol-allow-list cell. No DNS is needed. | One client child/listener fits the seam and obfs4 adds framing/padding; exact costs remain R-037. Failure is explicit, with no WebTunnel fallback. | Mature Tor component and simpler Bridge front, but a 64-module multi-transport closure, four open security-update branches, temporary uTLS fork, and GPL-3.0-or-later linked code create disproportionate audit, distribution, and maintenance cost. |
| O2 — standalone WebTunnel `v0.0.6` | HTTPS-shaped traffic addresses the allow-list profile. Numeric URL host plus separate pinned TLS name removes Endpoint public DNS; an informed probe and address block remain limitations. | One client child/listener fits the seam; TLS/HTTP Upgrade overhead and all Bridge helpers are charged. R-037 must prove the same bytes and thresholds with DNS zero. | Current small release, one CC0 dependency, MIT transport, no observed cgo/unsafe imports, and clean removal. Requires a controlled TLS reverse proxy, certificate fixture, path-secret handling, and secret-only stderr. |
| O0 — choose none | Preserves the product contract when neither candidate passes; no first-party camouflage, direct fallback, or third family is allowed. | Stage 5 returns `stop`; no capacity or availability result is claimed. | Lowest hidden maintenance cost; R-036 must be reopened with a changed contract or later candidate decision. |

## Recommendation

Do **not** choose a maintained candidate yet. Accept the candidate-neutral
contract and supply precommit only after explicitly resolving the experiment-
ordering conflict. Then exercise O1 and O2 as separate configurations under the
same contract and select exactly one from that evidence; failure of both selects
O0. Source evidence currently favors O2 for supply size and protocol-allow-list
fit, but that is a prior for the comparison, not a selection.

The strongest argument against the current O2 prior is that its ordinary-HTTPS appearance depends
on correct TLS/web-front operation and a secret path, while an informed probe or
address blocker still wins and the current standalone client does not expose an
explicit `addr` argument. The numeric-host construction and resolver observation
make the no-DNS condition falsifiable; they do not turn controlled evidence into
an indistinguishability or real-world censorship-resistance claim.

## Disposition

- State: `blocked`; the Adapter contract is ready for review, but candidate
  selection is correctly withheld.
- Blocker: the required two-candidate exercise must precede selection, while the
  controlling gate forbids that exercise until R-036 is already `decided` and a
  brief exists. Product Owner authorization must change this ordering explicitly.
- Candidates remain exactly standalone WebTunnel `v0.0.6` and Lyrebird obfs4
  `0.8.1` at the commit/tree identities above; neither is maintained or packaged.
- No runtime binary, module dependency, experiment, package, command, public
  protocol, ADR, or implementation authority was created.
- Generated source checkouts are disposable and must be deleted from the owned
  system-temporary directory after review checks.
- Documents changed: this R-036 record and the R-036 row in
  [questions.md](../questions.md).
- Remaining gates before Stage 5 code: explicit acceptance of R-033, R-035, and
  the R-036 ordering change plus final selection; completed and accepted R-037;
  R-036 marked `decided`; and an explicit accepted Stage 5 implementation brief.
