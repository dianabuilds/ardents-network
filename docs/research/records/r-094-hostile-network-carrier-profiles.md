---
id: R-094
title: Hostile-network carrier and Entry transport profiles
status: decided
owner: Product Owner and Codex
started: 2026-08-24
reviewed: 2026-08-24
---

# R-094 — Which authenticated carrier and Entry transport profiles can replace one another under hostile-network failure without weakening Route semantics, creating a downgrade path, or making unbounded new endpoint exposure?

## Decision this unlocks

Decide the H4-2 carrier seam, the first additional authenticated transport
profile after TCP/TLS, and whether any entry-only camouflage profile is justified
for a bounded alpha or later public-network claim. The decision must distinguish
replacement of a failed Carrier/Route attachment from an impossible promise to
continue one TCP or QUIC session unchanged over another wire protocol.

## Current contract

- ADR-0024 selects `ardents-interactive-route-v1`: native Route, mutually
  authenticated TLS 1.3 over TCP carrier legs, authenticated Network State as
  the sole Route/discovery authority, endpoint-local Route selection, bounded
  Entry lifecycle, and Service Connection-owned recovery.
- No current Node, peer, source, Bridge, or H3 artifact may choose a lower
  transport/profile or act as an unbounded fallback authority.
- State-referenced Entry Invites do not contain a carrier endpoint/key. Current
  State supplies authenticated candidate and mutual-TLS facts.
- The threat model assumes blocking, DPI, active probing, source exposure,
  malicious infrastructure, resource exhaustion, and traffic analysis. A
  transport that works on one network is not a censorship-resistance claim.

Relevant owners: [Network Route and Node](../../technical/network-route-node.md),
[ADR-0024](../../adr/0024-native-interactive-route-foundation.md),
[ADR-0025](../../adr/0025-state-referenced-entry-invites.md), and the
[threat model](../../security/threat-model.md).

## Hypotheses

- **H1:** A deep internal Carrier Module can present one authenticated,
  resource-bounded reliable-carrier Interface to Route/Entry while TCP/TLS and
  QUIC adapters vary behind its seam. State-authorized profile selection and
  Service Connection recovery can replace a failed attachment without exposing
  a transport-specific choice to a Node or Application.
- **H2:** Entry camouflage must be a separate endpoint-adjacent Adapter class,
  with its own direct-source/distribution/exposure contract, rather than a
  generic replacement for every inter-Node carrier leg.
- **H0:** No evaluated profile set meets the current Route, source-exposure,
  resource, and maintenance contract; TCP/TLS remains the sole maintained
  profile until the product contract changes.

## Evaluation criteria

- **Semantics:** exact peer/profile authentication, Route/Leg binding, ordered
  reliable byte delivery, finite backpressure, cancellation, drain, cleanup,
  and Service Connection recovery remain identical at the Carrier Interface.
- **Selection safety:** only authenticated State/publication capability selects
  an offered profile; unsupported, blocked, stale, replayed, or downgraded
  choices fail explicitly. A Node never chooses the weaker profile.
- **Exposure:** retry/profile racing, bridge acquisition, STUN/TURN/broker use,
  and direct source contact have finite, precommitted endpoint exposure and do
  not create a hidden first-contact or resolver fallback.
- **Hostile network:** active probing, DPI classification, UDP blocking,
  TCP/UDP reset/drop/reordering, NAT rebinding, MTU variation, censorship of
  known endpoints, and selective profile blocking have declared results.
- **Resources and operations:** CPU, memory, file descriptors, ports, traffic,
  anti-abuse, observability, update, drain, rollback, and failure recovery are
  finite on the selected reference host. A transport needing a hidden bridge
  fleet, broker, proxy, or 24/7 operator team records that dependency.
- **Maintenance:** implementations are reviewed/maintained, have compatible
  licenses, and can be qualified by the current one-to-one project team. A
  popular library alone is not evidence of fit.

## Evidence plan

### Primary sources

- RFC 9000 (QUIC), RFC 9221 (QUIC datagrams), RFC 9114 (HTTP/3), accessed
  2026-08-24.
- RFC 9298 (CONNECT-UDP) and RFC 9484 (CONNECT-IP), accessed 2026-08-24.
- Tor specifications for Bridges and Pluggable Transports, accessed
  2026-08-24.
