---
status: accepted
date: 2026-11-22
---

# ADR-0130 — Retire Control and ACA1 from alpha bundles

## Context

The ACA1 reader retired under [ADR-0127](0127-retire-aca1-inspection-and-portable-enrollment-diagnosis.md).
Enrollment and packaging still required its nine files and the Control
executable in the headless inventory. The Product Owner confirmed there are
no prior bundle consumers or compatibility obligations in [#563](https://github.com/dianabuilds/ardents-network/issues/563#issuecomment-7077977124).

## Decision

The sole accepted Endpoint bundle descriptor is
`ardents-closed-alpha-enrollment-v5`. It carries the cohort, release, platform,
environment, network, target path, Endpoint artifact and trusted root fields.
The headless bundle contains the Endpoint and Authority Custody executables,
the descriptor, trusted root and Release metadata. The protected Linux option
requires the complete protected group under [ADR-0119](0119-bind-protected-endpoint-generation-to-release.md).

Enrollment rejects older descriptors, unknown inventory entries and missing or
partial required companions. One `enrollment verify` operation checks the
independent manifest pin before interpreting the inventory and checks the
actual executable. Release authenticates metadata and retains its own floors.
Infrastructure retains its separate inventory and offline preparation and
signing operations remain purpose-specific.

This supersedes the Endpoint descriptor and companion editions in
[ADR-0112](0112-pin-network-enrollment-v3-sole-descriptor.md) and
[ADR-0128](0128-retire-headless-node-companion.md).
