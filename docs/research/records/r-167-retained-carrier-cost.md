---
id: R-167
title: Who pays for a retained Carrier after its last child joins?
status: open
owner: Product Owner and design assistant
started: 2026-09-23
reviewed: 2026-09-23
---

# R-167 — Retained Carrier future traffic

## Decision this unlocks

Issue [#262](https://github.com/dianabuilds/ardents-network/issues/262) needs one future-cost and retirement rule for an actual-work Carrier after its last child Join and the related forwarding-parent host reservation can be released. This record compares policy candidates; it does not change the pool, provider tariff, selected Carrier set, or qualification status. The separate [#84](https://github.com/dianabuilds/ardents-network/issues/84) parent-refill calculation is not this decision.

## Current contract

The [C0 scope](../../product/scope.md) requires finite budgets, bounded lifetimes, joined cleanup, and explicit overload. The [threat model](../../security/threat-model.md) includes malicious peers and Sybil actors; payload encryption supplies no anonymity claim. The [Network/Node owner](../../technical/network-route-node.md#adjacent-node-carrier-profiles) selects exact State-pinned TCP/TLS-v2 or QUIC-v2 Carriers with no fallback. [Hosting qualification](../../development/privacy-qualification.md#counting-useful-work-and-hosting-cost) requires work and termination to be reserved, low-watermark refusal, and separate observed tx/rx accounting. The [text workload](../../product/protected-service-workload.md#acceptance-and-limits) has a 512-byte/64 KiB exchange at p95 ≤3 s cold and ≤1 s warm in its declared envelope. The current 120-second pool retention is implemented behavior, not proof that its future traffic is paid or that either latency gate has qualified.

## Hypotheses and falsification

- **H1, retain with separate coverage:** a pool-owned, finite host-period reservation remains attached to each idle Carrier through keepalive and final close, including any bounded post-close traffic, without borrowing a released parent's budget. An actual packet after that coverage ends, or a reservation that cannot fit the selected provider period/low watermark, falsifies this policy.
- **H2, end retention before release:** the last useful lease closes and joins its Carrier while an exact covering reservation is still live, with any required bounded tail covered until no further outgoing traffic can occur. A later packet without coverage, a close result lost from the owner, or a selected warm-latency miss falsifies it.
- **H0:** neither candidate fits the declared cost and latency limits; change the accepted owner contract explicitly before runtime work. Disabling QUIC keepalive while silently retaining a connection is not assumed safe: the peer, retransmission and close paths still need a bound.

## Evaluation criteria

Every outgoing byte from a locally retained Carrier, including TCP probes where supported, QUIC PING/ACK/retransmission and close traffic, must be assigned to a live finite reservation or shown impossible after retirement. Actual interface counters remain spent. An untrusted peer may send packets during idle and close. State/profile invalidation, low watermark, deadline, Stop/Join, failed close and concurrent exact-key borrow must have one terminal owner and no reuse of an invalid incarnation. Preserve both selected Carriers and the text workload's cold/warm gates; no speculative dial, peer fallback, changed authority or new privacy claim is allowed. The resource owner remains the one actual host-period ledger, including other local processes and counted directions.

## Evidence plan

### Primary sources

All first-party source observations below use maintained `dev@a9e81ebcc1bfc2bf7a4bef40d2dc0fd65c334995`, accessed 2026-09-23: `internal/route/closed_carrier_pool.go`, `closed_node_carrier.go`, `node_carrier_quic.go`, `closed_carrier_retirement.go`, `internal/node/closed_forwarding_link.go`, `closed_forwarding_listener.go`, `closed_forwarding_admission.go`, and `internal/resource/hosting_ledger.go`. The pinned Go 1.26.8 `net.Dialer.KeepAlive` documentation and pinned quic-go v0.62.0 `interface.go`, `connection.go`, `client.go`, `closed_conn.go`, and `transport.go` are dependency primary sources, accessed from the installed exact-version source on 2026-09-23. The accepted [ADR-0048](../../adr/0048-maintain-tcp-and-quic-carriers.md) retains the paired Carrier choice; [ADR-0085](../../adr/0085-bound-forwarding-replenishment.md) governs parent refill only.

### Named falsification experiment before selection

On the fixed Ubuntu host/profile with a real selected Node pair and the shared installed hosting ledger, establish one actual-work Carrier for each profile, Join its last child, and retain the Node process for more than the current 120-second window. Capture the exact last lease release, parent reservation release, pool entry/close, both endpoint interface tx/rx and packet timestamps, and the ledger's observed/reserved/remaining values. Repeat for quiet peer, peer packets after last Join, State invalidation, low watermark, pool Reap, failed close and a second exact-key borrower. Attribute TCP probes, QUIC PING/ACK/retransmission, close and unrelated host traffic separately. A packet after all matching coverage is released is a red result even if the total period remains under quota. Run the text cold/warm latency cohort separately for either proposed early-close policy; a local loopback timing pass is not qualification. Capture build, configuration, commands, exit codes and counter reconciliation outside the repository.

### Failure scenarios

A hostile peer prolongs idle exchange, sends packets during close, or goes silent; a second parent borrows the exact Carrier while the first releases; State changes while Reap or a late reader invalidates an old incarnation; the period reaches low watermark or the host ledger cannot persist; physical Close fails after a parent cleanup decision. None may silently create unpaid future work or replace the original cleanup result.

## Findings

- **Sourced fact:** `ClosedCarrierLease.Release` leaves an actual-work entry idle for 120 seconds after its last active lease; `Reap` later closes it. The pool has at most 32 entries and keys them by exact Network/Profile/Node/peer/Carrier facts. Unused entries close at last release.
- **Sourced fact:** the outgoing QUIC adapter sets `KeepAlivePeriod=1s` and `MaxIdleTimeout=5s`. Pinned quic-go's `nextKeepAliveTime` uses the larger of its configured interval and 1.5×PTO and queues PING on idle expiry. Thus an idle retained QUIC connection can emit traffic; 120 exact one-second packets is not an established bound.
- **Sourced fact:** the outgoing TCP/TLS adapter uses `net.Dialer{}`. Pinned Go 1.26.8 documents that a zero `KeepAlive` enables probes, currently at a 15-second default, where supported by the protocol and OS. Therefore TCP retention is not demonstrated cost-free either. This is a conditional source fact, not measured target-host traffic.
- **Sourced fact:** `closedForwardingLink.stop` retires its child session before `lease.Release`. The ordinary parent handler joins links before `ClosedForwardingChannel.Cancel`, which releases that parent's host reservations. `Hosting.Reserve` and `HostingReservation.Release` are tied to those admitted parent envelopes; the reviewed pool entry has no separate host reservation field.
- **Sourced fact and ordering inference:** quic-go `Conn.CloseWithError` waits for the connection context. In `connection.go`, `conn.run` installs a closed packet handler able to answer later peer packets and defers context cancellation until return. In `transport.go`, that handler queues a CONNECTION_CLOSE copy for selected later packets. The `client.go` single-use dial goroutine calls `Transport.Close` **after** `conn.run` returns; `Transport.Close` closes its owned UDP socket and joins its listener. Therefore the context can release `CloseWithError` before that separate transport close has joined. This adapter exposes only the `Conn`, not the transport join, and its `quicNodeCarrier.Close` returns after `CloseWithError`. Returning from that call alone is not a proven no-future-byte witness. The actual size/duration of the race and kernel tail remain unmeasured.
- **Inference:** already incurred traffic will be charged on later host observation, but later observation is not a reservation for future packets. A shared host's remaining balance or another parent's reserve cannot silently pay an idle Carrier's future work. Immediate close may remove most idle cost, yet must cover close/tail traffic and test the 1-second warm gate.

## Options

1. **Retain useful Carriers with a pool-owned host reservation.** Supports current reuse intent if a finite per-profile tx/rx/time bound and shared-ledger owner are accepted. It adds up to 32 concurrent idle obligations and must release them only after a proven traffic quiescence boundary. The current source and provider profile do not supply those numbers.
2. **Close on last useful lease under its still-live parent coverage.** Removes the 120-second warm Carrier reuse path. The current QUIC adapter cannot treat `CloseWithError` return as a joined UDP transport boundary; a bounded adapter-level join or separately covered tail is needed before the last matching reserve is released. It also needs no late session-reader invalidation of a replacement. Cold and warm workload gates must pass on both Carriers; no such result is yet available.
3. **Retain while disabling keepalive.** Not selected: TCP and QUIC behavior differs, remote traffic and close remain, and a QUIC idle timeout can invalidate apparent reusable state. It needs its own explicit proof and performance comparison, not a hidden local switch.

## Recommendation

Choose neither policy yet. First run the named traffic and warm-latency comparison, then select the smallest policy that proves future-cost ownership on the fixed profile. Confidence is high that the current pool has a future-cost accounting gap, and low in a numeric idle/close reserve or early-close latency result. The strongest argument against deferring a choice is that ordinary parent traffic allowances may be large enough in practice; that does not prove who owns later packets after their reservations are released.

## Disposition

Open draft for #262. No pool policy, numeric reserve, runtime patch, ADR or qualification verdict is accepted. No experiment code was created. A later accepted decision belongs in the current Network/Node and hosting qualification owners, with one bounded implementation card and both-Carrier behavior oracles.