- Official implementation documentation and security advisories for each
  candidate library, accessed during evaluation.

### Disposable QUIC dependency review

At the experiment stage, the exact `github.com/quic-go/quic-go v0.61.0`
dependency was accepted only for the build-ignored R-094 two-adapter harness.
That historical gate did not itself authorize a maintained import. The later
H4-2B decision completed the direct-dependency review and promoted the same
pinned version under ADR-0048.

- the cached module checksum is
  `h1:ui88A53s8MSVYLC56en0KQ17HARk+9986Dn0SBfKNvA=` and its `go.mod` requires
  Go 1.25.0; the experiment uses the repository's Go 1.26.6 toolchain;
- the upstream v0.61.0 release is immutable and signed, and its source is MIT
  licensed. Its release notes also contain breaking/removal changes, so
  `quic-go` types must remain private Adapter implementation details:
  <https://github.com/quic-go/quic-go/releases/tag/v0.61.0> and
  <https://raw.githubusercontent.com/quic-go/quic-go/v0.61.0/LICENSE>;
- upstream publishes a private vulnerability-reporting path and public
  advisories. Earlier core/path/handshake vulnerabilities and earlier HTTP/3
  vulnerabilities demonstrate the need for per-release requalification; the
  first experiment imports no `http3` package:
  <https://github.com/quic-go/quic-go/security> and
  <https://raw.githubusercontent.com/quic-go/quic-go/v0.61.0/SECURITY.md>; and
- a 2026-08-24 binary-mode `govulncheck` of the exact baseline executable found
  zero reachable or imported-package vulnerabilities. It reported one
  module-level `golang.org/x/crypto/openpgp` advisory, but the binary does not
  import or call that package. This is a point-in-time experiment gate, not a
  future maintained-release security claim.

### Experiment

For each candidate, create a disposable experiment only after fixing its exact
profile: topology, peer/State authority, bearer binding, profile selection,
endpoint/source contacts, timeouts, resource ceilings, blocked/failure
injections, captured counters, and falsification thresholds. Experiments must
compare against the TCP/TLS baseline under the same workload and must not turn
a local proxy, foreign broker, or H3 reader into maintained fallback behavior.

### Failure scenarios

- A censor blocks UDP but not TCP, blocks TCP but not UDP, fingerprints one
  profile, or actively probes a suspected entry.
- A peer or Node advertises an unsupported/lower profile, replays stale profile
  data, or causes repeated cross-profile retries.
- QUIC path migration, NAT rebinding, MTU loss, or connection-ID behavior links
  attempts or defeats the selected binding.
- A bridge, proxy, STUN/TURN service, broker, or distribution source exposes
  the endpoint then appears in a forbidden Route/Resolution role.
- A transport Adapter leaks queues, ignores drain/resource limits, strands an
  update, or duplicates/drops bytes during Service Connection recovery.

## Findings

- **Sourced fact:** `golang.org/x/net/quic` describes itself as work in progress,
  not ready for production, with an API subject to change. Its documented
  limitations include untuned performance, no address migration, no path MTU
  discovery, and fixed non-adaptive stream windows. [Go package
  documentation](https://pkg.go.dev/golang.org/x/net/quic) (accessed
  2026-08-24).
- **Inference:** `x/net/quic` is unsuitable as the first maintained H4-2B
  Carrier Adapter. It may remain a source of implementation evidence, but its
  stated limitations directly overlap H4-2's migration, MTU, throughput, and
  maintenance criteria.
