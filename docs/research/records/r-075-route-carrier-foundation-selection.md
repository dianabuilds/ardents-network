---
id: R-075
title: Can Stage 8 select a maintained Route and Carrier foundation for M7--M9 without promoting the H3 laboratory tracer or importing another network's authority?
status: accepted
owner: Product Owner and Codex
started: 2026-08-23
reviewed: 2026-08-23
---

# R-075 — Route and Carrier foundation selection

## Decision this unlocks

DA-06 is the stop condition for M7 Entry/Carrier, M8 Route, and M9
Publication/Connection. This record determines whether an existing H3 tracer
or external overlay may become their maintained foundation in Stage 8.

## Current contract

R-001/R-004 retain an Interactive Route information-flow contract: a complete
Route and exact target must not be available to one ordinary role, and no
direct, shorter, cached, or weaker-profile fallback may turn an unavailable
Route into success. R-013 selected native C-5/C2 only for Carrier Lab and
explicitly excluded a production Route. R-036/ADR-0012 select WebTunnel only
as one endpoint-adjacent H3 camouflage Adapter. ADR-0009 selects Go as the
maintained foundation. R-023 Qualification and independent operator evidence
remain unfulfilled.

## Hypotheses

- **H1:** promote the native C-5/C2 laboratory tracer.
- **H2:** adopt or embed Tor/Arti as the maintained foundation.
- **H3:** use libp2p Circuit Relay as the Route foundation.
- **H4:** treat WebTunnel as the Route foundation.
- **H0:** select no foundation until a concrete profile supplies the missing
  protocol, authority, compatibility, and Qualification evidence.

## Evaluation criteria

An option must name its Route wire/profile, Node authority and discovery,
version/downgrade/retirement rule, endpoint-adjacent Carrier scope, recovery
owner, and one maintained implementation compatible with the selected Go
foundation. It must preserve R-001/R-004 role-local knowledge and R-023
evidence conditions without silently importing foreign naming, directory,
identity, or governance roots. A controlled lab run is not Qualification or
independent-operation evidence.

## Evidence plan

### Primary sources

- R-001, R-004, R-013, R-023, R-036, ADR-0009, and ADR-0012, inspected
  2026-08-23.
- [Tor onion-service protocol overview](https://spec.torproject.org/rend-spec/protocol-overview.html), accessed 2026-08-23.
- [Arti FAQ and status](https://arti.torproject.org/FAQs/), accessed 2026-08-23.
- Official libp2p Circuit Relay documentation, access attempt 2026-08-23;
  the prior R-013 primary-source evaluation remains the retained evidence.

### Experiment

No experiment can qualify a protocol that has not named its authority,
compatibility, and observer profile. A future candidate must run the R-023
controlled topology and prove failure, downgrade, role-knowledge, recovery,
and independent operator conditions against its selected profile.

### Failure scenarios

- a lab Route wire becomes a public protocol with no negotiation or retirement;
- foreign directory, descriptor, peer-ID, or identity semantics become Ardents
  naming or authority by import;
- a Bridge camouflage process learns or chooses a complete Route;
- a direct or shorter path is treated as recovery; and
- controlled co-resident roles are reported as independent operators.

## Findings

- **Sourced fact:** Tor hidden services use introduction points, rendezvous,
  descriptors, HSDir placement, and a Tor service identity; these are part of
  one protocol, not an interchangeable Carrier Channel.
- **Sourced fact:** Arti's April 2026 status permits client and onion-service
  use but not relay operation, and it is a Rust project.
- **Inspection:** R-013's native C-5/C2 result is a same-host controlled
  Carrier Lab result and explicitly rejects promotion to a production Route.
- **Inspection:** R-036 confines WebTunnel to the endpoint-adjacent Carrier
  Adapter and forbids it from selecting Route, retry, or continuity policy.
- **Inference:** none of H1--H4 satisfies every criterion without either
  promoting unqualified laboratory bytes, importing another network's
  authority, or contradicting the selected Go/ownership model.

## Options

| Option | Disposition |
|---|---|
| Native C-5/C2 | Reject as foundation: retained only as H3 evidence; no public wire, authority, compatibility, or Qualification profile exists. |
| Tor/Arti | Reject as foundation: Tor owns foreign descriptor/directory/identity semantics; Arti lacks relay support and is not the maintained Go foundation. |
| libp2p Circuit Relay | Reject: peer-addressed reachability relay does not by itself implement the accepted split Route or its knowledge boundary. |
| WebTunnel | Reject: endpoint-adjacent camouflage Adapter only, never Route authority. |
| H0 | Choose: no maintained Route/Carrier foundation is selected in Stage 8. |

## Recommendation

Choose H0 with high confidence. Do not begin M7--M9 implementation or retain
their current protocols by package migration. A future DA-06 closure requires
a new record tied to a concrete Interactive Route profile and must name the
wire, authority/discovery, versioning/retirement, selected Carrier boundary,
recovery ownership, R-023 Qualification plan, and one maintained Go
implementation. The strongest argument against H0 is that it delays three
Stage 8 waves; that delay is an honest consequence of no selected protocol,
not a reason to mislabel the tracer as a foundation.

## Disposition

**Accepted H0 on 2026-08-23 under the Product Owner's standing Stage 8
delegation.** At this point DA-06 remained open: H0 was an explicit
non-selection, not an authorization to implement a substitute. R-013 stayed
active for a future profile-bound candidate and WebTunnel remained only the H3
Adapter selected by ADR-0012. No ADR was created because this record selected
no technology, protocol, or format.

**Subsequent disposition, 2026-08-23:** R-076 supplied the missing concrete
native Profile and ADR-0024 selected it. DA-06 is therefore closed for
M7--M9. This record's rejection of promoting the existing H3 tracer, Tor/Arti,
libp2p Circuit Relay, or WebTunnel remains in force.
