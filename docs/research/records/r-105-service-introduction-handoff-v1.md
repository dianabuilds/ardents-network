---
id: R-105
title: Service Introduction handoff v1
status: open
owner: Product Owner and Codex
started: 2026-08-24
reviewed: 2026-08-24
---

# R-105 — How does the selected C-2 Introduction path authorize one Publisher-side Responder attachment and local Endpoint handoff without revealing Service material to Route Nodes or making Rendezvous a route planner?

## Decision this unlocks

Define or reject the smallest maintained Introduction runtime necessary for the
H4 usable-alpha User-to-Publisher path. It must cause the Publisher-side
Responder to create one exact State-authorized Rendezvous leg for the same
attachment as the User-side Initiator, then hand the resulting byte carrier to
the Publisher Endpoint. It must not convert Rendezvous into a Service lookup,
target-resolution, or route-selection authority.

## Current contract

- ADR-0024 selects separate C-5 data and C-2 Introduction shapes. Endpoint
  retains complete Route selection; Entry and Service Connection retain their
  existing authority boundaries.
- ADR-0026 selects the closed HPKE `SealedIntroduction` v1 envelope. Its
  visible header binds Network, epoch/digest, Introduction and Rendezvous Node
  identities, an opaque reachability value, expiry, JoinHandle, and
  EndpointHandshake. Its ciphertext is Service-only.
- ADR-0033 selects `RelaySetup`/`RelayReady` only for an Endpoint-to-transit
  authorization after admitted Entry TLS. The maintained Initiator now proves
  that path to Rendezvous.
- The maintained Rendezvous duty pairs two *outbound* Node legs by attachment
  ID. It cannot create or identify a Publisher-side leg by itself. There is no
  maintained Introduction runtime, SealedIntroduction plaintext grammar,
  one-use JoinHandle ledger, Publisher delivery channel, or Responder duty.
- The current Authority-signed Service Credential contains only an Ed25519
  `InstancePublic` key. The selected `SealedIntroduction` HPKE construction
  requires an X25519 recipient public key. No signed Service HPKE key,
  reviewed key-conversion rule, or independent key-distribution authority is
  currently selected.
- Existing Endpoint `Accept` already consumes an opaque authenticated Route
  byte carrier through its local Route attachment socket; it must not acquire
  Route State or learn node topology.

## Hypotheses

- **H1:** a finite State-authorized Introduction duty can authenticate one
  sealed request, spend one JoinHandle, and deliver a Service-only instruction
  that authorizes the Publisher Endpoint/Responder to dial exactly one named
  Rendezvous leg with the same attachment ID.
- **H2:** Rendezvous can derive the Publisher side from an attachment ID or a
  local Responder listener can accept it without an Introduction delivery.
- **H0:** no construction preserves the declared role-local knowledge and
  publication boundaries; the first alpha must narrow its service journey.

## Evaluation criteria

- The User receives an explicit Target Link outcome and never supplies an IP,
  Node identity, Publisher listener, or local proxy destination.
- A Route Node sees at most its declared adjacent role facts. It never sees a
  Service Target, publication, Instance key/material, complete Route, browser
  request, or alternate route candidate.
- The Publisher only receives Service-encrypted material plus finite delivery
  context; it cannot use a replay, foreign Network/epoch, expired request, or
  modified visible header to create a new Responder leg.
- Every authorized outbound leg is bound to the exact Network/epoch/digest,
  attachment ID, Rendezvous identity/key, profile, and expiry. No role chooses
  a peer, endpoint, fallback, or retry.
- Introduction, Publisher Endpoint, and Responder all have separate finite
  admission, byte, deadline, drain, withdrawal, replay, and local-handoff
  bounds. Failure becomes a classified unavailable result, not a direct
  Publisher connection.
- The implementation fits the one-person project: standard-library/reviewed
  HPKE only, no hidden broker/directory, public cloud relay, or durable
  operator service beyond roles already in authenticated State.

## Evidence plan

### Primary sources

- ADR-0005, ADR-0024, ADR-0026, ADR-0027, and ADR-0033, inspected
  2026-08-24.
