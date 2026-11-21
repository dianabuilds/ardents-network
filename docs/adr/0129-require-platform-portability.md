---
status: accepted
date: 2026-11-21
---

# ADR-0129 — Require Windows, Ubuntu and macOS support

## Context

The Product Owner made Windows, Ubuntu and macOS mandatory product targets
and required platform-specific implementation only for unavoidable host
differences. This amends the broader platform target in ADR-0125 and ADR-0126.

## Decision

1. Ardents must support Windows, Ubuntu and macOS. The applicable product
   behavior and protection contract is shared across them.
2. Keep requirements, domain logic, protocols, configuration semantics and
   validation platform-independent wherever they can be expressed portably.
   Use a shared implementation wherever it can satisfy the required behavior
   and protections on all target platforms.
3. Add platform-specific code only for a required host operation or guarantee
   that the portable implementation cannot provide. Keep that code at the
   boundary that performs the operation. The owning component continues to
   decide what the observed OS facts authorize. A missing required protection
   yields the same explicit unavailable outcome on every platform.

## Consequences

For native code, identify the exact host dependency and the common guarantee
its implementation preserves. This applies to existing code as it is revised,
including the former Installation packages under `internal/platform`.

The selected C0 qualification profile remains Ubuntu LTS `x86-64`.
Supported versions, release scope and native qualification evidence follow
the applicable release contracts.
