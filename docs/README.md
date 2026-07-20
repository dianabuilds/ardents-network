# Docs Map

## Purpose

This file explains the role of documents stored directly in `docs/` so the
repository has a clear distinction between:

- core architectural source of truth;
- supporting requirement and architecture deep-dives;
- active registers;
- non-normative reference notes.

## Core Source Of Truth

These documents define the active product form and must be read before
substantial changes:

- `system-concept.md`
- `system-frame.md`
- `system-properties.md`
- `canonical-network-foundation.md`
- `engineering-constraints.md`
- `development-contract.md`

`reference-invariants.md` is also mandatory when work touches network,
discovery, messaging, or publication foundation.

## Supporting Deep Dives

These documents remain active and useful, but they refine a slice of the
architecture rather than redefining the whole system:

- `communication-contracts.md`
- `network-and-discovery-requirements.md`
- `network-privacy-requirements.md`
- `network-privacy-architecture.md`
- `network-transport-variants-requirements.md`
- `network-transport-architecture.md`
- `network-participation-profiles.md`
- `node-runtime-requirements.md`
- `workload-and-services-requirements.md`
- `data-substrate-requirements.md`

When a supporting document conflicts with the core source of truth or with the
current domain documents in `docs/domains/`, the core and domain documents win
first and the supporting document must then be updated.

## Active Registers

These files are operational registers, not general architecture specs:

- `security-exceptions.md`

They should contain only currently active runtime or dependency exceptions.

## Proposed Target Architecture

These documents describe intended post-stabilization architecture. They are
not current runtime claims and become normative only after explicit acceptance
and corresponding updates to the active requirements and domain documents:

- `network-resilience-and-anonymity-target-architecture.md`
  Defines the target Waku-backed transport, reliability, onion-routing, mixnet,
  HTTPS camouflage, metadata-privacy, delivery, and qualification design.

## Non-Normative Reference

These files are intentionally kept as legacy/reference aids. They do not define
the target architecture of `v1` on their own:

- `reference-extraction-notes.md`
- `reference-runtime-flows.md`

They are useful when extracting proven mechanisms from legacy code without
copying legacy package shape.

## Are There Extra Files Here?

At the moment there are no obviously extra top-level documents that should be
deleted immediately.

Current status:

- core source-of-truth files are active;
- supporting deep-dives still have a clear role;
- `reference-*` files are not normative, but they are still useful enough to
  keep as explicit legacy guidance.

Possible future cleanup, if desired:

- move `reference-*` files into a dedicated `docs/reference/` folder;
- group transport/privacy deep-dives under a dedicated subfolder once their
  set grows further.