- `internal/route/sealed_introduction.go`, `internal/route/relay_setup.go`,
  `internal/node` Rendezvous/Initiator duties, and Endpoint Service Connection
  acceptance, inspected 2026-08-24.

### Experiment

Before retaining runtime code, build a deterministic multi-process tracer with
one User Endpoint, Initiator, Introduction, Rendezvous, Responder, and
Publisher Endpoint. Capture only synthetic fixtures. It must prove exact
end-to-end attachment delivery, visible-header and ciphertext substitutions,
JoinHandle replay, stale/withdrawn State, unavailable Publisher local handoff,
duplicate Responder attempts, cancellation, and drained zero-owned-work
outcomes. The tracer must show that no direct User-to-Publisher or
Rendezvous-to-Publisher path occurs.

### Failure scenarios

- A malicious Introduction replays, swaps, delays, or replaces one delivery.
- A malicious Rendezvous learns a Service target or manufactures a Publisher
  leg from attachment metadata.
- A Service/publisher receipt is replayed to open a second Responder leg.
- The local Publisher Endpoint handoff is missing, slow, or points outside its
  owner-controlled socket.
- State changes an Introduction, Rendezvous, or Responder identity between
  selection and delivery.
- A failed delivery silently becomes a direct service connection, a generic
  local proxy, or a Node-selected fallback.

## Findings

- **Sourced fact:** `SealedIntroduction` provides a canonical HPKE envelope,
  but no plaintext grammar or replay/delivery protocol.
- **Sourced fact:** `internal/service/publication.Credential` supplies an
  Ed25519 Instance public key, while `SealIntroduction` accepts an HPKE/X25519
  public key. The current schemas do not bind such an HPKE recipient to the
  Service Credential.
- **Measurement:** the maintained Initiator → Rendezvous path requires the
  opposite leg to present the identical attachment ID before useful bytes can
  pass. A test-only manually opened Responder leg proves the pairing mechanics,
  not publisher reachability.
- **Inference:** adding a TCP listener to Responder would change the selected
  topology and let a remote party bypass the required Publisher-side
  authorization. It is not a safe implementation shortcut.

## Options

1. **State-assigned Introduction delivery with a Service-encrypted one-use
   handoff instruction.** The Introduction duty carries only the sealed
   envelope and finite visible binding, verifies/spends a JoinHandle, and
   reaches a Publisher-side local delivery owner. That owner opens the selected
   Responder-to-Rendezvous leg and hands its carrier to the existing Publisher
   Endpoint socket. This appears aligned but needs a closed plaintext and
   delivery protocol, including a dedicated X25519 HPKE public key in the
   Authority-signed Service Credential (or a separately accepted equivalent
   binding).
2. **Rendezvous-derived Publisher pairing.** Reject unless a future contract
   explicitly delegates service lookup and publisher notification to
   Rendezvous. It violates current endpoint-local selection and role knowledge
   boundaries.
3. **Responder public listener.** Reject: it creates a distinct ingress shape,
   exposes a new Publisher endpoint, and has no authorization binding to the
   selected SealedIntroduction/Service Instance.
4. **Narrow the alpha to a direct local/static demo.** Reject for the stated
   usable-alpha objective because it would not prove User-to-Publisher service
   access through H4-2.

## Recommendation

Choose no wire/runtime option yet. First derive a candidate plaintext and
delivery transcript from option 1, including exactly who knows and spends the
JoinHandle, which existing Service publication fact authorizes the recipient,
and how the Publisher-side local socket is authenticated. Then run the named
tracer before accepting a new ADR.

**Confidence:** high that a separate Introduction delivery is required; low
that the current record alone specifies a safe maintained protocol. The
strongest argument against pausing is that the existing HPKE envelope looks
nearly complete; its missing plaintext/replay/lifecycle semantics are exactly
where unauthorized ingress and topology leakage would otherwise enter.

## Disposition

Open and implementation-blocking for the complete H4-2/H4-3 service path.
No implementation, dependency, public wire, or product claim is selected by
this record. The previous test-only Responder leg remains evidence only.
