---
status: accepted
date: 2026-11-21
---

# ADR-0128 — Retire the headless Node companion

## Context

Platform-led Node roles run through `ardents-next`. The headless alpha
bundle still included the former Node command, whose remaining routes only
refused retired invocations. The Product Owner approved removing the command
and its process-refusal tests while preserving verification of existing bundles.

## Decision

New headless alpha bundles use `ardents-closed-alpha-enrollment-v4`. The
pinned inventory requires Endpoint, control and Authority Custody and
rejects the former Node companion. Descriptor fields, pin verification,
executable identity and protected-generation checks retain their existing
rules.

Existing v3 bundles keep their original verification rules. Headless
verification requires both Node and Custody. General verification accepts
either the pair or neither. A partial pair fails both scopes. The retired
v1 and v2 descriptors continue to fail. Existing bundles and pins are
verified in place.

The separate platform Infrastructure artifact keeps its signed
`ardents-node-linux-amd64` identity and contains the `ardents-next`
executable.
