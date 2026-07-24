# Authorize private messages before durable replay admission

Status: accepted for the first release

A receiver will admit a private message to the durable replay ledger only after
all message-level authentication and authorization checks succeed. Receive
processing therefore has this fixed security order: bounded outer framing,
coarse time and routing checks, receiver `Subscribe` capability resolution,
AEAD authentication and decryption, canonical inner-message decoding, message
class/scope/lifetime checks, Ed25519 sender-signature verification, retained
sender-grant `Publish` authorization, and finally atomic durable replay
admission.

AEAD authentication proves possession of the channel generation secret. It
does not prove that the claimed sender signed the message or currently has
`Publish` permission. Consequently, an AEAD-valid envelope is not eligible to
consume durable replay capacity until the sender checks have completed. The
authorization checks and replay admission use the same normalized observation
time derived for the receive attempt.

Replay admission remains the boundary before domain delivery. The ledger
serializes competing admissions, durably persists a new message ID before
reporting success, and rejects later admissions of the same ID. This gives
exactly-one admission under concurrent delivery and preserves the rejection
across process restart. A crash after replay admission but before domain
processing may prevent redelivery, so this is an at-most-once delivery boundary,
not an exactly-once domain-application guarantee.

## Considered Options

- Admitting immediately after AEAD authentication was rejected because any
  participant holding a `Subscribe` channel secret could consume the bounded
  durable replay budget with invalid signatures, unauthorized senders, or
  class-invalid envelopes.
- A bounded pre-authorization reservation followed by atomic promotion was
  rejected because it adds another capacity pool, expiry policy, crash state,
  and promotion transaction without a demonstrated throughput requirement.
- Admitting after domain delivery was rejected because concurrent or retained
  duplicates could reach non-idempotent domain handlers before a durable replay
  decision existed.

## Consequences

Invalid signatures, missing or revoked `Publish` grants, and invalid
class/scope/lifetime combinations do not mutate the replay ledger or consume its
capacity. Authenticated and authorized messages still fail closed if the replay
ledger cannot persist their admission. Callers must treat successful replay
admission followed by a domain failure as an at-most-once terminal attempt and
must not rely on transport redelivery for recovery.

Canonical ownership of ledger files and protection against multiple
same-file/path-alias ledger instances are separate concerns tracked by ARD-018;
this decision neither weakens nor resolves them.
