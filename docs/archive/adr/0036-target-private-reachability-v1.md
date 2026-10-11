---
status: accepted
date: 2026-08-25
supersedes: none
---

# ADR-0036 — Resolve Target Links through private, current descriptors

## Context

H4-3 requires a User who possesses only a Target Link to obtain the current
Service publication and live C-2 introduction facts.  A Target Link is
intentionally location-independent and cannot contain those mutable values.
Passing a Publisher origin, a JSON execution plan, or a project catalog to the
User would instead create a direct source, a hidden directory, or a replayable
second authority.

The existing private naming exchange is deliberately Namespace-specific: its
current proof is a Namespace proof.  Reusing it as a descriptor authority would
silently make the Namespace own Target reachability.

## Decision

Add a distinct, Target-keyed **Private Reachability Resolution** protocol.
It uses the already accepted fixed-size OHTTP relay/Gateway separation, but has
its own request, response, descriptor verifier, durable Gateway state, and
role evidence.  The endpoint chooses authenticated Relay and Gateway identities
only from the Rendezvous Domain, excludes both identities and known families
from the resulting Service Connection's Rendezvous choice, and uses a separate
Initiator entry attachment for the lookup.  No direct Publisher source or
fallback exists.

An Authority-signed Service Credential is the Target's authority binding: the
Target must equal the canonical derivation from the Credential Authority key.
The Authority issues Credentials in non-overlapping validity intervals.  A
Gateway persistently tracks one Target's highest accepted Credential generation
and exact publication digest.  It accepts only a valid higher generation after
the old generation's terminal validity, retains one identical generation, and
marks an equal-generation digest conflict unavailable.  A response therefore
cannot turn an older still-valid publication into a successful current result.

The current Instance signs each short-lived Reachability Descriptor below its
current Publication.  That descriptor supplies only the live C-2 introduction
slot facts needed to begin the existing composition.  Its expiry is no later
than the Credential expiry and its publication digest is exact.  A stale,
withdrawn, conflicting, malformed, wrong-network, wrong-target, or expired
descriptor is `unavailable`/`invalid`; it never triggers another Target, a
direct Publisher attempt, or a browser handoff.

The Gateway is an availability and privacy participant, not a Service
Authority, Namespace authority, or general directory.  It may withhold a
response and therefore cause an explicit unavailable result.  It cannot create
a valid descriptor, override an Authority's Target binding, choose User Route
peers, mint Entry/Node authorization, or make an old generation current.  The
protocol offers no protection against Relay/Gateway collusion, timing/volume
correlation, compromised Authority/Instance, or a Broad Traffic Observer.

## Consequences

- H4-3's Target Link remains the complete shareable destination: its runtime
  obtains mutable reachability only through this protocol.
- Existing `naming/resolution` stays Namespace-only.  The maintained OHTTP
  dependency is reused under its existing ADR-0014 approval, without sharing
  Namespace records, state, or authority.
- Service Authority issuance and Publisher publication gain an explicit
  non-overlap/currentness invariant; Gateway restart persistence is required.
- The first alpha may have a small, pre-provisioned Destination Resolution
  corpus and a single failure domain, but must state that availability and
  independent-operation claims are not qualified.
- The protocol must be qualified with substituted Target, old valid
  publication, same-generation conflict, stale slot, wrong Entry invite,
  Gateway withholding, and no-direct-origin cases before it supports browser
  handoff.

## Compliance

R-106 records the comparison and Product Owner acceptance.  The complete
data/wire and process boundary is in
`docs/technical/private-reachability.md`; maintained behavior precedes any
H4-3 `open` command.
