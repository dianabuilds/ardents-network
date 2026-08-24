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
- Credential v2 now binds a distinct `IntroductionHPKEPublic` X25519 recipient
  under the existing Authority signature (ADR-0034). The Ed25519
  `InstancePublic` remains a Service Connection key; no key-conversion rule or
  independent key-distribution authority exists.
- The Publisher Endpoint's existing `IntroductionSocket` is an owner-local
  Unix IPC used once at publication to obtain a signed acknowledgement. It has
  no Reachability registration, sealed-request reader, remote delivery, or
  attachment lifecycle. Reusing it as a C-2 channel would silently change its
  narrow publication-control authority.
- The only maintained Endpoint-to-transit admission record is `EntryBinding`
  v1. Its fixed `InitiatorNodeID` and `AcceptEntryAttachment` receiver bind a
  User-to-Initiator TLS attempt. It does not authorize Publisher-to-Responder
  admission. Renaming or reusing it would alter the accepted wire without a
  versioned record and explicit authorization semantics.
- Existing Endpoint `Accept` already consumes an opaque authenticated Route
  byte carrier through its local Route attachment socket; it must not acquire
  Route State or learn node topology.

On 2026-08-24 the closed Service-only plaintext is selected as
`ServiceIntroductionInstruction` v1: Target, Credential generation, current
publication digest, and attachment ID. Its codec is owned by
`internal/service/publication`, and only the decrypted Publisher compares it
with its current live publication. Network/State/slot context remains in the
authenticated outer header; no Route Node can decode this instruction.

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

**Control-slice evidence (2026-08-24):**
`experiments/r-105-live-introduction-tracer/run-local.ps1` built a disposable
binary outside the repository and ran separate loopback Publisher,
Introduction, and User processes. All synthetic cells passed on the first
run: one exact delivery; one spent-JoinHandle refusal; visible
EndpointHandshake AAD substitution refusal at the Publisher; ciphertext
substitution refusal at the Publisher; and no delivery after the Publisher
closed its slot. The Introduction role read/compared only outer opaque handles
and relayed canonical sealed bytes; its control path neither parsed nor
printed Service-only plaintext. The runner deleted its binary/log directory
afterwards.

**Full exact-path evidence (2026-08-24):** the same disposable runner then
started six separate processes: User, Initiator, Introduction, Rendezvous,
Responder, and Publisher. The User first delivered the C-2 sealed instruction;
the Publisher validated its current synthetic publication, opened the separate
Responder attachment, and the two State-pinned Node legs paired at maintained
Rendezvous. The User sent one synthetic HTTP request through the Initiator and
received the exact static Reference Site response from the Publisher. Each
role emitted one successful terminal record, and the maintained Initiator and
Rendezvous processes drained with zero active connections.

This proves one exact full synthetic route, not the entire R-105 matrix. It
does **not** yet prove stale/withdrawn State, unavailable Publisher local
handoff, duplicate Responder attempts, cancellation, browser integration, or
the maintained C-2 duty lifecycle. Those remain retained-runtime gates.

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
- **Sourced fact:** `requestIntroductionAcknowledgement` dials a local Unix
  socket, sends only Target/generation/expiry/network/broker plus a nonce, and
  receives one signature. It is not a Service reachability or delivery API.
- **Sourced fact:** `route.EntryBinding` and `AcceptEntryAttachment` are
  Initiator-specific. The State-assigned Responder cannot use them as its
  publisher-side first-hop authorization without a new closed record.
- **Measurement:** the maintained Initiator → Rendezvous path requires the
  opposite leg to present the identical attachment ID before useful bytes can
  pass. A test-only manually opened Responder leg proves the pairing mechanics,
  not publisher reachability.
- **Inference:** adding a TCP listener to Responder would change the selected
  topology and let a remote party bypass the required Publisher-side
  authorization. It is not a safe implementation shortcut.

## Accepted subdecision

On 2026-08-24 the Product Owner accepted a separate X25519 public recipient
in the versioned Authority-signed Credential. ADR-0034 closes that binding as
Credential v2 and deliberately rejects a v1 compatibility reader for alpha.
This resolves the recipient-key gap only; it does not decide the C-2
plaintext, delivery, replay ledger, Responder admission, or Publisher local
handoff.

On the same date, the Product Owner accepted C-2 as a live Publisher-originated
Introduction slot. ADR-0035 selects that topology and the separate closed
`EndpointTransitBinding` v1 first-hop record for Introduction and Responder.
The remaining runtime/control grammar must still be tested before it is
retained.

## Options

1. **State-assigned Introduction delivery with a Service-encrypted one-use
   handoff instruction.** The Introduction duty carries only the sealed
   envelope and finite visible binding, verifies/spends a JoinHandle, and
   reaches a Publisher-side local delivery owner. That owner opens the selected
   Responder-to-Rendezvous leg and hands its carrier to the existing Publisher
   Endpoint socket. This appears aligned but needs a closed plaintext and
   delivery protocol plus an exact Publisher-to-Responder admission record.
   Its recipient key is the dedicated X25519 HPKE public key in Credential v2
   (ADR-0034).
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

Choose option 1: State-assigned live Introduction delivery. Credential v2 and
ADR-0035 select the recipient binding, topology, and two additional first-hop
roles. First derive and test the closed plaintext/control transcript, including
exact JoinHandle spending, Service-publication binding, local socket
authentication, and terminal resource ownership.

**Confidence:** high that a separate Introduction delivery is required; low
that the current record alone specifies a safe maintained protocol. The
strongest argument against pausing is that the existing HPKE envelope looks
nearly complete; its missing plaintext/replay/lifecycle semantics are exactly
where unauthorized ingress and topology leakage would otherwise enter.

## Disposition

Open and implementation-linked for the complete H4-2/H4-3 service path.
ADR-0034 selects the signed X25519 recipient binding; ADR-0035 selects live
C-2 slots and EndpointTransitBinding v1. The tracer now supports the selected
delivery/HPKE binding shape and one exact six-process static-reference route.
The retained C-2 duty lifecycle, State/failure matrix, local handoff, and
browser lifecycle remain to be implemented and tested. The previous test-only
Responder leg remains evidence only.
