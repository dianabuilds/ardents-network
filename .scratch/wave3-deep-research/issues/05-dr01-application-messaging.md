# DR-01: Design bounded Application Messaging

Status: ready-for-agent
State: closed
Labels: ready-for-agent
Research class: R2

## Parent

`../PRD.md`

## What to build

Select a bounded Application Messaging model over the accepted Channel Grant
authority. Define addressing, one-to-one and group membership lifecycle,
online/offline delivery, acknowledgement, deduplication, ordering, expiry,
receive behavior, backpressure, quotas, malicious authorized-member handling,
small payloads versus Content References, revocation, rotation, recovery, and
audit attribution.

The Application interface must remain small while the deep module hides Waku
selectors, encryption, replay admission, Store queries, retries, and transport
profile details.

## Acceptance criteria

- [x] The selected design consumes the accepted DR-03 authority model without duplicating it.
- [x] At least two materially different addressing/delivery designs are compared.
- [x] The external interface and conversation/member state machine are explicit.
- [x] Delivery, acknowledgement, deduplication, ordering, expiry, cursor/stream, retry, and terminal outcomes are explicit.
- [x] Quotas, backpressure, replay, privacy, abuse, revocation, rotation, restart, and recovery are explicit.
- [x] Large data uses a justified Content Reference boundary.
- [x] Arbitrary `Publish(topic, bytes)` and Waku internals are absent from the public interface.
- [x] A proposed ADR decision and vertical implementation slices are ready for review.

## Blocked by

- DR-03

## Comments

Accepted 2026-07-25 after independent research and integrator review.

Evidence:

- `docs/engineering/research/application-messaging.md`
- Proposed
  `docs/adr/0015-authority-backed-bounded-application-conversations.md`
- canonical capability remains `no/no/no/no`

Selected opaque authority-backed conversations with Operator-only membership,
five bounded Application operations, durable local send acceptance,
recipient-local cursor ordering, at-least-once delivery, Node receipts distinct
from Application consumption, and Content References for large immutable data.
Rejected topic-centric and recipient-mailbox SDKs and any public Waku,
selector, generation, encryption, Store-query, or retry controls.

Validation:

- Wave 3 packet and ADR contract review
- compatibility review against accepted DR-03 research and Proposed ADR-0011
- documentation, architecture, capability-catalogue, and relevant Messaging
  tooling gates
