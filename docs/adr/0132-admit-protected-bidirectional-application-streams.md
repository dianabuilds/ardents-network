---
status: accepted
date: 2026-11-24
---

# Admit protected bidirectional Application streams

## Context

ADR-0081 selects the first fixed document workload. The Service Connection
already carries ordered Application bytes. Participants need an ordinary
interface that accepts arbitrary bytes and preserves the common protection
baseline. The Product Owner approved the direction in #803 and required
qualification with two installed Endpoints.

## Decision

Amend ADR-0081's fixed Application selection to admit a second, explicitly
selected Application profile for bidirectional ordered bytes. Preserve the
document profile and its existing commands, signed identities and wire bytes.
Both workloads use the same protected Route and mandatory authority,
confinement and lifecycle rules.

The participant connects to an exact Service Target through the Endpoint.
The authenticated Instance confirms the selected Application profile on the
original Connection before admitting user payload. Connection continuity
retains that binding. Missing or incompatible confirmation refuses the
operation before payload effects. A signed local worker profile and exact
local authority govern execution. Applications own their byte interpretation.
The Endpoint owns destination verification, admission and stream lifetime.

The CLI accepts input and emits output as bytes with diagnostics on a separate
channel. Local EOF closes only the send direction. Success requires the
authenticated terminal outcome. Interruption can leave partial output and
never implicitly repeats an Application operation. See the
[participant contract](../product/protected-service-workload.md#participant-interface-for-arbitrary-data).

## Consequences

Implementation must specify finite per-direction byte, duration and concurrency
bounds, and bind them to the admitted Application job. Release qualification
requires the real workflow on two installed Endpoints, byte-exact tests in both
directions, profile-refusal and termination controls, and document regression
coverage. The decision changes the target workload. Existing release evidence
still applies only to the workloads and artifacts it exercised.
