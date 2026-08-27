---
id: R-107
title: Private lookup Entry-to-Relay carrier
status: decided
owner: Product Owner and Codex
started: 2026-08-25
reviewed: 2026-08-26
---

# R-107 — How does an Endpoint carry one OHTTP reachability lookup from a User-to-Initiator Entry without a direct Relay TCP connection or a generic proxy surface?

## Decision this unlocks

Select the H4-3A carrier that joins the implemented Target descriptor OHTTP
adapter to an actual Endpoint Route path.

## Current contract

- R-106/ADR-0036 select Target-keyed Private Reachability Resolution.
- `route.EntryBinding` authenticates an exact User-to-Initiator attempt. The
  Initiator accepts the existing C-2 `RelaySetup` with next role Rendezvous and
  the separately authenticated, finite `ResolutionRelaySetup` under ADR-0037.
- The current OHTTP Client/Relay/Gateway test correctly separates opaque Relay
  and Target-aware Gateway, but its ordinary HTTP transport is an adapter test
  input, not an Endpoint-private carrier.
- A Destination Resolution Role is in the non-adjacent Rendezvous Domain;
  relay/Gateway identities and families must be excluded from the later Service
  Connection Rendezvous.

## Hypotheses

- **H1:** extend the Initiator-side Entry duty with one separately bound,
  finite `resolution-relay` operation that carries only one fixed OHTTP
  envelope to one State-selected Gateway, with no Target parsing.
- **H2:** construct a complete ordinary Route before private lookup and carry
  OHTTP within it.
- **H0:** neither option preserves current Entry authority and bounded resource
  ownership; H4-3 must defer Endpoint-private lookup.

## Evaluation criteria

- Entry sees endpoint adjacency but no Target; Relay sees no Target; Gateway
  sees Target but no endpoint origin.
- The new operation is bound to Network/State/deadline/fresh TLS key and one
  opaque Gateway identity, cannot dial arbitrary hosts, and has hard byte/time
  limits.
- Existing C-2 `RelaySetup` behavior and Entry Invite semantics remain
  unchanged; an Entry Invite cannot authorize an Introduction, Responder,
  Service, browser, or generic TCP/HTTP proxy action.
- One malicious Initiator cannot select an alternate Gateway, retain a request,
  observe plaintext Target, or reuse the carrier for Application Data.

## Evidence plan

Build separate User, Initiator/Relay, Gateway, and Publisher processes. Prove
fixed envelope forwarding; target absence from Initiator capture; wrong Gateway,
replay, expiry, byte overflow, and HTTP/proxy-form rejection; then attach the
success result to the existing C-2 Reference Site test.

## Findings

- **Historical fact:** before this selected implementation the Initiator duty
  read only `RelaySetup` and required its next role to be Rendezvous.
- **Inference:** passing a regular `http.Transport` to the OHTTP Client from
  Endpoint would create the prohibited direct Relay source. It cannot be a
  production H4-3 adapter.
- **Inference:** the narrowest viable H1 form is a new one-use
  `ResolutionRelaySetup` after the existing admitted Entry TLS. It binds only
  Network/Epoch/Digest/Attachment, Initiator identity, one State-selected
  Gateway Node identity/public key, a whole-second deadline, and a fixed
  envelope-size profile. The Initiator derives the Gateway literal endpoint
  only from its fresh State view, opens no arbitrary host, forwards exactly one
  opaque OHTTP envelope, and closes both legs. It cannot parse a Target or
  descriptor.
- **Inference:** the existing Entry Invite may remain the admission capability
  for this finite Initiator operation because it grants no Target, Service,
  browser, or Gateway choice. The new setup—not the Invite—limits the action
  to resolution relay. Entry replay consumption remains per attachment/TLS key.
- **Measurement (2026-08-26):**
  `TestInitiatorForwardsOneOpaqueResolutionEnvelopeToExactGateway` proves one
  Entry-attached Initiator forwards one opaque 4-KiB OHTTP envelope to exactly
  its State-pinned Gateway and drains with no remaining connection. The
  maintained C-2 tracer then uses the same carrier before verifying one exact
  Descriptor and opening its separately attached C-2 path. Its bounded result
  does not prove arbitrary carrier use, a participant Route configuration,
  multi-host operation, or browser privacy.

## Recommendation

H1's `ResolutionRelaySetup` is accepted. It keeps endpoint-adjacent exposure
at the existing Initiator Entry while adding one closed opaque operation rather
than a fifth Role Domain or generic proxy. The strongest argument against it is
the new Route wire/duty surface; its bounded implementation needs ongoing
qualification rather than a generic-carrier claim.

## Disposition

**Decided — H1 accepted by the Product Owner on 2026-08-25.** ADR-0037 fixes
the new closed `ResolutionRelaySetup` operation. The Initiator duty,
Endpoint-private adapter, and maintained C-2 tracer now implement it. The
carrier remains bounded evidence only; it does not supply participant Route
configuration, first-run acquisition, multi-host qualification, or browser
privacy.
