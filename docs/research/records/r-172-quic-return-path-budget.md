---
id: R-172
title: QUIC reply budget at the UDP socket and current Node TLS compatibility
status: decided
owner: Product Owner and Codex
started: 2026-10-15
reviewed: 2026-11-28
---

# R-172 — QUIC return-path budget

The branch evaluates the public single-path Go candidate for [#519](https://github.com/dianabuilds/ardents-network/issues/519).
The [October research](https://github.com/dianabuilds/ardents-network/blob/8e64f4bc8f9f65f5eb9bdcf68656fce6c1b928b8/docs/research/records/r-172-quic-return-path-budget.md)
and the [dependency finding](../../development/dependencies.md#existing-quic-finding-return-path-reply-budget) retain the historical observations.
[ADR-0136](../../adr/0136-return-quic-from-patched-official-source.md) returned QUIC to the Carrier set of the closed release on 2026-11-28 and supersedes [ADR-0122](../../adr/0122-select-tcp-only-closed-release.md) for the Carrier set.

## Question and evidence

Does the candidate preserve the granted byte allowance and destination through the actual UDP write, from a small packet that arrives from a new address up to the server's reply? Does a real connection using the current Node TLS functions survive a validated change of client port? Can Ardents adopt the candidate under the dependency acceptance rule? The [experiment](../../../experiments/r-172-quic-return-path/README.md) fixes the hypotheses, falsification criteria, controls and reproduction commands. The protocol oracle is [RFC 9000 section 8](https://www.rfc-editor.org/rfc/rfc9000.html#section-8): an endpoint that receives packets from an address that is not yet validated MUST limit the data it sends to that address to three times the data received from it. Section 8.2.1 permits an unexpanded PATH_CHALLENGE only when that limit forbids 1200 bytes and requires a second expanded validation.

## Source identities, inspected 2026-11-28

**Candidate.** `github.com/qoke/mp-quic-go v0.3.1`, module sum `h1:dz9LkG+QZr2DY/voKFZuNdlmDY4HbrKHrg2jA5DGiNM=`, commit `084c13ea3834fbe58cedb63db9208951d714f839`. The commit is an unsigned merge of `AeonDave/mp-quic-go` v0.1.3 (`4dfa088333223f33487db14afba95f69d16a46d0`) and official quic-go v0.63.0 (`9d085cc690f7c96451e8ae5659eb0e64671da47a`), authored by the repository owner on 2026-10-11. It changes 300 files with 68624 added and 14454 removed lines and carries no pull request or review history. Outside `http3`, `interop`, `integrationtests` and `example`, it keeps 75 Go files identical to v0.63.0, changes 255 and adds 80, among them multipath, preferred address, zero-RTT replay and address discovery code; it drops none. The MIT license file is byte-identical to the one of quic-go v0.63.0.

**Correction mechanism.** The single-path server grants the probe replies to a datagram from an unseen address three times that datagram's size, the packer counts header, authentication data and padding against the grant and pads to the smaller of the grant and 1200 bytes, and the server does not switch the path before validation. [Budget and switch](https://github.com/qoke/mp-quic-go/blob/v0.3.1/connection.go#L1686-L1753), [shared grant](https://github.com/qoke/mp-quic-go/blob/v0.3.1/connection.go#L1837-L1914), [probe padding](https://github.com/qoke/mp-quic-go/blob/v0.3.1/packet_packer.go#L1006-L1047), [validation states](https://github.com/qoke/mp-quic-go/blob/v0.3.1/path_manager.go#L268-L315).

**Official releases.** The controls are `github.com/quic-go/quic-go` v0.62.0 (`h1:ZHDjCk5OacATwGvs8PWE97CTvX7AqZiVoW7++ZOXTf8=`, the version of the historical finding), v0.63.0 (`h1:LIFGHI4PFUhhw2dDD1ARHdCff143ffMHwZtbnbuJ78A=`) and v0.66.0 (`h1:NCsBBGZrI2HZC6aKt4I89T5i4Crt39EYNqkh0w2Wgmg=`, released 2026-11-24). Upstream `master` at `220f6c06007ad87ed8efa7746052f6aab8137fa6` (2026-11-27) still pads every probe to 1200 bytes without a budget argument in [packet_packer.go](https://github.com/quic-go/quic-go/blob/220f6c06007ad87ed8efa7746052f6aab8137fa6/packet_packer.go#L893-L915). No upstream pull request or issue about this probe budget exists; the latest pull requests mentioning amplification date from 2022.

## Measurements on 2026-11-28

**Method.** One real TLS 1.3/QUIC v1 session per case, ALPN `ardents-route-v3`, server with the current `SharedServerTLS`, client with `NodeClientTLS` or `RoleClientTLS`, both classified by `ClassifySharedTLS`. A user-space NAT owns two loopback sockets in front of the implementation's own client. After a quiet handshake and echo it moves the client to the second socket and either withholds the client's later packets or keeps forwarding. Delivered packets are unchanged client packets. The relay reads the server's datagrams at both sockets and counts UDP payload bytes. Each endpoint's tracer reports the sizes and frame types it received and packed. Test source: Ardents `cd86ebe5b` on `codex/519-quic-return-path`, the last commit that changes the session test (the cases were introduced by `7b18c4dc7` and `6cab7bb35`); the Linux evidence for the candidate and the patched release ran at that commit and at `15a41ced5`. Go 1.27.4. macOS arm64 (darwin 27.0.0) locally; Linux x86-64 through [standctl](../../../scripts/standctl/README.md) with the `online` service as container user 10001: lab-1, kernel 6.8.0-149, and lab-2, kernel 5.15.0-203. Both platforms produced the same sizes.

| Case | First packet from the new address | Server payload to the new address before validation | Limit |
| --- | ---: | --- | ---: |
| Withheld replies, 32-byte packets one at a time | 32 | 96, then 96 + 96, then 96 × 3, then 96 × 4 | 96, 192, 288, 384 |
| Withheld replies, one larger packet | 404 | 1200 | 1212 |
| Ordinary rebinding, Node client and direct role client | 32 | 96, then 1200 only after a PATH_RESPONSE | 96 |
| Idle connection with a one-second keep-alive | 25 | 75 | 75 |

**Measurement.** The candidate passed every case on macOS arm64 and on both Linux stands, with its native UDP writer and with its generic PacketConn writer, including five consecutive macOS runs and three Linux runs with the race detector, which reported no race. The server tracer shows `received 32 [StreamFrame]` followed by `sent 96 [PathChallengeFrame]`, and each `sent 1200 [PathChallengeFrame]` after a `received 1200 [AckFrame PathResponseFrame]`. During the withheld phase the server sent 2.4 to 2.8 KB to the original mapping only: acknowledgements, echo replies and retransmissions on the validated path. After forwarding resumed the server moved to the rebound mapping and the application exchange continued.

**Measurement, controls.** Each official release replied with 1200 bytes to the 32-byte packet and to the 25-byte keep-alive, on both platforms with both writers, for both client principals, and in the ordinary rebinding. Each passed the 404-byte case. The control therefore separates the historical behavior from the candidate's.

**Earlier evidence, 2026-11-26 and the morning of 2026-11-28.** Seven library checks and six historical adapter tests passed on Linux and macOS. A supplied grant reached the selected loopback socket with the real packer, send queue and writer. Those checks used the library's test sealer and mock packer. [Official packer](https://github.com/quic-go/quic-go/blob/v0.66.0/packet_packer.go#L893-L913), [Quinn setting](https://docs.rs/quinn-proto/0.11.19/quinn_proto/struct.ServerConfig.html#method.migration).

**What is real and what the test supplies.** Real: both QUIC endpoints, their keys, the handshake, the server's receive path, budget, packet packer, send queue and UDP writer, and the kernel's delivery to the relay's sockets. Supplied by the test: self-signed Ed25519 certificates, a local Node key verifier, an echo application, the relay that forwards, withholds and releases datagrams without changing them, and a tracer that only observes. The relay hides the client's UDP socket type, so only the server's two writers are compared. Every socket is on loopback. This is not a capture on a physical interface, a kernel NAT, a spoofed third-party address or an installed Node.

**Candidate observations outside the budget.** When a withheld PATH_RESPONSE let the second challenge expire, the server dropped its path record with the validated flag and answered later packets with unexpanded probes of at most three times their size again. With path MTU discovery disabled, the server sometimes sent one 1366-byte MTU probe after a validated migration, because the migration resets the probe timer without checking the setting. Neither adds bytes before validation.

**Environment failures, no protocol result.** The first `go mod download` of v0.62.0 failed once and succeeded on retry. Runner drafts used an invalid `go fmt -l`, a wrong test package path, a tracer `Close` without an error result, a deadline that could close a channel twice, an echo handler that waited on streams the client never closed, and a fixed packet index that one macOS race run found bundled with another frame. The test now delivers only packets of the intended size. One macOS race run failed on the earlier index assumption after observing 96 bytes within the limit; it is kept as a failed attempt. One draft check in the keep-alive case compared all bytes sent after validation with the few bytes of the keep-alive; it failed first against the patched release and was removed, since no limit applies after a PATH_RESPONSE. The upstream test adaptation needed a mock that reports the real probe length.

## Applicability to the current owners

| Path | Evidence | Still open |
| --- | --- | --- |
| Server with `SharedServerTLS`, Node client with certificate | Live sessions, both writers, macOS and Linux | Installed Node and Service process |
| Server with `SharedServerTLS`, certificate-free direct role client | Live ordinary rebinding, both writers, macOS and Linux | Role-specific admission and spend under QUIC |
| ALPN `ardents-route-v3`, TLS 1.3, session tickets disabled, X25519MLKEM768 then X25519 | Negotiated in every session | None for this mechanism |
| Native and generic server writers | Measured on macOS and Linux | Windows writer: the test cross-compiles and vets for windows/amd64 and windows/arm64 and was not executed |
| Retired Ardents keep-alive of one second | Measured: 25 bytes in, 75 bytes out | Retired idle, handshake and stream limits, 32 KiB windows, `InitialPacketSize`, enabled path MTU discovery and `DialAddr` client sockets were not varied |
| Connection close | Client close reaches the server, listener, transports, echo goroutine and relay readers join in every case | Long-running resource return |
| Multipath, preferred address, observed address, 0-RTT | Not used; Ardents disables 0-RTT | Not exercised if a future selection enables them |

The product has no QUIC caller to admit: ADR-0122 removed the adapter and the architecture guard excludes `github.com/quic-go/quic-go` and the retired package path. That guard names no other QUIC module, so a differently named module would not be excluded by it.

## Maintenance review

The candidate fails the dependency acceptance rule for want of a credible maintenance and security-fix path; the register records the [disposition](../../development/dependencies.md#existing-quic-finding-return-path-reply-budget).

- **Provenance.** `qoke/mp-quic-go` is a fork of `AeonDave/mp-quic-go`, itself a fork of quic-go. The parent received no push after 2026-01-06. The candidate repository was created on 2026-09-26, has one star, no fork, no release, no open issue, and its last push is the candidate commit of 2026-10-11.
- **Upstream distance.** Upstream is 284 commits ahead of the candidate's base. Among them: the send-queue wakeup [#6223](https://github.com/quic-go/quic-go/pull/6223), absent from the candidate at `connection.go` lines 3430 and 3498, which fixes a connection that stops sending until the next received packet; the QUIC v2 key-update label (#5973); quadratic copying in `TryWriteAll` (#6042, #6043); the macOS IPv4 source address (#6146); a send-stream leak (#6157); the out-of-order CRYPTO bound (#6265); a 0-RTT transport-parameter data race (#6297) and reduced 0-RTT limits (#6291); the AEAD integrity limit across keys (#6340, #6341); and a path-switch race (#6368).
- **Advisories.** The Go vulnerability database lists eight advisories under `github.com/quic-go/quic-go`. The distinctive code of each of the six transport fixes is present in the candidate: the deferred handshake completion of GO-2023-2160, the PATH_RESPONSE and path limits of GO-2024-2459, the control-frame limit of GO-2024-2682, the Linux `IP_PMTUDISC_PROBE` mode of GO-2024-3302, the lost-probe guard of GO-2025-3735 and the initial-key drop of GO-2025-4017. The other two, GO-2025-4233 and GO-2026-5676, affect only `http3`, which Ardents would not import; the fork's `http3` tree was not inspected. This is a source comparison, not a rerun of the upstream regressions. The database lists nothing under the fork's module path, and the scanner matches by module path, so a future quic-go advisory would not flag the fork. `govulncheck` v1.8.0 with a database last modified 2026-11-24T15:56:00Z reported no finding for the test module, which selects Ardents' `x/crypto` v0.59.0, `x/net` v0.62.0 and `x/sys` v0.50.0.
- **The fork's own closure.** Its module requires `x/net` v0.56.0, which [GO-2026-5026](https://pkg.go.dev/vuln/GO-2026-5026) affects. The repository's scheduled vulnerability workflow has failed every day since 2026-11-24 without a fix push. Its security policy describes the project as under active development.
- **Team capacity.** Adoption would make the one-human-and-Codex team the maintainer of a fork that must track and backport upstream transport corrections. Repository instructions select no private fork.

## Decision

The candidate corrects the specific finding: in a live authenticated session its server answers a small packet from a new address with at most three times the received payload, on both mandatory executed platforms with both writers, where every official release through v0.66.0 and `master` answers with 1200 bytes. It is not a library Ardents can select today. It is a single-author fork of a fork with no change since 2026-10-11, 284 upstream commits behind, with a known advisory in its own dependency closure and a failing vulnerability scan, and no scanner would report a future quic-go advisory against it.

The issue stays open. For #519, the composite evidence now has the actual-component causal regression, the independent oracle and a discriminating control, with exact source and module identities. It lacks a supported source, Windows execution, the retired configuration variants, output paths other than the probe, an Ardents artifact and any installed Node or Service execution. The finding is resolved for no admitted use, because no admitted QUIC use exists.

## Prepared correction on the official release

**Correction.** [`quic-go-v0.66.0-probe-budget.patch`](../../../experiments/r-172-quic-return-path/quic-go-v0.66.0-probe-budget.patch), SHA-256 `f5573d4c91a971b46a96135e72cae968f90c8fe18352f38f5da9a62c9dde0b9d`, applies to official quic-go v0.66.0 at commit `d08d86670a1b480bb81a216fb59f6a2b23e8d444`. It changes three production files with 104 added and 18 removed lines and adds or adapts the tests. Since v0.64.0 the official server switches paths only after validation; the probe's unconditional padding was the remaining gap. The patch pads the server's path probe to the smaller of 1200 bytes and three times the received datagram. A reply to the client's PATH_CHALLENGE is packed into the same probe. The PATH_RESPONSE to an unexpanded probe marks the address validated, which lifts the limit for that address, and the next packet on the path carries a second PATH_CHALLENGE in a datagram of 1200 bytes. The path is switched only after that second challenge is answered, as before. The client side is unchanged.

**Measurement.** With the patch applied, upstream's own tests pass on macOS arm64: the `quic` package, every package under `internal` and the `integrationtests/self` suite, which includes the NAT rebinding tests over real sockets. The same patch applies to upstream `master` at `220f6c06` with an offset of one line and passes the same suites. The Ardents session test passes every case against the patched release on macOS arm64 and on the Linux stand, with both writers and with the race detector: 32 bytes in, 96 bytes out, cumulative 96, 192, 288 and 384; 404 bytes in, 1200 bytes out; 96 bytes as the first reply to 32 and 75 bytes as the first reply to the 25-byte keep-alive, with 1200 bytes only after a PATH_RESPONSE. The unpatched official releases keep failing with 1200 bytes. The runner applies the patch to a verified copy of the official module with its own context-exact applier, so a run also proves the identity of the base.

**Upstream observations outside the finding.** The official client sends its PATH_RESPONSE on the active path in a regular 37-byte packet, not expanded to 1200 bytes as section 8.2.2 requires. After `Path.Switch` to a path whose socket is a wrapped `net.PacketConn`, official v0.66.0 keeps ECN enabled and panics in `basicConn.WritePacket`, with or without the patch. The Node TLS check with the client's probing API therefore runs against the candidate only. Ardents' sockets are `*net.UDPConn`, and the retired adapter did not use that API.

## Product Owner decisions, 2026-11-28

The Product Owner first chose to keep TCP/TLS as the Carrier, to select neither the candidate nor another fork, and not to ask upstream quic-go. Later the same day the Product Owner stated that a need for QUIC exists, preferably in the nearest closed release, and allowed the correction to be offered upstream. The Product Owner then accepted [ADR-0136](../../adr/0136-return-quic-from-patched-official-source.md): QUIC returns to the Carrier set of the nearest closed release together with TCP/TLS, and the correction comes from official quic-go v0.66.0 with the retained patch inside the repository until an official release carries it. The multipath candidate, a socket-level counter of first party and a weaker refusal stay rejected.

The upstream project's security policy asks for exploitable production issues to be reported privately, and its AI policy asks the submitting person to review the text and to understand every line of a proposed change. The private report is drafted and waits for the Product Owner; nothing has been submitted.

## Disposition

- Question state: answered for the selected source. #519 stays open for the staged return: the source (#1080), the adapter and Carrier selection (#1081), and the both-Carrier qualification (#1082).
- Accepted follow-ups: ADR-0136; #1080, #1081 and #1082 in that order.
- Documents changed: this record, the experiment README, ADR-0136 and its index, the dependency register, the testing owner, the Route migration contract, the execution owner and every owner that named ADR-0122.
- Experiment code: retained. `run.py patched` is the regression for the official release with the patch; `run.py return-path --module candidate` and `run.py controls` keep the candidate and the unpatched controls. Everything lives outside the product module graph and is rendered only when the runner executes. #1081 moves the session test into the checked repository profile.
