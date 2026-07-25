# DR-01: Design bounded Application Messaging

Status: ready-for-agent
State: open
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

- [ ] The selected design consumes the accepted DR-03 authority model without duplicating it.
- [ ] At least two materially different addressing/delivery designs are compared.
- [ ] The external interface and conversation/member state machine are explicit.
- [ ] Delivery, acknowledgement, deduplication, ordering, expiry, cursor/stream, retry, and terminal outcomes are explicit.
- [ ] Quotas, backpressure, replay, privacy, abuse, revocation, rotation, restart, and recovery are explicit.
- [ ] Large data uses a justified Content Reference boundary.
- [ ] Arbitrary `Publish(topic, bytes)` and Waku internals are absent from the public interface.
- [ ] A proposed ADR decision and vertical implementation slices are ready for review.

## Blocked by

- DR-03

## Comments

None.
