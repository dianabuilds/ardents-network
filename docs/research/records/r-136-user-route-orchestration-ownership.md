---
id: R-136
title: Which module owns volatile User-route orchestration?
status: closed
owner: Product Owner and Codex
started: 2026-09-03
reviewed: 2026-09-03
---

# R-136 — Which module owns volatile User-route orchestration?

## Decision this unlocks

Resolve the apparent overlap between ADR-0024's Endpoint-local Route selection
and ADR-0066's Endpoint-owned Transit Grant lifecycle before connecting the
headless Application Interface to the maintained private-reachability route.

## Current contract

ADR-0024 selects the native Interactive Route, authenticated State/private
Service inputs, Entry ownership, and no direct fallback. ADR-0066 makes the
Endpoint the durable, separate at-most-once owner of Introduction and
Responder Transit Grant request/key lifecycles. `internal/network/state`
already projects one exact destination-resolution Gateway, issuer, and
candidate by identity; a Reachability Descriptor names the exact Introduction
and Rendezvous slot. The Connection Interface accepts only a Service Link and
must consume its local capability before protected work.

## Hypotheses

- **H1:** `internal/route` owns the volatile User-route sequence from exact
  State/Descriptor facts through the authenticated attachment; Endpoint keeps
  name/capability handling and a callback adapter for durable credentials.
- **H2:** Endpoint continues to compose State, Entry, Gateway, RelaySetup,
  credential carrier, Introduction, and attachment directly.
- **H0:** Neither boundary can preserve one owner per fact without changing a
  selected wire grammar or allowing an unselected fallback.

## Evaluation criteria

- exactly one owner selects each Gateway, issuer, Initiator, Introduction, and
  Rendezvous fact;
- a credential journal cannot select a carrier or peer, and Route cannot
  mutate durable credential state;
- credential presentation becomes irreversible at receiving-Node TLS
  admission, not later Service TLS; an unconfirmed attempt is burned;
- all refusal and cleanup paths release Route admission exactly once;
- the Application Interface, Transit Grant v1, Descriptor, RelaySetup, and
  Introduction wire bytes remain unchanged.

## Evidence plan

### Primary sources

- `docs/adr/0024-native-interactive-route-foundation.md`, accessed 2026-09-03.
- `docs/adr/0066-use-role-scoped-transit-grant-requests.md`, accessed 2026-09-03.
- `internal/network/state/resolution_view.go`, `internal/endpoint/connection_interface.go`,
  `internal/endpoint/transit_credential_acquisition.go`, and `internal/route`,
  inspected 2026-09-03.

### Experiment

Build the complete User flow using only the State projection, a current Entry
contact, an opaque credential callback, and the existing closed codecs. Run
the Route and Endpoint behavior tests and the repository checks. Falsify H1
if the implementation needs a caller-supplied peer/URL, a Route import of the
credential subpackage, a new wire grammar, or cannot burn a credential after
receiving-node presentation.

### Failure scenarios

Test/inspect State conflict or expiry, an invalid Gateway profile, missing
Entry contact, private lookup refusal, descriptor/State disagreement, peer or
family overlap, issuer refusal, cancellation, credential-journal failure,
Introduction delivery failure after TLS admission, concurrent attachment
close, and Route shutdown while opening.

## Findings

- **Sourced fact:** State's Resolution view returns exact valid facts by
  identity and never exposes candidate ordering to a consumer.
- **Sourced fact:** Descriptor verification authenticates the Target,
  Publication, Introduction, and Rendezvous facts but intentionally leaves
  literal endpoints to State.
- **Sourced fact:** the Endpoint journal persists `presenting` before a Grant
  is used and resolves it only to a terminal spent/burned state; Route records
  `spent` only after the receiving node's delivery result confirms admission,
  and resolves any earlier ambiguity as `burned`.
- **Inference:** placing the Entry relay exchange inside Route prevents the
  durable journal from owning a carrier while the callback prevents a
  `route` -> `route/credential` import cycle.
- **Measurement:** focused `go test ./internal/route ./internal/endpoint`
  passes after the composition move on 2026-09-03.

## Options

H1 gives Route one deep volatile interface (`Open`, `Attach`, `Close`) while
leaving Endpoint's durable grant state on its existing bounded owner. H2 keeps
working code but divides one route across shallow Endpoint helpers and leaves
the old random candidate-selection implementation disconnected from the user
journey. H0 is rejected: H1 reuses every accepted grammar and introduces no
new selection or fallback.

## Recommendation

Choose H1 with high confidence. The strongest argument against it is the new
credential callback seam. It is justified by two concrete adapters: the
Endpoint durable journal in production and a behavior adapter in Route tests;
the callback contains only one opaque envelope exchange and cannot select a
peer.

## Disposition

Question closed. ADR-0070 records the ownership boundary. The maintained
Route, Endpoint, package-map, and technical contracts are updated in the same
change. No experiment directory or wire migration is required.
