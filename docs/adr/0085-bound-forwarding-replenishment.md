---
status: accepted
date: 2026-09-14
---

# Bound forwarding replenishment to the admitted parent channel

## Context

The closed Route grammar permits a fresh class-2 token to replenish a live
forwarding channel, but its former wording also reserved lane zero for one
admission handshake and permitted ADMIT on an unspecified existing lane. That
left the control target and the refill arithmetic ambiguous. A child must not
multiply its parent allowance, and a refill must not turn a host allowance into
an unbounded provider-cost commitment.

## Decision

After the initial HELLO and successful class-2 ADMIT, a forwarding channel may
receive a later ADMIT only on lane zero. It is parent-channel control, never a
child-lane operation. The new token must be class 2 and satisfy the original
receiver, TLS exporter, HELLO, peer, purpose and deadline bindings. The
original absolute deadline remains fixed.

Account the complete later ADMIT frame against the parent remaining reserve
before accepting its effect. After the token is verified and durably spent,
set the remaining parent byte reserve to exactly 32 MiB. Do not add another
32 MiB, refund past debits, alter child allocation, or extend the deadline.

Before spending that token, the installed host owner must reserve the refill's
additional byte allowance and any termination capacity not already held. The
host owner supplies its actual provider period, unit, counted directions,
interfaces and allowance; absent that reservation, the refill is refused.

## Consequences

The Route and Node implementation retain one parent admission owner across
the forwarding lifetime and expose no child replenishment path. The host
reservation is part of accepted work, rather than a later accounting sample.
This decision selects wire and accounting semantics only. It does not qualify
an installed host or invent the provider policy required for that qualification.
