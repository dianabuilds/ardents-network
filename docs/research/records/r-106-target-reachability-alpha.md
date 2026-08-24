---
id: R-106
title: Target Link reachability acquisition for closed alpha
status: decided
owner: Product Owner and Codex
started: 2026-08-25
reviewed: 2026-08-25
---

# R-106 — How can a User who has only an exact Target Link obtain one current authenticated Service Publication and its C-2/Entry facts for the closed alpha without a direct Publisher origin, a hidden directory, a stale replay path, or a shadow Namespace authority?

## Decision this unlocks

Select the production acquisition boundary for H4-3A's Target Link open flow,
or narrow the alpha claim before any user-facing `open` command is built.

## Current contract

- A Target Link is exactly a network-bound opaque Target. It contains neither
  origin, mutable reachability, Node identity, nor a naming fallback
  ([CONTEXT](../../../CONTEXT.md)).
- Private Reachability Resolution is required for Target Links as well as
  name-derived Targets and has no direct public fallback. Its maintained
  target-descriptor protocol is not implemented yet.
- `publication.Decode` verifies a bounded Credential and publication against
  the target's Service Authority. `OpenUserReferenceSite` further compares the
  Target Link against that publication before emitting a browser origin.
- A User C-2 attempt additionally needs a State-selected Introduction,
  Initiator, and Rendezvous, a bounded Entry acquisition, and one opaque
  Introduction authorization. The maintained C-2 APIs deliberately require
  these typed inputs; they do not look up or choose them.
- `entry.Import` owns State-issued Entry Invite validation/replay state. A
  Publisher cannot manufacture a User's Entry Invite merely by signing a
  Publication.
- H4-4A name resolution remains deferred. A project-signed catalog cannot be
  passed off as current Namespace or current reachability authority.

## Hypotheses

- **H1:** One explicit, bounded, signed alpha invitation can carry an exact
  Target Link plus independently verifiable Publication and Entry facts while
  declaring that it is a short-lived invitation, not current private
  reachability resolution.
- **H2:** A minimal private reachability role can provide a current descriptor
  without joining H4-4 naming or exposing a direct Publisher origin.
- **H0:** Neither candidate retains target binding, freshness, and role
  separation with one-person alpha operations; H4-3 must defer its open flow.

## Evaluation criteria

- The User can start from an exact Target Link and receives a current signed
  Publication or an explicit unavailable/stale/conflicting outcome.
- No carrier Node receives a direct Publisher origin, Service private key,
  complete Route, or authority to choose a Target or a fallback.
- A stale, replayed, equivocated, withheld, substituted, or foreign-network
  artifact cannot silently create a Service Connection.
- A Publisher cannot mint an Entry Invite or an authorization for a transit
  Node; State/Entry retain their existing authority.
- The chosen form is bounded, inspectable, and usable by the actual one-person
  alpha team without becoming a hidden global directory or Namespace.

## Evidence plan

### Primary sources

- Current glossary definitions for Target Link, Private Reachability
  Resolution, Service Descriptor, and Destination Resolution Role, inspected
  2026-08-25.
- `internal/service/publication`, `internal/entry`, `internal/endpoint`, and
  ADR-0032 through ADR-0035, inspected 2026-08-25.

### Experiment

For each candidate, construct separate User, Publisher, and Node processes.
Record accepted origin-free inputs, exact Target/Network binding, expiry and
floor behavior, missing/current/stale/conflicting cases, and whether User can
complete one C-2 Reference Site request. The experiment must reject a changed
Publication, old valid Publication, wrong Target Link, substituted Entry
Invite, and direct Publisher endpoint before an HTTP request is possible.

### Failure scenarios

- A replayed invitation presents an old but still time-valid Publication.
- A project catalog or relay equivocates between two Publication generations.
- A User imports an Entry Invite for a different State generation or Node.
- The reachability service withholds, observes, or substitutes one Target
  request; a direct Publisher fallback is attempted.
- A static alpha invitation is accidentally described as DNS, Namespace, or a
  general private resolver.

## Findings

- **Sourced fact:** a Target Link deliberately cannot supply mutable
  reachability or a Publisher origin.
- **Sourced fact:** current C-2 composition consumes caller-supplied,
  State-selected transit facts and a signed Publication; its success in the
  six-process test is not an acquisition proof.
- **Sourced fact:** a Publication Credential authenticates its Authority,
  Target, Network, capabilities, generation, and validity interval, but
  `publication.Decode` has no User-held per-Target publication floor or
  authenticated source of a newer generation. It therefore accepts an older
  still-time-valid signed generation.
- **Sourced fact:** an Entry Invite is validated against a current State view
  at the Initiator. It cannot be made current merely by placing its bytes next
  to a Publication in a Publisher-signed artifact.
- **Inference:** a JSON plan that simply embeds the required bytes would make
  the product flow appear complete while leaving their producer, freshness,
  and authority unexplained. It is not an acceptable production adapter.
- **Inference:** option 1 cannot satisfy the stated `Target Link -> site`
  journey by itself: if a User needs a separately distributed invitation to
  learn the current facts, the invitation is the acquisition authority. It is
  potentially a useful closed-alpha test fixture, but must be explicitly
  described as such and cannot be presented as Target Link reachability.

## Options

1. **Explicit short-lived alpha invitation.** Carry an exact Target Link,
   signed Publication, and independently verifiable State/Entry references
   through a new bounded format. It may be a closed-alpha bootstrap only and
   must have explicit expiry/floor/replay behavior. It cannot be called the
   Target Link itself or Namespace resolution.
2. **Minimal Private Reachability Resolution.** Add a target-descriptor
   publication/lookup role with its own authenticated freshness and privacy
   contract. This fits the glossary but is a material protocol and operations
   slice.
3. **Project catalog lookup.** Reject as a sole authority: R-098's catalog is
   an inspectable index, not Release, State, Namespace, or reachability
   authority.
4. **Direct Publisher endpoint.** Reject: it bypasses C-2 and reveals a
   prohibited origin-like ingress.

## Recommendation

Choose option 2: implement a minimal Private Reachability Resolution role for
the accepted H4-3 journey. The Product Owner accepted this choice on
2026-08-25. Its currentness basis is deliberately narrow: Target-derived
Authority binding, Authority-issued non-overlapping Credential lifetimes,
Gateway-held durable per-Target floor/conflict state, and short-lived
Instance-signed live slot evidence. It is not Namespace proof or a catalog.

**Confidence:** high that a direct plan or catalog would violate the existing
contract, and that a signed invitation alone cannot establish currentness for a
Target Link. The strongest argument against the chosen role is its operational
and protocol scope before one alpha user can open one site; the counterweight is
that it is the smallest selected path which preserves the stated H4-3 journey.

## Disposition

Decided. ADR-0036 and `docs/technical/private-reachability.md` own the selected
boundary. The closed Descriptor v1 codec and its Endpoint-to-C-2 Reference
Site composition are implemented and covered by the maintained C-2 Reference
Site behavior test. Gateway-local durable generation/conflict state has its own
restart, stale-slot, and same-generation-conflict behavior tests. The
separate-process tracer has not yet adopted the descriptor. The next work is
private lookup and the falsification experiment. R-106 no longer blocks
selection; it continues to govern that implementation and qualification.
