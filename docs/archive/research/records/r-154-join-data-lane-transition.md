---
id: R-154
title: Activate the admitted Rendezvous data lane after JOIN
status: completed
owner: Product Owner and Codex
started: 2026-09-09
reviewed: 2026-09-09
---

# R-154 — How does an admitted Rendezvous JOIN become a data lane?

## Decision this unlocks

Complete the exact closed JOIN-to-data grammar needed by the protected text
journey without substituting a self-hop or unframed byte stream. The question
was registered in [the research queue](../questions.md); its approved answer is
[ADR-0083](../../adr/0083-activate-joined-rendezvous-data-lane.md).

## Current contract

[ADR-0081](../../adr/0081-select-closed-protected-service-contract.md), the
[text workload](../../product/protected-service-workload.md), the
[threat model](../../security/threat-model.md#closed-successor-claim-contract)
and [protocol owner](../../technical/protected-route-protocol.md) retain separate
Source/Responder authority, receiver-local admission and framed bounded lanes.
[CONTEXT.md](../../../CONTEXT.md) owns Service, Instance and Route terminology.
The open detail was lane allocation/activation after the already specified JOIN.

## Hypotheses

- **H1:** One dedicated admitted channel per side, with JOIN/RESULT on local lane
  1, can activate one bounded framed data lane without new authority.
- **H2:** Existing OPEN can establish the data lane without adding a self-hop
  exception or changing its next-Node selection semantics.
- **H0:** Neither proposed transition meets the existing ownership contract.

## Evaluation criteria

Require opposite-side secret/context/profile pairing before any Service bytes,
no third-side replacement, unchanged caller authority and receiving validation,
fixed operation/result sizes, original receive credit and aggregate reserves,
finite unmatched setup and independently bounded data lifetime. Preserve separate
channel nonces, directional EOF and joined cancellation. No new operator role,
library, license or maintenance dependency is introduced. The trusted user flow
continues to use the same Target Link and publication outcome.

Protected information and adversary remain the five-part claim in the threat
model: this amendment introduces no new anonymity measurement. Rendezvous still
observes its matching JOIN material and timing; endpoint compromise and retained
traffic correlation are not addressed by choosing a lane number.

## Evidence plan

### Primary sources

Repository sources inspected 2026-09-09: the current protocol owner's OPEN,
HELLO/ADMIT/ACCEPT, lane framing, JOIN body and terminal operation bounds;
the current workload and threat model linked above. The Product Owner approved
the complete eight-point JOIN-to-data proposal in the active implementation
conversation on the same date. These are contract/design evidence, not tests.

### Experiment

No experiment was run for this decision. Before acceptance, use real Node,
Source and Responder owners on TCP/TLS and QUIC; capture fixed RESULT ordering,
credit and lifetime accounting, then exercise the failure cases below. This
record does not convert existing component fixtures into network evidence.

### Failure cases

Before implementation acceptance, falsify H1 if data can precede pairing, a
wrong local nonce/lane activates data, duplicate/third sides replace ownership,
context/profile mismatch pairs, setup timeout permits a late join, matching
truncates an otherwise admitted data lifetime to setup time, credit creates
extra parent budget, reverse traffic dies on directional EOF, or cancellation
leaves an owned reader/writer alive. Test exact size/padding refusals as well.

## Findings

- **Sourced fact:** OPEN selects a real next Node. It is not a defined local
  Rendezvous allocation, so H2 needs an additional unchecked self-hop exception.
- **Sourced fact:** The accepted contract already fixes class-2 data admission,
  JOIN matching fields, odd Endpoint lane IDs and common framed data/credit.
- **Inference:** A dedicated local lane 1 and matching fixed RESULT make the
  activation boundary explicit while preserving those existing owners.
- **Assumption:** The implementation can join both sides within the original
  resource/lifetime bounds; the proposed falsification tests must establish it.
- **Measurement:** None. No latency, capacity or anonymity result is claimed.

## Options

1. Dedicated admitted JOIN channel and explicit lane-1 activation: fits the
   selected boundaries; requires a real bounded pairing/bridge implementation.
2. Self-OPEN at Rendezvous: rejected because it changes OPEN's authority and
   next-hop semantics and adds an avoidable exceptional allocation path.
3. Raw Service bytes after RESULT: rejected because it bypasses selected lane
   framing, receive credit, terminal semantics and aggregate accounting.

## Recommendation

Choose option 1, as approved by the Product Owner. Confidence is in contract
consistency only. The strongest remaining risk is implementation error when
coordinating pairing, deadline transitions, credit and two-sided shutdown.

## Disposition

Decided through ADR-0083; the current protocol owns the full transition.
Implementation and all acceptance remain in the existing C0 issue sequence.
No experiment artifact was produced, no dependency was selected, and this
completed clarification creates no additional active research or implementation.