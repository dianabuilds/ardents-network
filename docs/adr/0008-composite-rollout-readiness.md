# ADR 0008: Composite rollout readiness

- Status: Accepted
- Date: 2026-07-24
- Decision owners: Deployment, Operations, Diagnostics, Identity, Network

## Context

The Compose rollout accepted a Node after `network status` reported either a
ready seed or a joined peer. That signal did not prove that the protected
Operator API remained usable, Diagnostics remained healthy, or the Node
retained its identity and the Operator's Access Grant. A deployment could
therefore commit an image that was connected but not operable.

## Decision

`NodeService.GetNodeRuntime` is the only rollout readiness contract. The RPC is
protected by the Operator interface. A successful response proves both the
protected API path and admission through the retained Access Grant. Its
`runtime.readiness` value contains explicit checks for:

- `protected_api`;
- `access_grant`;
- `network`;
- `diagnostics`;
- `identity`, including a ready state, Principal, and public key.

The daemon owns evaluation of network, Diagnostics, and identity readiness.
The protected RPC boundary adds the API and Access Grant checks only after
authorization succeeds. The deployment orchestrator does not duplicate these
rules: it accepts a Node only when the authorized response has
`runtime.readiness.ready == true`.

The orchestrator polls until its bounded deadline. Every negative readiness
response retains the canonical reason, and transport, authentication, and
authorization failures retain their command failure reason. The final timeout
reports the last observed reason.

## Consequences

- Network-only success can no longer commit a rollout.
- Any degraded contract component blocks the current Node and triggers the
  existing transaction compensation.
- Runtime and deployment tooling use the same machine-readable decision.
- Adding a new mandatory component requires changing the runtime contract and
  its degradation matrix, not deployment-specific predicates.
