---
id: R-172
title: QUIC reply budget at the UDP socket and current Node TLS compatibility
status: review
owner: Product Owner and Codex
started: 2026-10-15
reviewed: 2026-11-28
---

# R-172 — QUIC return-path budget

The branch evaluates the public single-path Go candidate for [#519](https://github.com/dianabuilds/ardents-network/issues/519).
The [October research](https://github.com/dianabuilds/ardents-network/blob/8e64f4bc8f9f65f5eb9bdcf68656fce6c1b928b8/docs/research/records/r-172-quic-return-path-budget.md)
and the [dependency finding](../../development/dependencies.md#existing-quic-finding-return-path-reply-budget) retain the historical observations.
[ADR-0122](../../adr/0122-select-tcp-only-closed-release.md) selects TCP/TLS for the closed release.

## Question and evidence

Does the candidate preserve the granted byte allowance and destination through the actual UDP write? Does a real connection using the current Node TLS functions survive an ordinary validated change of client port? The [experiment](../../../experiments/r-172-quic-return-path/README.md) specifies the hypotheses, falsification criteria, controls and reproduction commands. The protocol oracle is [RFC 9000 §8](https://www.rfc-editor.org/rfc/rfc9000.html#section-8).

**Source inspection, 2026-11-28.** The candidate is MIT-licensed `github.com/qoke/mp-quic-go v0.3.1`, commit `084c13ea3834fbe58cedb63db9208951d714f839`, module sum `h1:dz9LkG+QZr2DY/voKFZuNdlmDY4HbrKHrg2jA5DGiNM=`. The single-path server shares each incoming datagram's allowance among its probe replies, the packer includes padding in that allowance, and the probe send queue writes the packed buffer to the selected peer. [Candidate server](https://github.com/qoke/mp-quic-go/blob/v0.3.1/connection.go#L1686-L1715) and [send queue](https://github.com/qoke/mp-quic-go/blob/v0.3.1/send_queue.go#L86-L89).

**Earlier evidence, 2026-11-26.** Seven library checks and six historical adapter tests passed on Linux and macOS. The historical adapter also passed with race detection on macOS and cross-compiled for Windows amd64. Official quic-go v0.66.0 retained unconditional minimum-size probe padding. Quinn's fixed-address server setting was a viable component alternative, with a Rust integration and loss of connection continuity on NAT rebinding. [Official packer](https://github.com/quic-go/quic-go/blob/v0.66.0/packet_packer.go#L893-L913) and [Quinn setting](https://docs.rs/quinn-proto/0.11.19/quinn_proto/struct.ServerConfig.html#method.migration).

## Measurements on 2026-11-28

The source was Ardents `7d399ef3f38f84a65038fe4dddd9cc6721226ebf`. The experiment at `2c3c249e70384f67a4c43b36f1f0ae74d323a84d` ran on macOS arm64 and the project Linux x86-64 stand. Both completed with the same observations.

| Supplied probe allowance (bytes) | UDP payload delivered to selected port | Payload delivered to original port |
| ---: | ---: | ---: |
| 1 | 0 | 0 |
| 300 | 300 | 0 |
| 1200 | 1200 | 0 |
| 1500 | 1200 | 0 |

**Measurement.** Each OS passed all four cases with the native UDP writer and all four with the generic PacketConn writer. The recipient counted complete UDP payload bytes including padding. This covers a real packet buffer, send queue, destination and kernel write. The test supplies the grant and uses the upstream sealer and packet-number fixtures. It does not drive the changed-address receive logic.

**Measurement.** A separate real TLS/QUIC session ran for each of the Node and direct principals using Ardents' current `SharedServerTLS`, appropriate client configuration and `ClassifySharedTLS`. In each run the candidate's ordinary path probe on the new port sent 2400 and received 1200 UDP payload bytes in aggregate through probe completion. The client meter checked each observed receive against three times its cumulative sends. Both clients switched ports and exchanged byte-exact request and reply data. The unauthorized Node key was rejected at classification before application exchange. This uses fixture keys, a local verifier and normal padded path probes. The counts are observations of these runs, not a required wire transcript or a measurement with a small inbound datagram.

**Measurement.** The separate existing Linux process test `TestRouteRefusesSignedQUICBeforeEffects` passed for the actual `ardents-next` CLI built from the base revision. It authenticated the signed QUIC profile, observed refusal, checked spend and socket effects, and checked for TCP fallback. A separate Linux/amd64 production dependency listing for `./cmd/ardents-next` excluded both the candidate and official quic-go. The candidate was present only in the research builds. No Windows execution or installed positive QUIC Node/Service run was made.

**Environment.** The first offline stand process run lacked the required x/sys module. The first local runner draft attempted a Go overlay inside the module cache and Go rejected it. The first online stand candidate run stopped during module retrieval; its captured error body was unavailable. The successful process run used a writable module cache and the successful candidate run used a verified temporary source copy and writable GOPATH and module cache. These earlier setup failures supplied no protocol results.

## Historical 32-byte input

**Source inference.** For a single 32-byte UDP payload attributed to the established connection on a previously unseen address, the reviewed single-path candidate grants its immediate path-probe replies at most **96 UDP payload bytes in total** before address validation. A record for the original address does not validate the new one: the lookup compares the peer address and local path. The native and generic readers slice the input to the bytes actually read. The grant uses the length of the received packet data. The packer includes header, authentication overhead and padding in the limit, and the sender deducts each packed length before considering another reply. A reply that cannot fit can be withheld. The probe writer passes the selected destination and requests zero GSO segment size. This establishes a bound for the inspected branch. It does not determine whether a particular datagram provokes any output. [Input size](https://github.com/qoke/mp-quic-go/blob/v0.3.1/connection.go#L76), [path validation lookup](https://github.com/qoke/mp-quic-go/blob/v0.3.1/path_manager.go#L285-L295), [shared grant](https://github.com/qoke/mp-quic-go/blob/v0.3.1/connection.go#L1837-L1869), [probe writer](https://github.com/qoke/mp-quic-go/blob/v0.3.1/send_conn.go#L157-L165).

**Measurement on Linux x86-64 and macOS arm64, experiment commit `1b1069d2c40549d36417be1e1e2303205aad8b44`.** For a separately supplied 96-byte grant and a response that fits, the selected loopback socket received one 96-byte UDP payload and the original socket received zero. The native UDP and generic PacketConn variants passed. The module's unchanged server-budget and shared-allowance regressions passed in the same run. Those input-side tests use a mock packer. The socket fixture uses the actual packer and UDP writer with a test sealer. Neither receipt is a direct measurement of a single authenticated 32-byte input through the entire live server.

**Decision for #519.** The candidate provides source and component evidence for the specific padding correction. The recorded ordinary migration exchanged full-size probes. The earlier 32-byte observation belongs to v0.62.0. A matching capture for the candidate was absent from the retained R-172 runs, and the public candidate repository's Actions artifact list returned zero entries on 2026-11-28. Exact prevalidation UDP output for that live case remains unmeasured. Keep future QUIC admission open under the composite evidence criteria below.

## Decision

Prefer the Go candidate for further qualification. Its public fork derives from quic-go v0.63.0. The inspection on November 26 found that the later upstream send-queue wakeup correction was absent from both corresponding send branches of this tag. Establish the applicable backports and ongoing maintenance before adoption. [Upstream correction](https://github.com/quic-go/quic-go/pull/6223).

The issue stays open. The experiment exercises one ordinary migration and the output stage for supplied budgets. It leaves causal coverage of the complete receive-to-send budget under cancellation, repeated or competing paths, each admitted caller and Windows native execution open. The signed production Node duty still requires pre-effect refusal. Positive Node process/Service execution depends on a product decision to reintroduce the Carrier and a bound integration candidate under the [existing acceptance method](../../development/testing.md#quic-return-path-reply-budget-evidence).