- **Sourced fact:** `quic-go` identifies itself as a production-ready pure-Go
  QUIC implementation, publishes an MIT license and a vulnerability-reporting
  policy, implements QUIC v1 plus DPLPMTUD, and aims to support the latest two
  Go releases. [quic-go source repository](https://github.com/quic-go/quic-go)
  and [security policy](https://github.com/quic-go/quic-go/security) (accessed
  2026-08-24).
- **Measurement:** the current module uses Go `1.26.6`. `go list -m` reports
  cached indirect `github.com/quic-go/quic-go v0.61.0`, whose module declares
  Go `1.25.0`; the current local toolchain therefore meets that candidate's
  declared minimum. `golang.org/x/net v0.58.0` is also indirect and declares Go
  `1.25.0`. This is toolchain compatibility evidence only, not an authorization
  to import either package.
- **Sourced fact:** the current `entry.CandidateOpener` returns a generic
  `net.Conn`, but explicitly says its implementation owns TCP/TLS. The concrete
  `route.OpenEntryAttachment` directly dials `tcp` and performs the TLS 1.3
  handshake before EntryBinding exchange. [Entry contract](../../internal/entry/contract.go)
  and [native Entry attachment](../../internal/route/entry_attachment.go)
  (accessed 2026-08-24).
- **Inference:** H4-2 currently has a useful bounded retry/exposure owner, but
  not the deep Carrier seam described in H4-2: the route code owns a TCP/TLS
  adapter and `net.Conn` is too specific to demonstrate a QUIC stream adapter.
  The seam must move the concrete dial/handshake/exporter behavior below a
  transport-neutral, authenticated ordered-carrier result before a second
  adapter can be compared fairly.
- **Sourced fact:** QUIC provides flow-controlled streams, connection setup,
  and network-path migration; its standard also explicitly covers path
  validation, PMTU discovery, connection termination/draining, version
  downgrade, and traffic analysis. [RFC 9000](https://www.rfc-editor.org/rfc/rfc9000.html)
  (accessed 2026-08-24). These are transport properties, not evidence that an
  Ardents Route remains authenticated or recoverable across a profile change.
- **Implementation fact:** cached `quic-go v0.61.0` exposes a QUIC connection
  state containing Go's `tls.ConnectionState`; that standard value has the
  exporter needed by the existing binding approach. This supports testing the
  same authentication/binding oracle on a QUIC stream, but is local-source
  inspection only and does not authorize a dependency or select the library.
- **Inference:** the first QUIC experiment must disable QUIC datagrams and
  0-RTT, expose exactly one ordered bidirectional stream to the carrier contract,
  and classify loss/migration as attachment behaviour. It must not make
  application datagrams, a new 0-RTT semantics, or transparent migration part
  of the first H4-2 claim.
- **Dependency fact:** `github.com/quic-go/quic-go v0.61.0` is already a
  reviewed indirect module because the maintained OHTTP closure imports its
  `quicvarint` package. `go mod why` confirms that the current product does
  not import QUIC transport APIs. A disposable experiment may import only this
  pinned, already reviewed version; no production package, `go.mod` role, or
  dependency selection changes as a result.
- **Measurement:** the disposable loopback baseline created separate ephemeral
  server/client certificates, required reciprocal TLS 1.3 authentication,
  compared 32-byte exporter material under the same label, and exchanged one
  exact ordered request/response on 2026-08-24. On the current Windows host it
  reported `peer-auth:ok,exporter:ok,transcript:ok` for both TCP/TLS and QUIC
  v1; client certificates from an untrusted CA were rejected for both.
- **Measurement:** five further sequential baseline runs on the same Windows
  host completed all TCP/TLS and QUIC happy/negative cases without a reported
  failure on 2026-08-24. This is a small local repeatability check, not a
  concurrency, load, latency, or availability result.
- **Measurement:** the cross-compiled Linux/amd64 baseline ran in local Ubuntu
  24.04 WSL with the same four successes. During the QUIC run, `quic-go`
  reported UDP receive-buffer tuning could raise its buffer only from 208 KiB
  to 416 KiB, below its requested 7 MiB. This is a local operational warning,
  not a throughput measurement or failure verdict; it makes UDP socket-buffer
  settings and observed values required H4-2 host-profile evidence.
  [Baseline experiment](../../../experiments/r-094-carrier-baseline/README.md)
- **Inference:** the same mutual-TLS identity/exporter and a single ordered
  byte lane are mechanically available for TCP/TLS and QUIC v1 in the two
  local platform profiles. That supports the experiment seam's common oracle,
  but says nothing about State-authorized profile choice, Carrier recovery,
  hostile network behaviour, multi-host resource bounds, or a maintained
  transport selection.
- **Dependency measurement (2026-08-24):** the exact experiment binary contains
  `quic-go v0.61.0`, `x/crypto v0.55.0`, `x/sys v0.47.0`, and the Go 1.26.6
  standard library. `govulncheck -mode=binary -show verbose` found no reachable
  or imported-package vulnerability. The build output was deleted after the
  scan; no generated artifact was retained in Git.
- **Dependency decision (2026-08-24):** R-094's direct review prerequisite is
  satisfied for disposable experiment code only. A maintained QUIC Adapter
  still requires a new dependency decision, full license/provenance inventory,
  host/fault evidence, and the normal `docs/development/dependencies.md` update.
- **Experiment result (2026-08-24):** the build-ignored two-Adapter harness
  implements one common Module oracle over actual TCP/TLS and QUIC v1 lanes.
  The Module alone validates the State-shaped offer, exact Ed25519 peer,
  TLS 1.3/ALPN state, reciprocal LegBinding, deadline, and terminal error class.
  Adapter-private results contain the raw lane, TLS state, and cleanup; the
  returned Carrier exports no `net.Conn`, QUIC type, datagram, resolver,
  fallback handle, or migration promise.
- **Measurement:** five pre-MTU-refinement Windows/amd64 runs each passed all 15
  common cases and one Linux/amd64 run in Ubuntu 24.04 WSL passed the same
  15/15. The cases cover
  happy transcript, wrong peer, wrong binding, stalled handshake, bounded
  stopped-reader backpressure, post-binding loss, pre-dial stale/unknown
  rejection, and a distinct State-shaped recovery attempt. TCP backpressure
  stopped at 79,768 bytes on Windows and 65,536 bytes on Linux; QUIC stopped at
  32,768 bytes on both. These are local budget observations, not performance or
  hostile-network measurements.
- **Negative experiment result:** treating QUIC `CancelWrite`/connection abort
  as the implementation of common graceful `Close` reset an otherwise complete
  transcript. Closing a TLS wrapper after a timed-out TCP write could also wait
  until the outer test deadline. The passing design keeps wire closure private:
  QUIC uses stream FIN for graceful completion and reset for abort; TCP bounded
  terminal cleanup closes the underlying connection. Therefore a common
  Carrier lifecycle is viable only if it specifies the result and ownership,
  not identical transport operations.
- **Operational measurement:** the Linux seam run again observed quic-go's UDP
  receive-buffer warning (208 KiB raised to 416 KiB versus 7 MiB requested).
  Actual socket buffers must be captured in NET-01A/H4-2 host evidence; no
  throughput conclusion is valid from WSL.
- **Sourced fact:** RFC 9000 requires QUIC paths to carry a UDP payload of at
  least 1200 bytes and notes that an IP MTU of 1280 leaves 1232 bytes over IPv6
  or 1252 over IPv4. Pinned quic-go v0.61.0 defaults its initial UDP payload to
  1280 bytes; its public Config permits an explicit minimum of 1200 and warns
  that an excessive initial size can cause handshake timeout.
  [RFC 9000 §14](https://www.rfc-editor.org/rfc/rfc9000.html#section-14),
  [quic-go Config](https://raw.githubusercontent.com/quic-go/quic-go/v0.61.0/interface.go),
  and [pinned default](https://raw.githubusercontent.com/quic-go/quic-go/v0.61.0/internal/protocol/params.go)
  (accessed 2026-08-24).
- **Negative MTU result:** with link MTU 1280 on both Docker namespaces and
  quic-go's default initial payload, TCP completed but QUIC timed out after
  about one second. Temporary `tcpdump` observers recorded zero UDP packets on
  both sides. This falsified the default library configuration as the H4-2
  candidate profile; it did not falsify the transport-neutral Carrier seam.
- **Candidate refinement:** the disposable QUIC profile now explicitly fixes
  `InitialPacketSize=1200`. The 15-case Windows seam oracle passed again, then
  TCP/TLS and QUIC each completed the exact 256 KiB transcript with MTU 1280 on
  both namespaces. Packet size is therefore a versioned Profile parameter that
  remains private to the Adapter, not an Interface option selected by Route.
- **Controlled fault measurement:** a checked runner built one trimmed Linux
  binary with SHA-256
  `92339074961e1f27150bf12e11a0c5c0111f1354efdceda1b8af624587fea3b9`
  and used tooling image
  `sha256:85074e6550c563477d7a1239bab07de3a18986472c08da97058f3264076c2e16`
  under Linux `6.6.87.2-microsoft-standard-WSL2`. Three complete runs each
  passed 10/10 client and 10/10 server outcomes; every client called one exact
  Adapter, retained FD count 7→7, and returned from one baseline goroutine to
  one or two within the one-second cleanup budget.
- **Selection-safety measurement:** under a 100% UDP-only `tc` drop, QUIC
  returned bounded `timeout`, called no TCP Adapter, and a separately authorized
  TCP control succeeded while the filter remained installed. The mirror
  TCP-only drop returned `timeout`, called no QUIC Adapter, and a separately
  authorized QUIC control succeeded. The first checked run counted six UDP and
  five TCP drops. This supports new-attempt replacement; it is not automatic
  fallback or a claim that application delivery survived the failed attempt.
- **Regression measurement:** after the relay/rebinding code was added, the
  complete fault runner passed 10/10 client and 10/10 server outcomes once more
  with binary SHA-256
  `0024f74f60e9d7f6adad00fc8989d68767b4f6c1a924dd9c8153e7fffaad3f15`:
  eight exact successes and the two expected selective-refusal timeouts. This
  checks that the new lab control path did not change the prior matrix; it does
  not enlarge the three-run repeatability sample for the earlier binary.
- **Loss/reorder measurement:** both namespaces used queue limit 256, delay
  20 ms ± 5 ms with 25% correlation, random loss 5%, reorder 10% with 25%
  correlation, and seed 94095. Both profiles completed the exact transcript in
  all three checked runs, and the runner required nonzero drop counters on both
  sides. One earlier nominally identical TCP run instead ended in a clean
  timeout after 28,536 ms with seven client drops. The valid conclusion is the
  predeclared “exact success or bounded classified failure” invariant, not a
  reproducible availability, latency, or TCP-versus-QUIC performance ranking.
- **Operational limit:** quic-go still reported an effective UDP receive-buffer
  increase from 208 KiB to only 416 KiB versus 7 MiB requested, while Docker
  Desktop did not expose `net.core.rmem_max`/`wmem_max` in the containers. No
  host sysctl, throughput, or supported-host claim follows from that warning.
- **NAT-port-rebinding measurement:** a second checked runner placed client,
  opaque UDP relay, and server in three namespaces across two isolated Docker
  bridges. Three complete runs each passed the unchanged-port control and a
  QUIC case in which the relay replaced its server-facing UDP socket after
  Carrier `Open`. Each run passed 2/2 client, 2/2 server, and 2/2 relay outcomes
  with binary SHA-256
  `0024f74f60e9d7f6adad00fc8989d68767b4f6c1a924dd9c8153e7fffaad3f15`
  and Ubuntu image
  `sha256:33ceb71981b602c1a7443a53469e4dba065f7503eab3078a2d7a57a2ab987517`.
  The server-facing relay tuple retained IP `172.19.0.3` and changed ports in
  all three runs; nine packets preceded each change and 253–269 followed it.
  The same QUIC Adapter and attachment then delivered the exact 262,144-byte
  transcript, retained client FD count 7→7, and joined client/relay goroutines.
  [Rebinding experiment](../../../experiments/r-094-carrier-seam-spec/README.md)
- **Inference:** the experimental quic-go profile can transparently survive
  this narrow same-IP NAT source-port change without a new Carrier attempt.
  Migration remains Adapter-private behaviour: the common Carrier Interface
  neither requests nor reports it. This result says nothing about a changed
  client address/interface, a new access network, physical-path diversity, or
  cross-profile continuation.
- **Separate-host WAN measurement:** the same trimmed Linux binary SHA-256
  `0024f74f60e9d7f6adad00fc8989d68767b4f6c1a924dd9c8153e7fffaad3f15`
  was packaged in a scratch image ID
  `sha256:ccd356a97cc7fa9a13f7a26e9a98fae76082e29a59e09d1304daa73d1a6d0efe`.
  A resource-limited server container ran on the project-operated Ubuntu
  22.04.5 LTS host at `82.23.173.198`, Docker Engine 29.4.1, kernel
  `5.15.0-185-generic`; the client ran in Docker Desktop 4.55.0/Engine 29.1.3
  under kernel `6.6.87.2-microsoft-standard-WSL2`. The hosts shared no Docker
  network, filesystem, or kernel. The remote containers were read-only,
  unprivileged UID 65532, capability-free, `no-new-privileges`, and capped at
  128 MiB, 0.5 CPU, and 64 PIDs. Existing ports 80/443 and containers were not
  changed.
- **Separate-host WAN result:** TCP/TLS and QUIC independently used the public
  endpoint on high port 47940 with fresh attempt tokens and exact matching offer
  digests. TCP/TLS returned `success`, `tcp=1,quic=0`, the exact 262,144-byte
  transcript, FD `7→7`, goroutines `1→1`, joined cleanup, and 456 ms client
  elapsed time. QUIC returned the same exact transcript and cleanup with
  `tcp=0,quic=1` and 713 ms client elapsed time. Both server outcomes passed,
  exited zero without OOM, and the checked containers/listeners were removed.
  The disposable images and transfer artifacts were also removed from both
  hosts after digest verification. The elapsed values describe two single
  attempts, not a latency or performance comparison.
- **Operational observation:** both QUIC containers again reported that the UDP
  receive buffer grew only from 208 KiB to 416 KiB instead of the requested
  7 MiB. The WAN success therefore does not erase the supported-host buffer and
  resource gate.
- **Sourced fact:** pinned quic-go v0.61.0 gives a client explicit `AddPath`,
  `Path.Probe`, and `Path.Switch` operations. `AddPath` rejects server-side use
  and a server that disabled active migration; `Switch` rejects an unvalidated
  path. The pinned integration test constructs a second `Transport`, probes it,
  and only then switches traffic. This is a QUIC-specific lifecycle surface,
  not part of the transport-neutral Carrier contract.
  [connection source](https://raw.githubusercontent.com/quic-go/quic-go/v0.61.0/connection.go),
  [path source](https://raw.githubusercontent.com/quic-go/quic-go/v0.61.0/path_manager_outgoing.go),
  and [migration integration test](https://raw.githubusercontent.com/quic-go/quic-go/v0.61.0/integrationtests/self/connection_migration_test.go)
  (accessed 2026-08-24).
- **Negative path-proof measurement:** two checked dual-homed attempts reached
  the data plane under Linux `6.6.87.2-microsoft-standard-WSL2`, tooling image
  `sha256:85074e6550c563477d7a1239bab07de3a18986472c08da97058f3264076c2e16`,
  and trimmed binary SHA-256
  `92df951fbb999893168684e01c8a4b8535b5b85a40d541fc695f591c93d0063e`.
  The stable-path control delivered the exact 262,144-byte transcript with one
  QUIC Adapter call and joined cleanup. After Carrier `Open`, the changed-route
  cell reported a route to server-A through the B gateway with client-B source,
  but the exact server-B ingress filter counted zero packets and bytes. Client
  and server returned bounded timeouts at about 5.9 seconds, the server remote
  tuple remained on client-A, and all labeled resources were removed.
- **Inference:** the route-only automatic-migration hypothesis fails its
  predeclared oracle because the intended B path was not exercised. This is not
  evidence that QUIC path migration succeeds or fails. It is evidence that a
  kernel route mutation is insufficient as this Docker/quic-go experiment and
  that explicit QUIC path management must not leak into the common Carrier seam
  without a later, separately selected product need.
- **Remaining limit:** no NET-01A run, genuine source-address/interface or
  physical access-path migration, active probing/DPI, phase-by-phase cancellation,
  CPU/memory peak, or complete server-side descriptor census has run. The
  pinned lab tooling image has an
  inspected package lock and digest but no builder on `main`, so the runner is
  reproducible on this current host rather than portable qualification
  infrastructure. These remain later capacity/path-diversity falsifiers. The
  namespace result alone did not select QUIC, a direct dependency, or a
  maintained Go Interface; ADR-0048's later product decision did.

## Options evaluated

1. **TCP/TLS only.** Lowest complexity and current selected baseline; it does
   not provide a second path against UDP/TCP-specific blocking.
2. **TCP/TLS plus QUIC v1.** QUIC offers a standards-based authenticated
   multiplexed carrier, but UDP blocking, path behavior, congestion, and
   implementation maturity must be measured. The first candidate should expose
   only a reliable bidirectional Carrier, not a new Application datagram API.
3. **Entry-only pluggable/camouflage transport.** Candidates include an
   obfs4-like profile or a newly evidenced WebTunnel successor. It may address
   first-hop blocking but has separate distribution, probing, and maintenance
   obligations and does not automatically change inter-Node carriage.
4. **Proxy/broker-assisted entry.** MASQUE, HTTP CONNECT, WebRTC, or
   Snowflake-like designs may assist specific blocked-entry cases, but introduce
   proxy/broker, source-exposure, abuse, and availability dependencies. They are
   not generic route fallbacks.

## Recommendation

The Product Owner accepted the TCP/TLS-only first profile on 2026-08-24, then
accepted completion of the full H4-2 epic on 2026-08-27. Reject `x/net/quic` as
the maintained implementation on its stated maturity/limitation profile.
Maintain the reviewed, pinned `quic-go v0.61.0` reliable-stream Adapter beside
TCP/TLS after its direct-dependency review and common-oracle evidence.

**H4-2B decision (2026-08-27):** completion of the full H4-2 epic, rather than
stopping at its TCP/TLS-first A slice, selects the already evidenced
`quic-go v0.61.0` candidate as the second maintained adjacent-Node Carrier.
ADR-0048 fixes the narrow profile: one ordered stream, TLS 1.3, the existing
ALPN and reciprocal LegBinding, 1200-byte initial packet, no datagrams, no
0-RTT, no Adapter-selected fallback, and no migration claim. H4-2D implements
each attempted profile as an authenticated signed State choice.

The disposable [two-Adapter Carrier seam experiment](../../../experiments/r-094-carrier-seam-spec/README.md)
now exercises the candidate contract in local and separate-namespace slices.
Its experiment-only dependency review is accepted. TCP/TLS and the refined
QUIC v1 candidate pass exact peer/binding, deadline, stopped-reader
backpressure, cleanup, attachment-loss, selective block, loss/reorder, MTU 1280,
same-IP NAT-port rebinding, and distinct recovery-attempt oracles.
`InitialPacketSize=1200` is part of the maintained QUIC Profile because the
library default was falsified at MTU 1280. Keep censorship-oriented adapters
separate and entry-only until their distribution and exposure model has
evidence. Do not restore H3 WebTunnel bytes or configuration by migration.

The first [local carrier baseline](../../../experiments/r-094-carrier-baseline/README.md)
and two-Adapter slice support the common TLS/binding/byte-lane oracle. Two
maintained implementations now justify the narrow `internal/route.Carrier`
Interface. The separate-host public-path slice passes the same oracle. A later
route-only dual-homed attempt failed to prove that any packet used its intended
B path, so it yields no migration verdict. Any explicit
`AddPath`/`Probe`/`Switch` or genuine access-path transition starts as a separate
QUIC-private question with complete cancellation and process/socket accounting.
Actual supported-host UDP buffers remain required before any QUIC capacity
claim.

**H4-2C decision (2026-08-27):** select no blocked-entry/camouflage profile for
the functional alpha. The evaluated families require a concrete censorship
condition and named Bridge, distribution, proxy/broker, exposure, abuse, and
operations contracts that the current product does not have. ADR-0049 makes
unavailability explicit and forbids hidden alternate contacts. QUIC diversity
does not become a censorship-resistance claim.

**Confidence:** high for the maintained two-profile functional contract and
negative alpha selection; low for any capacity, path-diversity, or censorship
claim, which this decision deliberately does not make.

## Disposition

Decided and promoted to H4-2. TCP/TLS remains the maintained first profile;
ADR-0048 additionally authorizes the pinned QUIC reliable-stream Adapter for
H4-2B. ADR-0049 closes H4-2C with no proxy, browser, Bridge, or camouflage
implementation. Signed Node Record v2 profile selection, v1-as-TCP compatibility,
unknown-profile rejection, Node projection, profile observability, and
successor drain/withdrawal close H4-2D for the maintained alpha pair. The local carrier baseline,
two-Adapter common-oracle slice, separate-namespace fault matrix, synthetic
same-IP NAT-port-rebinding slice, real separate-host WAN slice, and
experiment-only dependency review are complete. The route-only dual-homed slice
is closed as a failed path proof, not a migration result. A maintained common
Interface is now promoted because two maintained adapters create the real
boundary. Genuine path migration and host-resource evidence remain a new,
later QUIC research question and are not implied by the H4-2B selection. The
experiment harness remains evidence, not maintained Carrier code.
