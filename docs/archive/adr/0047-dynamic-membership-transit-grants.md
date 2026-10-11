---
status: accepted
date: 2026-08-26
supersedes: 0039-state-authorized-transit-grants-v1.md (only for dynamic Introduction admission)
---

# ADR-0047 — Issue dynamic membership-level Transit Grants through State and Entry

## Context

ADR-0039 made an Introduction Transit Grant an exact, one-use, State-signed
adjacent-hop admission capability.  Its closed-alpha delivery form embedded
one such Grant in a Reachability Descriptor and provisioned its matching TLS
key to the intended Endpoint.  That proves a fixed trace, but it cannot serve
two independent Endpoints or repeated browser opens: the Descriptor has one
authorization field and a Grant is bound to one attachment and one local key.

H4-4's named path must not repair this with a caller route plan, a
publisher-specific access ticket, or by showing a Name or Target to a transit
issuer.  Those alternatives either introduce a second route authority or turn
network admission into Service authorization.  R-118 demonstrates that an
exact Target-free request can traverse a separate Endpoint, Initiator, and
issuer process, and that a real Entry TLS attachment can carry its opaque
post-admission data.

## Decision

For the project-operated alpha, Reachability Descriptor v2 may declare
**membership-level dynamic Introduction submission** instead of embedding a
fixed Introduction Grant.  This declaration is a closed mode, not an empty or
ambiguous authorization field.  It binds the existing signed State epoch,
Introduction Node, Reachability, JoinHandle, and expiry, but carries no
per-Service issuer ticket.

For that mode the Endpoint creates a fresh local TLS key and attachment, then
uses one separately admitted Entry TLS connection for a closed Credential
Relay operation.  State is the only source of the selected `transit-issuance`
duty and its authenticated OHTTP profile.  The relay setup binds Network,
State epoch/digest, the Entry-selected Initiator, the selected issuer identity,
and the exact digest of that signed profile, deadline, and one fixed
opaque-envelope capacity.  The Initiator accepts it only when the commitment
matches its State projection.  It contains no Name, Target, Descriptor,
Publisher material, C-2 peer, literal endpoint, or fallback choice.

The OHTTP plaintext is limited to the current Network/epoch/digest, the
already Descriptor-selected Introduction Node and role, fresh attachment,
fresh TLS public-key digest, and bounded expiry.  The State authority signs
the resulting normal Transit Grant v1 only after checking that exact tuple and
the live selected issuance duty.  The issuer's HTTPS listener requires a TLS
client certificate whose Ed25519 Node key is the selected Initiator key, and
the Initiator pins the issuer certificate to that State identity.  The issuer
rechecks a typed current-State duty before every signature.  The Endpoint
verifies the Grant, retains its
matching private key only until the sole spend or expiry, and presents it to
the Introduction.  The Grant continues to authorize only that adjacent hop;
the Publisher's sealed JoinHandle and Application remain the Service access
control.

The first implementation is explicitly project-operated alpha: State-selected
Initiator and issuer enforce one finite exchange per accepted Entry attachment
and their bounded duty resources.  It claims neither unlinkable admission nor
resistance to a malicious selected Initiator/issuer.  A hostile-peer,
unlinkable, rate-limited membership credential is separate H4-6 research.

ADR-0062 supersedes this decision's online signer custody and participant
lifecycle: the issuer uses a separately scoped State-authenticated Grant key,
not an Epoch authority private key, and owns durable finite budget/idempotency
with fixed encrypted outcomes. This ADR continues to own Descriptor v2 and the
target-free Credential Relay shape.

## Consequences

- Dynamic and fixed Descriptor authorization have distinct versioned wire
  modes.  A fixed Grant is never silently reinterpreted as permission to issue
  a new one; unsupported Descriptor versions/modes fail closed.
- The online issuance duty is a State/control availability and key-custody
  responsibility.  Absence, withdrawal, saturation, bad response, key loss,
  or State change prevents a new route and has no fallback.
- Mutual TLS proves the adjacent selected Initiator to the issuer; it does not
  make a malicious selected Initiator honest or create a participant quota.
  The separate bounded issuance-budget and lifecycle owner remain required
  before this becomes a participant-operable process.
- The issuer does not receive an alpha name, Target, Publication, Descriptor,
  sealed C-2 bytes, or stable Invite identifier.  This does not itself prove
  anonymity against a colluding Initiator and issuer.
- The static embedded Grant path remains valid only for historical/fixed
  Descriptor v1.  It is not the participant-ready dynamic H4-4 path.

## Compliance

- [ADR-0039](0039-state-authorized-transit-grants-v1.md) remains the Transit
  Grant grammar and receiving-Node verification decision.
- [ADR-0037](0037-private-reachability-entry-carrier.md) is the precedent for
  one closed Initiator-mediated OHTTP operation; it is not a generic relay.
- [R-118](../research/records/r-118-participant-transit-credential-lifecycle.md)
  records the rejected batch, reproducible carrier evidence, and limits.
