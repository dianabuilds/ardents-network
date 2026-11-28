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

## Decision

Prefer the Go candidate for further qualification. Its public fork derives from quic-go v0.63.0. The inspection on November 26 found that the later upstream send-queue wakeup correction was absent from both corresponding send branches of this tag. Establish the applicable backports and ongoing maintenance before adoption. [Upstream correction](https://github.com/quic-go/quic-go/pull/6223).

The issue stays open. The experiment exercises one ordinary migration and the output stage for supplied budgets. It leaves causal coverage of the complete receive-to-send budget under cancellation, repeated or competing paths, each admitted caller and Windows native execution open. The signed production Node duty still requires pre-effect refusal. Positive Node process/Service execution depends on a product decision to reintroduce the Carrier and a bound integration candidate under the [existing acceptance method](../../development/testing.md#quic-return-path-reply-budget-evidence).
