# ADR 0015: Authority-backed bounded Application conversations

- Status: Proposed
- Date: 2026-07-25
- Decision owners: Application Interface, Identity and Security, Messaging
- Research source: `docs/engineering/research/application-messaging.md`

## Context

The frozen product has authenticated private-envelope primitives, durable
authorization-before-replay admission, Waku Relay/Filter/Lightpush/Store
adapters, and an Application Interface for Identity and Content. It has no
Application Messaging contract, conversation state, inbox/outbox, delivery
receipt, or production Channel Grant authority.

Exposing `Publish(topic, bytes)` would make Applications own selectors,
encryption generations, transport choice, replay, Store queries, retries, and
membership races. Addressing each recipient mailbox directly would hide fewer
transport details but would still force every caller to reconstruct group
membership, fan-out, idempotency, and revocation.

Accepted ADR-0011 selects one deployment-owned Realm Authority Principal,
fresh Channel Grant generations on membership change, survivor activation
attestations, explicit fencing, and an independent monotonic checkpoint trust
root. Application Messaging must consume that authority rather than duplicate
or weaken it.

## Decision

Expose owner-independent, realm-scoped, opaque conversations through a small
typed Application interface:

- `ListConversations`;
- `SendMessage`;
- `ReceiveMessages`;
- `AcknowledgeMessages`;
- `GetDelivery`.

`ConversationID` is a random opaque `cv1_...` identifier. It is not a Waku
topic, selector, Principal derivative, or secret. Operator-only conversation
procedures create, change membership, rebind a member's delivery Node, and
close a conversation. Every membership change invokes ADR-0011 and activates
a fresh Application-channel generation before the new membership becomes
current.

A member is one Ardents Principal bound to exactly one active delivery Node in
that conversation. The public Application calls never choose recipients,
topics, channels, generations, providers, Relay, Filter, Lightpush, Store, or
retry policy. One-to-one conversations are the same aggregate with two
members; groups contain three through thirty-two members.

`SendMessage` requires an exact `messaging.message.send` Access Grant on the
conversation and active membership of the Effective Principal. An optional
one-hop Delegation is intersected under ADR-0002. The Node persists the
idempotency row, immutable message, and per-recipient outbox entries in one
transaction. Success means durable local acceptance only, not remote delivery
or Application processing.

The idempotency key is `(Effective Principal, ConversationID,
client_message_id)`. Reuse with the same canonical digest returns the original
result; reuse with different content fails as a conflict. The Node assigns an
opaque `MessageID`. Recipients atomically authorize the current membership and
generation, perform durable message-ID replay/deduplication admission, append
to the inbox, and only then issue a recipient-Node `received` attestation.
That attestation is evidence asserted by an approved Node release/host, not
proof that a person or Application processed the message.

Applications receive messages through bounded paged long-poll subscriptions
with durable owner-qualified cursors. Recipient-local admission cursor order is
stable; no global, causal, or identical cross-recipient order is promised.
`AcknowledgeMessages(through_cursor)` records Application consumption and makes
earlier entries eligible for retention cleanup. It is not a read receipt and
does not make Application side effects exactly once.

Inline payloads are at most 32 KiB. A message may instead carry at most sixteen
immutable owner-qualified Content References under ADR-0004. Messaging neither
copies the referenced bytes nor grants access to them; every recipient must
obtain independently valid Content authority. References outlive message
expiry according to their own retention contract.

Transport delivery is at-least-once until a recipient receipt, message expiry,
revocation, or terminal fencing result. Default TTL is 24 hours, minimum one
minute, maximum seven days. Expiry stops undelivered work but does not retract
already admitted inbox entries. A full recipient inbox withholds the receipt,
so bounded internal retry may continue until expiry. There is no exactly-once
delivery, read receipt, presence, typing, arbitrary broadcast, federation, or
caller-controlled retry in v1.

Conversation, message, subscription, inbox/outbox byte, retry, receipt, query,
and audit cardinalities are finite. Backpressure before durable acceptance
returns `ResourceExhausted`; after acceptance the outbox remains authoritative
and callers inspect `GetDelivery` rather than resending with a new key.

## Consequences

- Applications receive a small product-level contract while Messaging owns a
  deep conversation, persistence, delivery, and recovery module.
- ADR-0011 acceptance and production authority implementation precede
  Application Messaging implementation.
- Revocation and membership changes are bounded but not instantaneous during a
  partition; completion follows ADR-0011 activation/fencing truth.
- Message and cursor persistence becomes a new backup/restore consistency
  group checked against the latest authority checkpoint.
- Existing private discovery/data wire classes are not repurposed silently; a
  versioned Application-message class and compatibility window are required.
- Old Nodes report Messaging unavailable. Incompatible generations fail
  closed; downgrade requires a complete stopped backup restore.
- Capability `application.messaging` remains `Q=no` until matching-commit
  contract, security, multinode, deployment, recovery, and release evidence is
  accepted by DR-06.

## Rejected alternatives

### Topic-centric SDK

Rejected because it exposes Waku and security machinery, permits authority
confusion, and has no coherent conversation membership or delivery truth.

### Recipient-mailbox fan-out

Rejected because every caller would own group membership snapshots, partial
fan-out, idempotency, ordering, and membership-change races.

### Exactly-once or synchronous multi-recipient send

Rejected because transport receipts cannot prove Application side effects and
partitioned recipients would turn ordinary send into an unbounded distributed
transaction.
