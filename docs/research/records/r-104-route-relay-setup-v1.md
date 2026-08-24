---
id: R-104
title: Native Route relay setup v1
status: decided
owner: Product Owner and Codex
started: 2026-08-24
reviewed: 2026-08-24
---

# R-104 — How does an Endpoint-selected next Route position authorize an Initiator or Responder to open exactly one adjacent Node leg without exposing a complete Route or letting that Node select/fallback?

## Decision this unlocks

Define the smallest closed per-attachment setup exchange required to implement
the first maintained Initiator and Responder transit duties after Entry
admission. It must let the duty create its authorized next leg to the selected
Rendezvous without making either Node a Route planner.

## Current contract

- ADR-0024 selects endpoint-local Route selection and expressly prohibits a
  direct Service fallback.
- ADR-0026 fixes TLS 1.3 plus exact adjacent `LegBinding`, but does not define
  relay setup after User-to-Initiator Entry admission.
- `route.OpenEntryAttachment` authenticates the User-to-Initiator TLS leg and
  sends an `EntryBinding`; `route.AcceptEntryAttachment` validates/consumes it.
- The current EntryBinding contains the Network, epoch, digest, attachment,
  Initiator identity, expiry, client key digest, and opaque Invite. It contains
  no selected Rendezvous identity, endpoint, or authorization to open a next
  leg.
- Current `Route` selection is Endpoint-local and returns an opaque attachment;
  it does not hand a Node a route plan. The maintained Rendezvous duty therefore
  has no honest upstream caller yet.

## Hypotheses

- **H1:** one Endpoint-sent, TLS-confidential, fixed-width relay-setup record
  can name exactly one State-authorized next adjacent Node and bind it to the
  existing attachment/epoch/expiry before the Initiator or Responder dials.
- **H2:** the relay can derive the next peer independently from authenticated
  State and the attachment identity without receiving a setup record.
- **H0:** neither preserves endpoint route selection and role-local knowledge;
  the first alpha must narrow its Route contract before transit implementation.

## Evaluation criteria

- the Endpoint, not an Initiator, Responder, or Rendezvous, fixes the next
  peer and profile for the attachment;
- the transit duty validates that peer against its fresh authenticated State
  view before allocating a dial/TLS worker;
- only adjacent identity, endpoint, profile, attachment, epoch/digest, and
  expiry are visible to the transit duty; no Service Target, Instance,
  application bytes, complete Route, alternate candidate set, or retry policy
  is supplied;
- altered, replayed, expired, foreign-network, wrong-role, substituted,
  stale-State, and unavailable-next-peer setup fails before a non-authorized
  connection attempt; and
- the grammar, confirmation/terminal result, resource bound, drain behavior,
  and recovery interaction are precise enough for independent vectors.

## Evidence plan

### Primary sources

- ADR-0024 and ADR-0026, inspected 2026-08-24.
- `internal/route` Entry attachment, selection, LegBinding, and sealed
  Introduction implementation, inspected 2026-08-24.
- `internal/node` maintained Rendezvous duty and lifecycle, inspected
  2026-08-24.

### Experiment

After selecting a grammar, construct a deterministic three-process Initiator →
Rendezvous → Responder tracer using the maintained codecs and duty admission.
It must demonstrate exact reciprocal bindings, invalid setup refusal before a
next-leg dial, duplicate/replay refusal, bounded cancellation/drain, and no
direct Endpoint-to-Rendezvous path.

### Failure scenarios

- an Initiator derives or chooses a different Rendezvous after an Endpoint
  selected one;
- an Endpoint can make a relay dial an arbitrary IP or peer outside current
  State;
- setup reveals the opposite Endpoint, Service Target, Service Instance, or a
  complete Route;
- a replay starts an additional leg or survives State/expiry change; or
- failure changes into a direct connection, Node-selected fallback, or hidden
  multi-candidate retry.

## Findings

- **Inspection:** the current `EntryBinding` is intentionally insufficient for
  this job. Adding a dial directly in an Initiator listener would force that
  Node to choose its Rendezvous peer, contradicting ADR-0024's endpoint-local
  Route selection.
- **Inspection:** existing `LegBinding` authenticates an already-chosen
  adjacent Node pair. It cannot express the Endpoint-to-relay authorization
  needed before that leg exists, and overloading it would make a Node-to-Node
  record ambiguously serve an Endpoint-to-Node setup role.
- **Inference:** the H4-2A Rendezvous listener is a correct bounded first duty
  but is not a complete Route. Its current absence of an upstream maintained
  caller is intentional protocol incompleteness, not a reason to add a direct
  fallback.

## Options

1. **Closed relay-setup record over the admitted Entry TLS leg.** The Endpoint
   sends one fixed record naming a single preselected adjacent peer identity and
   State-authorized literal endpoint, bound to the current attachment, network,
   epoch/digest, profile, and expiry. The transit duty rechecks its current
   State and sends only the existing reciprocal LegBinding after dialing. It
   keeps Route selection endpoint-local and does not add a generic control API.
2. **Deterministic relay-side selection from State.** Rejected unless a future
   contract explicitly delegates selection: it changes the knowledge/authority
   boundary and can create Node-chosen retries or correlation.
3. **Endpoint directly dials Rendezvous after Entry admission.** Rejected: it
   bypasses the selected Initiator/Responder topology and turns the Node role
   into an unauthenticated side observation.
4. **Embed route setup in `LegBinding`.** Rejected: `LegBinding` has an
   established adjacent-Node meaning and no Endpoint authorization semantics.

## Recommendation

**Product decision (2026-08-24):** choose option 1. `RelaySetup` carries the
exact Network, epoch/digest, attachment, expiry, transit role/identity, and
one Endpoint-selected Rendezvous identity/public key. It deliberately carries
no literal endpoint. The transit duty obtains that endpoint only by rechecking
the exact current State candidate and its role/profile/validity. It returns an
identical `RelayReady` only after a reciprocal Node-to-Node LegBinding exists.
The entry/relay owner consumes an attachment identity once, so a setup cannot
start a second transit leg. No generic map, URI, next-candidate list, direct
fallback, or Node-selected retry is permitted.

**Confidence:** high for the authority and visibility boundary; medium for the
complete transit behavior until the Initiator/Responder three-duty tracer
supplies its stated failure evidence.

## Disposition

Decided and promoted through ADR-0033 plus the maintained `route` codec.
Initiator/Responder transit listeners, State recheck, replay ownership, and
the three-duty evidence remain implementation work. Direct dial, fallback, and
browser path remain unselected by this decision.
