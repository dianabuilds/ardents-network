# ADR 0007: Multi-provider fetch terminal semantics

- Status: Accepted
- Date: 2026-07-24
- Decision owners: Transfer, Discovery, Private Messaging

## Context

Private blob and manifest fetch requests are broadcast to multiple trusted Nodes.
A signed error proves only that one responder rejected or could not satisfy one
request; it does not prove that every provider failed. Treating the first error
as globally terminal lets one malicious or stale provider suppress a later
honest response.

Responses also need enough signed routing identity to prevent an error for one
resource, owner, or request from terminating another in-flight fetch. A fetch
must nevertheless terminate when every eligible provider fails or providers
remain silent.

## Decision

At request publication, the requester snapshots all currently usable trusted
Node records except itself. That immutable set is the candidate set for the
in-flight fetch; later discovery changes do not add or remove candidates.
A request with no candidates fails before publication.

The response wire contract is version 2; earlier unbound responses fail closed.
Every signed response binds:

- request ID and requester Principal;
- resource ID and resource kind;
- owner Principal for owner-qualified resources;
- responder Node Principal;
- status, error, and success payload.

Only a response whose responder belongs to the request's candidate snapshot and
whose complete signed binding matches the request can affect lifecycle state.
Malformed, mismatched, untrusted, and non-candidate responses are ignored and
cannot count another candidate as failed.

The first valid signed error from a candidate makes only that candidate terminal.
Further errors or successes from that candidate are ignored. When every
candidate has failed, the transfer fails immediately with a deterministic,
candidate-sorted exhaustion diagnostic.

The first valid success from a still-active candidate completes the transfer.
The response waiter is then unregistered; duplicate and late successes are
ignored and cannot repeat storage, source observation, events, or journal
completion.

Fetches inherit an earlier caller deadline and otherwise use a 15-second
deadline. If candidates remain silent, the deadline fails the transfer with the
number of failed candidates and the total candidate count. Cancellation follows
the same bounded lifecycle while retaining `context.Canceled` as the cause.

## Consequences

- One malicious provider cannot suppress an honest provider's valid success.
- A provider cannot reverse its own terminal error with a late success.
- Exhaustion and timeout are distinct, bounded, and operator-diagnosable.
- Candidate membership and response binding are fixed for each request, so
  concurrent discovery refreshes cannot change terminal semantics.
- Adding a new resource kind requires including all of its identity fields in
  the canonical signed response before it can use this fetch lifecycle.
