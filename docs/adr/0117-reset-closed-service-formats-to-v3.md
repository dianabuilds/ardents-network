---
status: accepted
date: 2026-09-29
supersedes: 0032, 0075, 0102, 0109
---

# ADR-0117 — Reset closed Service formats to one v3 edition

## Context

The maintained closed Service has no supported earlier clients or roots that
must migrate. Its
accepted formats nevertheless carry independent v1/v2/v3 labels, and the
retained Service Connection profile names a retired Route. The Product Owner
selected a direct v3 target and fresh-root cutover. [R-168](../research/records/r-168-service-v3-format-reset.md)
records the format inventory and the unverified deployment assumption.

## Decision

1. Credential and private Descriptor remain v3. Use `ardents-target:v3:` and
   `ardents-service-target-v3\0`; keep Target Link algorithm byte 1. Use v3
   request/response domains and root marker/schema for Instance; v3 public
   record prefix and root marker for Publication; v3 Reachability root marker
   and private stored-record envelope byte 3; v3 Service Connection prefix and
   numeric record version.
2. The Service Connection record's fixed profile is
   `ardents-service-connection-profile-v3`. Its logical context, continuity
   nonce/MAC/commitment domains, and TLS exporter label change to v3 together.
   Preserve the existing Terminal receipt/confirmation semantics. The shared
   `ardents-route-v3` and separate Application interfaces do not change. The
   retired Route v2 name remains only where Network/Route rejects old State.
3. All maintained Service producers and readers switch atomically as a source
   contract. No old format reader, writer, negotiated mode, fallback, adoption,
   or migration is retained. Unknown old roots, Links, and records fail before
   effects. Existing bytes are never deleted or rewritten automatically. The
   operator provisions fresh Service Authority, Instance, Publication, and
   Reachability roots and distributes a new Link.
4. The absence of supported old clients narrows the general compatibility
   obligation in the C0 scope for these exact Service identities. ADR-0032's
   Target Link v1 form, ADR-0075's v2 grammar, ADR-0102's v2 Instance formats,
   and ADR-0109's typed legacy
   envelope refusal are superseded only for their byte identities and old
   state handling. Their security and lifecycle invariants remain binding.

## Consequences

The same Authority public key derives a different Target. A mixed old/new
deployment is deliberately incompatible. New roots keep rollback, authority,
conflict, expiry, and resource floors; failure to parse old state grants no
permission to skip those checks. If a supported client or required persisted
state is discovered, implementation stops and returns to design. This closed
format reset does not qualify the installed protected text workload.
