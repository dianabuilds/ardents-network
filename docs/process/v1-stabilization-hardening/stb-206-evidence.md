# STB-206 Evidence — Privacy Observability And Carrier Capture

Date: 2026-07-19

## Accepted Capability

The canonical local network-status surface now reports the active
`ardents-private/1` posture independently from the Waku transport profile. Raw
Waku capture tests validate opaque selector shape, encrypted envelope framing,
and absence of declared product semantics for discovery and data traffic.

## Operator-Visible Truth

`GetNetworkStatus`, ConnectRPC, JSON, and `ard network status` expose:

- `privacy_profile` and `privacy_state`;
- `privacy_switch_reason` and `privacy_recovery_state`;
- unavailable `private_publication`, `private_discovery`, and
  `private_data_exchange` operations in `reduced_capabilities`;
- bounded `privacy_error_categories`.

Configured but temporarily unresolvable capabilities report
`recovery_pending`; missing channel configuration reports `blocked`; a fully
resolvable profile reports `steady`. The projection resolves current
permissions rather than trusting startup configuration or cached material.

The error projection is an explicit allowlist of stable capability categories.
Unknown errors and unknown `privacy.capability.*` lookalikes collapse to
`privacy.capability.unavailable`; resolver text, paths, subjects, capability
references, selectors, and secrets never enter the status contract.

## Capture And Mutation Coverage

- `ValidateOpaqueSelector` accepts only the fixed Waku pubsub topic and the
  capability-derived 32-character lowercase base32 selector shape.
- `ValidateEncryptedPayload` requires a structurally valid
  `ardents-private/1` outer envelope header and ciphertext length.
- the testkit capture inspector checks raw Waku envelopes for both structural
  violations and exact forbidden semantic markers without echoing those
  markers into findings.
- NPI-002 mutates a real captured opaque selector into a readable discovery
  topic and replaces ciphertext with plaintext; both mutations are detected by
  stable findings and remain canonical regression assertions.
- NPI-004 applies the same capture inspector to the private data request path.
- E2E-NPI-001 captures a private discovery-class publication from a real Waku
  subscriber before Open and proves the declared principal, service,
  operation, and full plaintext markers are absent.

The capture claims are intentionally bounded: they prove fixed carrier shape
and absence of selected plaintext semantics, not traffic-analysis anonymity or
size-correlation resistance.

## Acceptance Gates

- formatting gate — passed;
- `go vet ./...` — passed;
- canonical fast runner and import boundary — passed;
- touched production Go code-size guard — passed without soft or hard breach;
- canonical integration runner at
  `tests/.artifacts/reports/stb-206-final-integration`: 105/105 passed, 0
  failed; `summary.json`, raw reports, and `junit.xml` retained;
- canonical E2E runner at `tests/.artifacts/reports/stb-206-final-e2e`: 14/14
  passed, 0 failed; `summary.json`, raw reports, and `junit.xml` retained;
- race suite across Network Privacy, control status/projection, runtime
  process, ConnectRPC projection, and testkit — passed;
- test catalog: 119 tests, 32 scenarios, 119 formal bindings, 0 issues.

## Transition-Gate Decision

Accepted. Discovery/publication and data traffic use the private Waku carrier;
raw capture proves the selected semantics are absent; replay, tamper, expiry,
revocation, missing capability, temporary degradation, and recovery are
covered and observable; the carrier validator rejects the targeted readable
fallback mutations.
