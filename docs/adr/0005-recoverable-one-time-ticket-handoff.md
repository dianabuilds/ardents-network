# ADR 0005: Recoverable one-time ticket handoff

- Status: Accepted
- Date: 2026-07-24
- Decision owners: Identity, Bootstrap, CLI

## Context

Bootstrap and Application enrollment tickets are one-use plaintext capabilities,
while only their digests may be retained. Committing a digest before a protected
file or RPC response is delivered can strand the sole active ticket; retaining
recoverable plaintext would instead enlarge the credential-exposure boundary.

## Decision

Every ticket handoff has the states `issued`, `delivered`, `acknowledged`,
`expired`, and `reissued`. `issued`, `delivered`, and `acknowledged` are durable;
`expired` is derived from the stored deadline, and `reissued` is the atomic
transition that replaces an unacknowledged digest with a fresh `issued` digest.

Only the digest of the newest generation is authoritative. Retrying issuance may
atomically reissue an `issued` or `delivered` ticket, immediately invalidating the
older plaintext; an acknowledged ticket cannot be reissued. Delivery is
idempotently recorded by matching the plaintext to the authoritative digest.
Using a delivered ticket acknowledges it in the same transaction as enrollment.

Bootstrap separates database issuance from protected-file delivery. On restart,
the daemon first proves that an existing protected file matches the authoritative
digest; otherwise it reissues and atomically replaces the file. Application
ticket issuance records delivery only after the audit outbox is flushed and
before returning the RPC result. A retry after audit, transport, or client-file
failure reissues rather than waiting for expiry.

Plaintext is held only in bounded process memory and the designated protected
file or RPC response. It is never stored in the identity database, audit outbox,
logs, metrics, state labels, or delivery identifiers.

## Consequences

- At most one ticket digest is valid at every transaction boundary.
- A response that reached a client may later be invalidated by an explicit retry;
  callers must use the newest successfully delivered ticket.
- Recovery does not reproduce lost plaintext. It replaces its authority.
- Legacy active records are treated as delivered and legacy consumed records as
  acknowledged, so existing tickets remain usable across upgrade.
